package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	cronpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/cron"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/support"
	workspacepkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/workspace"
)

const (
	toolReadMaxBytes       = 12 * 1024
	toolWriteMaxBytes      = 24 * 1024
	toolCommandOutputLimit = 12 * 1024
)

type readFileArgs struct {
	Path      string `json:"path"`
	Mode      string `json:"mode"`
	Offset    int64  `json:"offset"`
	Length    int64  `json:"length"`
	MaxBytes  int    `json:"max_bytes"`
	StartLine int64  `json:"start_line"`
	MaxLines  int64  `json:"max_lines"`
}

type writeFileArgs struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Mode      string `json:"mode"`
	Overwrite bool   `json:"overwrite"`
}

type editFileArgs struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

type listFilesArgs struct {
	Path string `json:"path"`
}

type runCommandArgs struct {
	Command string   `json:"command"`
	Cmd     string   `json:"cmd"`
	Args    []string `json:"args"`
	Cwd     string   `json:"cwd"`
}

type weatherToolArgs struct {
	Location string `json:"location"`
	Type     string `json:"type"`
	Days     int    `json:"days"`
}

type cronAddArgs struct {
	Name         string `json:"name"`
	ScheduleType string `json:"schedule_type"`
	IntervalS    int64  `json:"interval_s"`
	AtEpoch      int64  `json:"at_epoch"`
	Message      string `json:"message"`
}

type cronRemoveArgs struct {
	JobID string `json:"job_id"`
}

type todoStepArgs struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type todoWriteArgs struct {
	Goal   string         `json:"goal"`
	Status string         `json:"status"`
	Steps  []todoStepArgs `json:"steps"`
}

type todoUpdateArgs struct {
	StepID string `json:"step_id"`
	Text   string `json:"text"`
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

func executeGetCurrentTimeTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = call
	_ = ctx
	now := time.Now()
	_, offsetSeconds := now.Zone()
	offsetHours := offsetSeconds / 3600
	offsetMinutes := (offsetSeconds % 3600) / 60
	offset := fmt.Sprintf("%+03d:%02d", offsetHours, absInt(offsetMinutes))
	return toolSuccessResult(map[string]any{
		"datetime":       now.Format(time.RFC3339),
		"date":           now.Format("2006-01-02"),
		"time":           now.Format("15:04:05"),
		"timezone":       now.Format("MST"),
		"utc_offset":     offset,
		"unix":           now.Unix(),
		"weekday":        now.Weekday().String(),
		"human_readable": now.Format("2006-01-02 15:04:05 MST"),
	})
}

func executeWeatherTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = ctx
	var args weatherToolArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}
	location := strings.TrimSpace(args.Location)
	if location == "" {
		return toolErrorResult(fmt.Errorf("location is required"))
	}
	if !vars.APIConfig.Weather.Enable || strings.TrimSpace(vars.APIConfig.Weather.Key) == "" {
		return toolErrorResult(fmt.Errorf("weather API is not configured"))
	}

	queryType := strings.ToLower(strings.TrimSpace(args.Type))
	if queryType == "" {
		queryType = "current"
	}
	days := args.Days
	if days <= 0 {
		days = 1
	}
	if days > 5 {
		return toolErrorResult(fmt.Errorf("days must be between 1 and 5"))
	}

	hoursFromNow := 0
	if queryType == "forecast" {
		if strings.TrimSpace(vars.APIConfig.Weather.Provider) != "openweathermap.org" {
			return toolErrorResult(fmt.Errorf("forecast is only supported with openweathermap.org"))
		}
		hoursFromNow = days * 24
	} else if queryType != "current" {
		return toolErrorResult(fmt.Errorf("unsupported type: %s", queryType))
	}

	condition, isForecast, localDatetime, speakableLocation, temperature, temperatureUnit := support.GetWeatherForTool(location, "", hoursFromNow)
	if strings.TrimSpace(condition) == "" || condition == "undefined" {
		return toolErrorResult(fmt.Errorf("weather lookup failed for %s", location))
	}
	return toolSuccessResult(map[string]any{
		"location":           strings.TrimSpace(speakableLocation),
		"requested_location": location,
		"type":               queryType,
		"days":               days,
		"is_forecast":        strings.EqualFold(isForecast, "true") || queryType == "forecast",
		"local_datetime":     strings.TrimSpace(localDatetime),
		"condition":          strings.TrimSpace(condition),
		"temperature":        strings.TrimSpace(temperature),
		"temperature_unit":   strings.TrimSpace(temperatureUnit),
		"summary":            buildWeatherSummary(speakableLocation, condition, temperature, temperatureUnit, localDatetime, queryType, days),
	})
}

func buildWeatherSummary(location, condition, temperature, unit, localDatetime, queryType string, days int) string {
	location = strings.TrimSpace(location)
	condition = strings.TrimSpace(condition)
	temperature = strings.TrimSpace(temperature)
	unit = strings.TrimSpace(unit)
	localDatetime = strings.TrimSpace(localDatetime)
	if queryType == "forecast" {
		return fmt.Sprintf("%s forecast in %d day(s): %s, %s°%s. Local time: %s.", location, days, condition, temperature, unit, localDatetime)
	}
	return fmt.Sprintf("%s weather now: %s, %s°%s. Local time: %s.", location, condition, temperature, unit, localDatetime)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func executeCronAddTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	var args cronAddArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}
	job, err := cronpkg.AddJob(cronpkg.Job{
		Name:         strings.TrimSpace(args.Name),
		ScheduleType: strings.TrimSpace(args.ScheduleType),
		IntervalS:    args.IntervalS,
		AtEpoch:      args.AtEpoch,
		Message:      strings.TrimSpace(args.Message),
		RobotESN:     strings.TrimSpace(ctx.ESN),
	})
	if err != nil {
		return toolErrorResult(err)
	}
	return toolSuccessResult(map[string]any{
		"job": map[string]any{
			"id":            job.ID,
			"name":          job.Name,
			"schedule_type": job.ScheduleType,
			"interval_s":    job.IntervalS,
			"at_epoch":      job.AtEpoch,
			"message":       job.Message,
			"robot_esn":     job.RobotESN,
			"next_run_at":   job.NextRunAt,
		},
	})
}

func executeCronListTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = call
	_ = ctx
	jobs := cronpkg.ListJobs()
	items := make([]map[string]any, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, map[string]any{
			"id":            job.ID,
			"name":          job.Name,
			"schedule_type": job.ScheduleType,
			"interval_s":    job.IntervalS,
			"at_epoch":      job.AtEpoch,
			"message":       job.Message,
			"robot_esn":     job.RobotESN,
			"created_at":    job.CreatedAt,
			"next_run_at":   job.NextRunAt,
			"last_run_at":   job.LastRunAt,
		})
	}
	return toolSuccessResult(map[string]any{
		"jobs":  items,
		"count": len(items),
	})
}

func executeCronRemoveTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = ctx
	var args cronRemoveArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}
	job, ok, err := cronpkg.RemoveJob(strings.TrimSpace(args.JobID))
	if err != nil {
		return toolErrorResult(err)
	}
	if !ok {
		return toolErrorResult(fmt.Errorf("job not found"))
	}
	return toolSuccessResult(map[string]any{
		"removed": true,
		"job": map[string]any{
			"id":   job.ID,
			"name": job.Name,
		},
	})
}

func executeTodoReadTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = call
	_ = ctx
	state, err := workspacepkg.LoadTodoState()
	if err != nil {
		return toolErrorResult(err)
	}
	if state.ActivePlan == nil {
		return toolSuccessResult(map[string]any{
			"path":        filepath.ToSlash(workspacepkg.TodoJSONRelativePath),
			"notice":      "我看了当前待办，现在没有进行中的计划。",
			"plan_status": "empty",
		})
	}
	return toolSuccessResult(map[string]any{
		"path":        filepath.ToSlash(workspacepkg.TodoJSONRelativePath),
		"goal":        state.ActivePlan.Goal,
		"plan_status": state.ActivePlan.Status,
		"active_plan": state.ActivePlan,
	})
}

func executeTodoWriteTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = ctx
	var args todoWriteArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}
	goal := strings.TrimSpace(args.Goal)
	if goal == "" {
		return toolErrorResult(fmt.Errorf("goal is required"))
	}
	if len(args.Steps) == 0 {
		return toolErrorResult(fmt.Errorf("at least one step is required"))
	}
	if len(args.Steps) > 8 {
		return toolErrorResult(fmt.Errorf("todo plan is too long; keep it to 8 steps or fewer"))
	}
	steps := make([]workspacepkg.TodoStep, 0, len(args.Steps))
	for i, step := range args.Steps {
		text := strings.TrimSpace(step.Text)
		if text == "" {
			return toolErrorResult(fmt.Errorf("step %d text is required", i+1))
		}
		steps = append(steps, workspacepkg.TodoStep{
			ID:     step.ID,
			Text:   text,
			Status: step.Status,
			Notes:  step.Notes,
		})
	}
	state := workspacepkg.TodoState{
		ActivePlan: &workspacepkg.TodoPlan{
			Goal:   goal,
			Status: strings.TrimSpace(args.Status),
			Steps:  steps,
		},
	}
	if err := workspacepkg.SaveTodoState(state); err != nil {
		return toolErrorResult(err)
	}
	loaded, err := workspacepkg.LoadTodoState()
	if err != nil {
		return toolErrorResult(err)
	}
	if loaded.ActivePlan == nil {
		return toolErrorResult(fmt.Errorf("todo plan was saved but could not be reloaded"))
	}
	return toolSuccessResult(map[string]any{
		"path":        filepath.ToSlash(workspacepkg.TodoJSONRelativePath),
		"goal":        loaded.ActivePlan.Goal,
		"plan_status": loaded.ActivePlan.Status,
		"active_plan": loaded.ActivePlan,
		"notice":      "我已经写好了当前待办计划。",
	})
}

func executeTodoUpdateTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = ctx
	var args todoUpdateArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}
	stepID := strings.TrimSpace(strings.ToLower(args.StepID))
	if stepID == "" {
		return toolErrorResult(fmt.Errorf("step_id is required"))
	}
	state, err := workspacepkg.LoadTodoState()
	if err != nil {
		return toolErrorResult(err)
	}
	if state.ActivePlan == nil {
		return toolErrorResult(fmt.Errorf("there is no active todo plan to update"))
	}
	found := false
	for i := range state.ActivePlan.Steps {
		step := &state.ActivePlan.Steps[i]
		if strings.ToLower(strings.TrimSpace(step.ID)) != stepID {
			continue
		}
		if text := strings.TrimSpace(args.Text); text != "" {
			step.Text = text
		}
		if status := strings.TrimSpace(args.Status); status != "" {
			step.Status = status
		}
		if notes := strings.TrimSpace(args.Notes); notes != "" {
			step.Notes = notes
		}
		found = true
		break
	}
	if !found {
		return toolErrorResult(fmt.Errorf("todo step not found: %s", args.StepID))
	}
	workspacepkg.NormalizeTodoStateForRuntime(&state)
	autoArchived := false
	if state.ActivePlan != nil && (state.ActivePlan.Status == "completed" || state.ActivePlan.Status == "cancelled") {
		autoArchived = workspacepkg.ArchiveActivePlan(&state)
	}
	if err := workspacepkg.SaveTodoState(state); err != nil {
		return toolErrorResult(err)
	}
	loaded, err := workspacepkg.LoadTodoState()
	if err != nil {
		return toolErrorResult(err)
	}
	if autoArchived {
		notice := "我已经更新了当前待办进度，全部步骤完成后我把这个计划自动收尾了。"
		if len(loaded.RecentPlans) > 0 && loaded.RecentPlans[len(loaded.RecentPlans)-1].Status == "cancelled" {
			notice = "我已经更新了当前待办进度，这个计划现在已结束，我把它自动收尾了。"
		}
		return toolSuccessResult(map[string]any{
			"path":         filepath.ToSlash(workspacepkg.TodoJSONRelativePath),
			"plan_status":  "empty",
			"recent_plans": loaded.RecentPlans,
			"notice":       notice,
		})
	}
	if loaded.ActivePlan == nil {
		return toolErrorResult(fmt.Errorf("todo plan update succeeded but could not be reloaded"))
	}
	return toolSuccessResult(map[string]any{
		"path":        filepath.ToSlash(workspacepkg.TodoJSONRelativePath),
		"goal":        loaded.ActivePlan.Goal,
		"plan_status": loaded.ActivePlan.Status,
		"active_plan": loaded.ActivePlan,
		"notice":      "我已经更新了当前待办进度。",
	})
}

func executeTodoClearTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = call
	_ = ctx
	state, err := workspacepkg.LoadTodoState()
	if err != nil {
		return toolErrorResult(err)
	}
	if state.ActivePlan != nil {
		workspacepkg.ArchiveActivePlan(&state)
	}
	if err := workspacepkg.SaveTodoState(state); err != nil {
		return toolErrorResult(err)
	}
	return toolSuccessResult(map[string]any{
		"path":        filepath.ToSlash(workspacepkg.TodoJSONRelativePath),
		"plan_status": "empty",
		"notice":      "我已经收起当前待办计划。",
	})
}

func executeReadFileTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = ctx
	var args readFileArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}

	absPath, err := resolveSafeToolPath(args.Path, false)
	if err != nil {
		return toolErrorResult(err)
	}

	mode := strings.TrimSpace(strings.ToLower(args.Mode))
	if mode == "" {
		mode = "bytes"
	}
	switch mode {
	case "bytes":
		return readFileBytes(absPath, args)
	case "lines":
		return readFileLines(absPath, args)
	default:
		return toolErrorResult(fmt.Errorf("unsupported mode: %s", mode))
	}
}

func readFileBytes(absPath string, args readFileArgs) nativeToolExecution {
	offset := args.Offset
	if offset < 0 {
		return toolErrorResult(fmt.Errorf("offset must be >= 0"))
	}

	length := args.Length
	if args.MaxBytes > 0 {
		length = int64(args.MaxBytes)
	}
	if length <= 0 || length > toolReadMaxBytes {
		length = toolReadMaxBytes
	}

	f, err := os.Open(absPath)
	if err != nil {
		return toolErrorResult(err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return toolErrorResult(err)
	}
	if info.IsDir() {
		return toolErrorResult(fmt.Errorf("path is a directory"))
	}

	sample, err := sniffFile(f, 512)
	if err != nil {
		return toolErrorResult(err)
	}
	if isBinaryToolData(sample) {
		return toolErrorResult(fmt.Errorf("file appears to be binary; use mode=bytes only for byte inspection and keep length small"))
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return toolErrorResult(fmt.Errorf("failed to seek to offset %d: %w", offset, err))
	}

	probe := make([]byte, length+1)
	n, err := io.ReadFull(f, probe)
	if err != nil && err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
		return toolErrorResult(err)
	}

	hasMore := int64(n) > length
	data := probe[:minInt64(int64(n), length)]
	if len(data) == 0 {
		return toolSuccessResult(map[string]any{
			"state":   "complete",
			"path":    absPath,
			"mode":    "bytes",
			"content": "",
			"notice":  "END OF FILE - no content at this offset",
		})
	}

	readEnd := offset + int64(len(data))
	notice := "END OF FILE - no further content."
	if hasMore {
		notice = fmt.Sprintf("TRUNCATED - call read_file again with mode=bytes and offset=%d to continue.", readEnd)
	}

	state := "complete"
	if hasMore {
		state = "truncated"
	}
	return toolSuccessResult(map[string]any{
		"state":       state,
		"path":        absPath,
		"mode":        "bytes",
		"total_bytes": info.Size(),
		"offset":      offset,
		"length":      len(data),
		"content":     string(data),
		"truncated":   hasMore,
		"notice":      notice,
	})
}

func readFileLines(absPath string, args readFileArgs) nativeToolExecution {
	startLine := args.StartLine
	if startLine <= 0 {
		startLine = 1
	}
	maxLines := args.MaxLines
	if maxLines <= 0 {
		maxLines = 80
	}

	f, err := os.Open(absPath)
	if err != nil {
		return toolErrorResult(err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return toolErrorResult(err)
	}
	if info.IsDir() {
		return toolErrorResult(fmt.Errorf("path is a directory"))
	}

	sample, err := sniffFile(f, 512)
	if err != nil {
		return toolErrorResult(err)
	}
	if isBinaryToolData(sample) {
		return toolErrorResult(fmt.Errorf("file appears to be binary; use mode=bytes for byte-based inspection"))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return toolErrorResult(err)
	}

	reader := bufio.NewReader(f)
	var out strings.Builder
	lineNo := int64(1)
	linesRead := int64(0)
	outputBytes := 0
	truncatedByBudget := false
	reachedEOF := false

	for lineNo < startLine {
		_, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			reachedEOF = true
			break
		}
		if err != nil {
			return toolErrorResult(err)
		}
		lineNo++
	}

	for !reachedEOF && linesRead < maxLines {
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			reachedEOF = true
		} else if err != nil {
			return toolErrorResult(err)
		}
		if line == "" && reachedEOF {
			break
		}
		formatted := fmt.Sprintf("%d|%s", lineNo, line)
		if outputBytes+len(formatted) > toolReadMaxBytes {
			remaining := toolReadMaxBytes - outputBytes
			if remaining > 0 {
				out.WriteString(formatted[:remaining])
				outputBytes += remaining
			}
			truncatedByBudget = true
			break
		}
		out.WriteString(formatted)
		outputBytes += len(formatted)
		linesRead++
		lineNo++
	}

	if out.Len() == 0 {
		return toolSuccessResult(map[string]any{
			"state":   "complete",
			"path":    absPath,
			"mode":    "lines",
			"content": "",
			"notice":  fmt.Sprintf("END OF FILE - no content at or after start_line=%d", startLine),
		})
	}

	notice := "END OF FILE - no further content."
	truncated := false
	if truncatedByBudget {
		truncated = true
		notice = fmt.Sprintf("TRUNCATED - byte budget reached. Call read_file again with mode=lines and start_line=%d to continue.", startLine+linesRead)
	} else if !reachedEOF {
		truncated = true
		notice = fmt.Sprintf("PARTIAL - more content remains. Call read_file again with mode=lines and start_line=%d and max_lines=%d to continue.", startLine+linesRead, maxLines)
	}

	state := "complete"
	if truncated {
		state = "truncated"
	}
	return toolSuccessResult(map[string]any{
		"state":      state,
		"path":       absPath,
		"mode":       "lines",
		"start_line": startLine,
		"lines_read": linesRead,
		"content":    out.String(),
		"truncated":  truncated,
		"notice":     notice,
	})
}

func executeWriteFileTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = ctx
	var args writeFileArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}
	if len(args.Content) > toolWriteMaxBytes {
		return toolErrorResult(fmt.Errorf("content too large"))
	}

	absPath, err := resolveSafeToolPath(args.Path, true)
	if err != nil {
		return toolErrorResult(err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return toolErrorResult(err)
	}

	mode := strings.TrimSpace(strings.ToLower(args.Mode))
	if mode == "" {
		mode = "overwrite"
	}
	overwrite := args.Overwrite || mode == "append"

	switch mode {
	case "overwrite":
		if _, err := os.Stat(absPath); err == nil && !overwrite {
			return toolErrorResult(fmt.Errorf("file already exists; set overwrite=true to replace it"))
		}
		if err := os.WriteFile(absPath, []byte(args.Content), 0o644); err != nil {
			return toolErrorResult(err)
		}
	case "append":
		f, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return toolErrorResult(err)
		}
		defer f.Close()
		if _, err := f.WriteString(args.Content); err != nil {
			return toolErrorResult(err)
		}
	default:
		return toolErrorResult(fmt.Errorf("unsupported mode: %s", mode))
	}

	return toolSuccessResult(map[string]any{
		"state":     "complete",
		"path":      absPath,
		"mode":      mode,
		"overwrite": overwrite,
		"bytes":     len(args.Content),
	})
}

func executeEditFileTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = ctx
	var args editFileArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}
	if strings.TrimSpace(args.Path) == "" {
		return toolErrorResult(fmt.Errorf("path is required"))
	}
	if args.OldText == "" {
		return toolErrorResult(fmt.Errorf("old_text is required"))
	}
	if len(args.OldText) > toolWriteMaxBytes || len(args.NewText) > toolWriteMaxBytes {
		return toolErrorResult(fmt.Errorf("edit text too large"))
	}

	absPath, err := resolveSafeToolPath(args.Path, false)
	if err != nil {
		return toolErrorResult(err)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return toolErrorResult(err)
	}
	if isBinaryToolData(data) {
		return toolErrorResult(fmt.Errorf("file appears to be binary; edit_file only supports UTF-8 text files"))
	}

	content := string(data)
	count := strings.Count(content, args.OldText)
	switch {
	case count == 0:
		return toolErrorResult(fmt.Errorf("old_text not found in file; make it match exactly"))
	case count > 1:
		return toolErrorResult(fmt.Errorf("old_text appears %d times; provide more surrounding context so the edit is unique", count))
	}

	updated := strings.Replace(content, args.OldText, args.NewText, 1)
	if err := os.WriteFile(absPath, []byte(updated), 0o644); err != nil {
		return toolErrorResult(err)
	}

	return toolSuccessResult(map[string]any{
		"state":       "complete",
		"path":        absPath,
		"edited":      true,
		"old_length":  len(args.OldText),
		"new_length":  len(args.NewText),
		"occurrences": count,
	})
}

func executeListFilesTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = ctx
	var args listFilesArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}
	path, err := resolveSafeToolDir(args.Path)
	if err != nil {
		return toolErrorResult(err)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return toolErrorResult(err)
	}
	items := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		item := map[string]any{
			"name":   entry.Name(),
			"is_dir": entry.IsDir(),
		}
		if info, err := entry.Info(); err == nil {
			item["size"] = info.Size()
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return fmt.Sprint(items[i]["name"]) < fmt.Sprint(items[j]["name"])
	})
	return toolSuccessResult(map[string]any{
		"state":   "complete",
		"path":    path,
		"entries": items,
	})
}

func executeRunCommandTool(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
	_ = ctx
	var args runCommandArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return toolErrorResult(err)
	}
	command := strings.TrimSpace(args.Command)
	if command == "" {
		command = strings.TrimSpace(args.Cmd)
	}
	if command == "" {
		return toolErrorResult(fmt.Errorf("command is required"))
	}
	cwd, err := resolveSafeToolDir(args.Cwd)
	if err != nil {
		return toolErrorResult(err)
	}
	if len(args.Args) > 0 {
		extra := make([]string, 0, len(args.Args))
		for _, arg := range args.Args {
			arg = strings.TrimSpace(arg)
			if arg == "" {
				continue
			}
			if len(arg) > 200 {
				return toolErrorResult(fmt.Errorf("argument too long"))
			}
			extra = append(extra, arg)
		}
		if len(extra) > 0 {
			command = command + " " + strings.Join(extra, " ")
		}
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "sh", "-lc", command)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	truncated := false
	if len(output) > toolCommandOutputLimit {
		output = output[:toolCommandOutputLimit]
		truncated = true
	}
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	state := "complete"
	if truncated {
		state = "truncated"
	}
	return toolJSONResult(map[string]any{
		"status":    status,
		"state":     state,
		"command":   command,
		"cwd":       cwd,
		"output":    string(output),
		"exit_code": exitCode,
		"truncated": truncated,
		"error":     errorString(err),
	})
}

func toolErrorResult(err error) nativeToolExecution {
	return toolJSONResult(map[string]any{
		"status": "error",
		"state":  "failed",
		"error":  errorString(err),
	})
}

func toolSuccessResult(payload map[string]any) nativeToolExecution {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["status"] = "ok"
	if _, ok := payload["state"]; !ok {
		payload["state"] = "complete"
	}
	return toolJSONResult(payload)
}

func toolJSONResult(payload map[string]any) nativeToolExecution {
	data, err := json.Marshal(payload)
	if err != nil {
		fallback := fmt.Sprintf(`{"status":"error","error":%q}`, err.Error())
		return nativeToolExecution{ResultContent: fallback, NeedsFollowUp: true}
	}
	return nativeToolExecution{ResultContent: string(data), NeedsFollowUp: true}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func localToolFollowupFromResults(toolMessages []openai.ChatCompletionMessage) string {
	if len(toolMessages) == 0 {
		return ""
	}

	type toolPayload struct {
		Status        string `json:"status"`
		State         string `json:"state"`
		Path          string `json:"path"`
		Mode          string `json:"mode"`
		Content       string `json:"content"`
		Notice        string `json:"notice"`
		Error         string `json:"error"`
		Datetime      string `json:"datetime"`
		HumanReadable string `json:"human_readable"`
		Summary       string `json:"summary"`
		Goal          string `json:"goal"`
		PlanStatus    string `json:"plan_status"`
		Count         int    `json:"count"`
	}

	var snippets []string
	for _, msg := range toolMessages {
		var payload toolPayload
		if err := json.Unmarshal([]byte(strings.TrimSpace(msg.Content)), &payload); err != nil {
			continue
		}
		switch payload.Status {
		case "error":
			if strings.TrimSpace(payload.Error) != "" {
				snippets = append(snippets, "我读取工具结果时遇到错误："+strings.TrimSpace(payload.Error)+"。")
			}
		case "ok":
			if strings.TrimSpace(payload.Notice) != "" {
				snippets = append(snippets, strings.TrimSpace(payload.Notice))
				continue
			}
			if strings.TrimSpace(payload.Goal) != "" {
				snippets = append(snippets, "我看了当前待办，目标是："+strings.TrimSpace(payload.Goal)+"，状态是："+strings.TrimSpace(payload.PlanStatus)+"。")
				continue
			}
			if strings.TrimSpace(payload.HumanReadable) != "" {
				snippets = append(snippets, "我查了当前时间："+strings.TrimSpace(payload.HumanReadable)+"。")
				continue
			}
			if strings.TrimSpace(payload.Summary) != "" {
				snippets = append(snippets, "我查到天气了："+strings.TrimSpace(payload.Summary))
				continue
			}
			if strings.TrimSpace(msg.Name) == "cron_list" {
				snippets = append(snippets, fmt.Sprintf("我查了定时任务，现在一共有 %d 个。", payload.Count))
				continue
			}
			base := filepath.Base(strings.TrimSpace(payload.Path))
			text := strings.TrimSpace(payload.Content)
			if base == "" || text == "" {
				continue
			}
			if len(text) > 160 {
				text = text[:160]
			}
			switch {
			case base == "MEMORY.md":
				snippets = append(snippets, "我查了长期记忆文件，目前内容是："+text)
			case base == "USER.md":
				snippets = append(snippets, "我查了用户设定文件，目前内容是："+text)
			default:
				snippets = append(snippets, "我查了文件 "+base+"，内容片段是："+text)
			}
		case "scheduled":
			snippets = append(snippets, "我已经开始执行这个动作了。")
		}
	}

	if len(snippets) == 0 {
		return ""
	}
	return strings.Join(snippets, " ")
}

func createToolFollowup(ctx context.Context, c *openai.Client, baseReq openai.ChatCompletionRequest, messages []openai.ChatCompletionMessage) (string, error) {
	followupReq := withoutNativeTools(baseReq)
	followupReq.Stream = false
	followupReq.Messages = append([]openai.ChatCompletionMessage(nil), messages...)
	resp, err := c.CreateChatCompletion(ctx, followupReq)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return strings.TrimSpace(removeSpecialCharacters(resp.Choices[0].Message.Content)), nil
}

func sniffFile(f *os.File, n int) ([]byte, error) {
	buf := make([]byte, n)
	readN, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return buf[:readN], nil
}

func isBinaryToolData(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return true
	}
	contentType := http.DetectContentType(sample)
	if strings.HasPrefix(contentType, "text/") || strings.HasSuffix(contentType, "/json") || strings.HasSuffix(contentType, "+json") {
		return false
	}
	if !utf8.Valid(sample) {
		return true
	}
	controlChars := 0
	for _, b := range sample {
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' && b != '\f' && b != '\b' {
			controlChars++
		}
	}
	return float64(controlChars)/float64(len(sample)) > 0.1
}

func resolveSafeToolDir(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd, nil
	}
	return resolveSafeToolPath(input, false)
}

func resolveSafeToolPath(input string, allowCreate bool) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("path is required")
	}

	roots := allowedToolRoots()
	if len(roots) == 0 {
		return "", fmt.Errorf("no allowed tool roots configured")
	}

	for _, root := range roots {
		candidate := normalizeToolPathForRoot(input, root)
		resolved, ok, err := validateToolPathAgainstRoot(candidate, root, allowCreate)
		if err != nil {
			return "", err
		}
		if ok {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("path outside allowed roots")
}

func normalizeToolPathForRoot(input, root string) string {
	candidate := strings.TrimSpace(input)
	if filepath.IsAbs(candidate) {
		return filepath.Clean(candidate)
	}

	cleanRoot := filepath.Clean(root)
	cleanInput := filepath.Clean(candidate)
	if cleanInput == "workspace" {
		return cleanRoot
	}
	workspacePrefix := "workspace" + string(filepath.Separator)
	if strings.HasPrefix(cleanInput, workspacePrefix) {
		cleanInput = strings.TrimPrefix(cleanInput, workspacePrefix)
	}
	return filepath.Clean(filepath.Join(cleanRoot, cleanInput))
}

func validateToolPathAgainstRoot(candidate, root string, allowCreate bool) (string, bool, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false, err
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", false, err
	}
	if !isWithinRoot(candidateAbs, rootAbs) {
		return "", false, nil
	}

	rootResolved := rootAbs
	if resolved, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootResolved = resolved
	}

	candidateResolved, err := filepath.EvalSymlinks(candidateAbs)
	if err == nil {
		if !isWithinRoot(candidateResolved, rootResolved) {
			return "", false, fmt.Errorf("access denied: symlink resolves outside allowed roots")
		}
		return candidateAbs, true, nil
	}
	if !os.IsNotExist(err) {
		return "", false, err
	}
	if !allowCreate {
		return "", false, err
	}
	ancestorResolved, ancErr := resolveExistingAncestor(filepath.Dir(candidateAbs))
	if ancErr == nil && !isWithinRoot(ancestorResolved, rootResolved) {
		return "", false, fmt.Errorf("access denied: symlink resolves outside allowed roots")
	}
	return candidateAbs, true, nil
}

func resolveExistingAncestor(path string) (string, error) {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if filepath.Dir(current) == current {
			return "", os.ErrNotExist
		}
	}
}

func isWithinRoot(candidate, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}

func allowedToolRoots() []string {
	seen := map[string]struct{}{}
	var roots []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		roots = append(roots, path)
	}
	for _, root := range workspacepkg.WorkspaceRoots() {
		add(root)
	}
	if len(roots) == 0 {
		if wd, err := os.Getwd(); err == nil {
			add(wd)
		}
	}
	return roots
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
