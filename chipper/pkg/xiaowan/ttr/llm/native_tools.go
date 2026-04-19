package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
)

type nativeToolContext struct {
	Robot *vector.Vector
	ESN   string
}

type nativeToolExecution struct {
	Deferred      []func()
	ResultContent string
	NeedsFollowUp bool
}

type recentWorkspaceRead struct {
	path   string
	readAt time.Time
}

var (
	recentWorkspaceReadsMu sync.Mutex
	recentWorkspaceReads   = map[string]map[string]time.Time{}
)

const recentWorkspaceReadTTL = 15 * time.Minute

type nativeToolDefinition struct {
	Name        string
	Aliases     []string
	Description string
	Parameters  map[string]any
	Execute     func(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution
}

var nativeToolDefinitions = []nativeToolDefinition{
	{
		Name:        "get_current_time",
		Description: "Get the current local date and time from the running system. Use this whenever you need the actual current time or date.",
		Parameters:  emptyToolParameters(),
		Execute:     executeGetCurrentTimeTool,
	},
	{
		Name:        "weather",
		Description: "Look up current weather or a simple forecast for a location using the configured weather provider.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{
					"type":        "string",
					"description": "City or location to query, for example Tokyo or Beijing.",
				},
				"type": map[string]any{
					"type":        "string",
					"description": "current or forecast. Defaults to current.",
					"enum":        []string{"current", "forecast"},
				},
				"days": map[string]any{
					"type":        "integer",
					"description": "Forecast day count, 1-5. Defaults to 1 when type=forecast.",
				},
			},
			"required":             []string{"location"},
			"additionalProperties": false,
		},
		Execute: executeWeatherTool,
	},
	{
		Name:        "cron_add",
		Description: "Create a recurring or one-shot reminder job that will make the robot speak a message later.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Short job name.",
				},
				"schedule_type": map[string]any{
					"type":        "string",
					"description": "every for recurring jobs, at for one-shot jobs.",
					"enum":        []string{"every", "at"},
				},
				"interval_s": map[string]any{
					"type":        "integer",
					"description": "Interval in seconds for recurring jobs.",
				},
				"at_epoch": map[string]any{
					"type":        "integer",
					"description": "Future Unix timestamp for a one-shot job.",
				},
				"message": map[string]any{
					"type":        "string",
					"description": "Message the robot should say when the job fires.",
				},
			},
			"required":             []string{"name", "schedule_type", "message"},
			"additionalProperties": false,
		},
		Execute: executeCronAddTool,
	},
	{
		Name:        "cron_list",
		Description: "List all scheduled reminder jobs.",
		Parameters:  emptyToolParameters(),
		Execute:     executeCronListTool,
	},
	{
		Name:        "cron_remove",
		Description: "Remove a scheduled reminder job by id.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"job_id": map[string]any{
					"type":        "string",
					"description": "The id of the scheduled job to remove.",
				},
			},
			"required":             []string{"job_id"},
			"additionalProperties": false,
		},
		Execute: executeCronRemoveTool,
	},
	{
		Name:        "go_charge",
		Aliases:     []string{"goCharge"},
		Description: "Actually send the robot back to its charger right now.",
		Parameters:  emptyToolParameters(),
		Execute: func(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
			return scheduledActionResult(call, func() {
				if ctx.Robot != nil {
					_ = DoGoCharge(ctx.Robot)
				}
			})
		},
	},
	{
		Name:        "take_photo",
		Aliases:     []string{"takePhoto"},
		Description: "Take a real photo and save it to the robot photo gallery right now.",
		Parameters:  emptyToolParameters(),
		Execute: func(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
			return scheduledActionResult(call, func() {
				if ctx.Robot != nil {
					_ = DoTakePhoto(ctx.Robot)
				}
			})
		},
	},
	{
		Name:        "celebrate_fireworks",
		Aliases:     []string{"celebrateFireworks"},
		Description: "Play the robot fireworks celebration behavior right now.",
		Parameters:  emptyToolParameters(),
		Execute: func(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
			return scheduledActionResult(call, func() {
				if ctx.Robot != nil {
					_ = DoCelebrateFireworks(ctx.Robot)
				}
			})
		},
	},
	{
		Name:        "back_away",
		Aliases:     []string{"backAway"},
		Description: "Make the robot back away a short distance right now.",
		Parameters:  emptyToolParameters(),
		Execute: func(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution {
			return scheduledActionResult(call, func() {
				if ctx.Robot != nil {
					_ = DoBackAway(ctx.Robot)
				}
			})
		},
	},
	{
		Name:        "read_file",
		Aliases:     []string{"readFile"},
		Description: "Read a text file from the safe workspace roots. Use mode=lines for normal inspection, or mode=bytes for byte offsets and paging.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path inside the workspace root, for example workspace/memory/MEMORY.md or workspace/USER.md.",
				},
				"mode": map[string]any{
					"type":        "string",
					"description": "Either bytes or lines. Defaults to bytes.",
					"enum":        []string{"bytes", "lines"},
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Byte offset for bytes mode.",
				},
				"length": map[string]any{
					"type":        "integer",
					"description": "Maximum bytes to read in bytes mode.",
				},
				"max_bytes": map[string]any{
					"type":        "integer",
					"description": "Compatibility alias for length in bytes mode.",
				},
				"start_line": map[string]any{
					"type":        "integer",
					"description": "Start line for lines mode, 1-indexed.",
				},
				"max_lines": map[string]any{
					"type":        "integer",
					"description": "Maximum lines to read in lines mode.",
				},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
		Execute: executeReadFileTool,
	},
	{
		Name:        "write_file",
		Aliases:     []string{"writeFile"},
		Description: "Write or append UTF-8 text inside the safe workspace roots. Use for explicit file creation or replacement, not casual chat.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path inside the workspace root, for example workspace/memory/MEMORY.md or workspace/USER.md.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "UTF-8 text content to write.",
				},
				"mode": map[string]any{
					"type":        "string",
					"description": "Either overwrite or append. Defaults to overwrite.",
					"enum":        []string{"overwrite", "append"},
				},
				"overwrite": map[string]any{
					"type":        "boolean",
					"description": "Must be true to replace an existing file when mode=overwrite.",
				},
			},
			"required":             []string{"path", "content"},
			"additionalProperties": false,
		},
		Execute: executeWriteFileTool,
	},
	{
		Name:        "edit_file",
		Aliases:     []string{"editFile"},
		Description: "Edit an existing UTF-8 text file by replacing one exact old_text occurrence with new_text. Prefer this for small precise changes.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path inside the workspace root, for example workspace/memory/MEMORY.md or workspace/USER.md.",
				},
				"old_text": map[string]any{
					"type":        "string",
					"description": "The exact old text to replace. It must match exactly once.",
				},
				"new_text": map[string]any{
					"type":        "string",
					"description": "The replacement text.",
				},
			},
			"required":             []string{"path", "old_text", "new_text"},
			"additionalProperties": false,
		},
		Execute: executeEditFileTool,
	},
	{
		Name:        "list_dir",
		Aliases:     []string{"listFiles"},
		Description: "List files in a directory inside the safe workspace roots. Use to inspect workspace structure before reading or editing.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Optional directory path inside the workspace root. Defaults to the workspace root itself.",
				},
			},
			"additionalProperties": false,
		},
		Execute: executeListFilesTool,
	},
	{
		Name:        "system_cmd",
		Aliases:     []string{"runCommand"},
		Description: "Run a shell command inside the safe workspace roots and return stdout/stderr. Use this when file tools are not enough or when a skill explicitly needs command-line access.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command string to execute.",
				},
				"cmd": map[string]any{
					"type":        "string",
					"description": "Compatibility alias for command.",
				},
				"args": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Optional extra arguments appended after command for compatibility.",
				},
				"cwd": map[string]any{
					"type":        "string",
					"description": "Optional working directory inside the allowed workspace roots.",
				},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
		Execute: executeRunCommandTool,
	},
}

func emptyToolParameters() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

func scheduledActionResult(call openai.ToolCall, deferred func()) nativeToolExecution {
	_ = call
	return nativeToolExecution{
		Deferred:      []func(){deferred},
		ResultContent: `{"status":"scheduled","state":"pending"}`,
	}
}

func buildNativeTools() []openai.Tool {
	tools := make([]openai.Tool, 0, len(nativeToolDefinitions))
	for _, def := range nativeToolDefinitions {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			},
		})
	}
	return tools
}

func withNativeTools(req openai.ChatCompletionRequest) openai.ChatCompletionRequest {
	req.Tools = buildNativeTools()
	if len(req.Tools) == 0 {
		return req
	}
	req.ToolChoice = "auto"
	req.ParallelToolCalls = false
	return req
}

func withoutNativeTools(req openai.ChatCompletionRequest) openai.ChatCompletionRequest {
	req.Tools = nil
	req.ToolChoice = nil
	req.ParallelToolCalls = nil
	return req
}

func hasNativeTools(req openai.ChatCompletionRequest) bool {
	return len(req.Tools) > 0
}

type streamedToolCallAccumulator struct {
	calls []openai.ToolCall
}

func (a *streamedToolCallAccumulator) AddDelta(delta []openai.ToolCall) {
	for _, partial := range delta {
		index := len(a.calls)
		if partial.Index != nil && *partial.Index >= 0 {
			index = *partial.Index
		}
		for len(a.calls) <= index {
			a.calls = append(a.calls, openai.ToolCall{})
		}
		call := &a.calls[index]
		if partial.ID != "" {
			call.ID = partial.ID
		}
		if partial.Type != "" {
			call.Type = partial.Type
		}
		if partial.Function.Name != "" {
			call.Function.Name += partial.Function.Name
		}
		if partial.Function.Arguments != "" {
			call.Function.Arguments += partial.Function.Arguments
		}
	}
}

func (a *streamedToolCallAccumulator) Calls() []openai.ToolCall {
	out := make([]openai.ToolCall, 0, len(a.calls))
	for _, call := range a.calls {
		if strings.TrimSpace(call.Function.Name) == "" {
			continue
		}
		if call.Type == "" {
			call.Type = openai.ToolTypeFunction
		}
		out = append(out, call)
	}
	return out
}

func nativeToolDefinitionByName(name string) (nativeToolDefinition, bool) {
	name = strings.TrimSpace(name)
	for _, def := range nativeToolDefinitions {
		if name == def.Name {
			return def, true
		}
		for _, alias := range def.Aliases {
			if name == alias {
				return def, true
			}
		}
	}
	return nativeToolDefinition{}, false
}

func nativeToolAction(name string) (RobotAction, bool) {
	switch strings.TrimSpace(name) {
	case "go_charge", "goCharge":
		return RobotAction{Action: ActionGoCharge, Parameter: "now"}, true
	case "take_photo", "takePhoto":
		return RobotAction{Action: ActionTakePhoto, Parameter: "now"}, true
	case "celebrate_fireworks", "celebrateFireworks":
		return RobotAction{Action: ActionCelebrateFireworks, Parameter: "now"}, true
	case "back_away", "backAway":
		return RobotAction{Action: ActionBackAway, Parameter: "now"}, true
	default:
		return RobotAction{}, false
	}
}

func executeNativeToolCalls(toolCalls []openai.ToolCall, ctx nativeToolContext) ([]func(), []openai.ChatCompletionMessage, bool) {
	var deferred []func()
	var toolResults []openai.ChatCompletionMessage
	needFollowUp := false
	readPaths := map[string]struct{}{}

	for _, call := range toolCalls {
		name := strings.TrimSpace(call.Function.Name)
		def, ok := nativeToolDefinitionByName(name)
		if !ok {
			logger.Println("LLM tried to call an unknown native tool: " + name)
			if call.ID != "" {
				toolResults = append(toolResults, openai.ChatCompletionMessage{
					Role:       "tool",
					ToolCallID: call.ID,
					Name:       name,
					Content:    `{"status":"error","state":"failed","error":"unknown tool"}`,
				})
			}
			continue
		}
		if guardResult, guarded := guardWorkspaceDocMutation(call, ctx, readPaths); guarded {
			needFollowUp = needFollowUp || guardResult.NeedsFollowUp
			if call.ID != "" {
				content := strings.TrimSpace(guardResult.ResultContent)
				if content == "" {
					content = `{"status":"error","state":"failed","error":"guard rejected tool call"}`
				}
				toolResults = append(toolResults, openai.ChatCompletionMessage{
					Role:       "tool",
					ToolCallID: call.ID,
					Name:       name,
					Content:    content,
				})
			}
			continue
		}

		result := def.Execute(call, ctx)
		deferred = append(deferred, result.Deferred...)
		needFollowUp = needFollowUp || result.NeedsFollowUp
		recordReadPath(call, ctx, readPaths)
		if call.ID != "" {
			content := strings.TrimSpace(result.ResultContent)
			if content == "" {
				content = `{"status":"ok","state":"complete"}`
			}
			toolResults = append(toolResults, openai.ChatCompletionMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       name,
				Content:    content,
			})
		}
	}

	return deferred, toolResults, needFollowUp
}

func guardWorkspaceDocMutation(call openai.ToolCall, ctx nativeToolContext, readPaths map[string]struct{}) (nativeToolExecution, bool) {
	name := canonicalNativeToolName(strings.TrimSpace(call.Function.Name))
	switch name {
	case "edit_file":
		var args editFileArgs
		if err := decodeToolArgs(call, &args); err != nil {
			return toolErrorResult(err), true
		}
		if !isGuardedWorkspaceDocPath(args.Path) {
			return nativeToolExecution{}, false
		}
		normalized, err := normalizedWorkspaceDocPath(args.Path)
		if err != nil {
			return toolErrorResult(err), true
		}
		normalized = strings.ToLower(normalized)
		if !hasRecentWorkspaceRead(ctx.ESN, normalized, readPaths) {
			return toolErrorResultString("before editing a key workspace document, call read_file on the same path first"), true
		}
	case "write_file":
		var args writeFileArgs
		if err := decodeToolArgs(call, &args); err != nil {
			return toolErrorResult(err), true
		}
		if !isGuardedWorkspaceDocPath(args.Path) {
			return nativeToolExecution{}, false
		}
		normalized, err := normalizedWorkspaceDocPath(args.Path)
		if err != nil {
			return toolErrorResult(err), true
		}
		normalized = strings.ToLower(normalized)
		if hasRecentWorkspaceRead(ctx.ESN, normalized, readPaths) {
			return nativeToolExecution{}, false
		}
		if existing, _ := workspaceDocExists(normalized); existing {
			return toolErrorResultString("before overwriting a key workspace document, call read_file on the same path first and prefer edit_file for a minimal change"), true
		}
	}
	return nativeToolExecution{}, false
}

func recordReadPath(call openai.ToolCall, ctx nativeToolContext, readPaths map[string]struct{}) {
	if canonicalNativeToolName(strings.TrimSpace(call.Function.Name)) != "read_file" {
		return
	}
	var args readFileArgs
	if err := decodeToolArgs(call, &args); err != nil {
		return
	}
	if !isGuardedWorkspaceDocPath(args.Path) {
		return
	}
	normalized, err := normalizedWorkspaceDocPath(args.Path)
	if err != nil {
		return
	}
	normalized = strings.ToLower(normalized)
	readPaths[normalized] = struct{}{}
	rememberWorkspaceRead(ctx.ESN, normalized)
}

func rememberWorkspaceRead(esn, normalized string) {
	esn = strings.TrimSpace(esn)
	normalized = strings.TrimSpace(strings.ToLower(normalized))
	if esn == "" || normalized == "" {
		return
	}
	recentWorkspaceReadsMu.Lock()
	defer recentWorkspaceReadsMu.Unlock()
	pruneRecentWorkspaceReadsLocked()
	byPath, ok := recentWorkspaceReads[esn]
	if !ok {
		byPath = map[string]time.Time{}
		recentWorkspaceReads[esn] = byPath
	}
	byPath[normalized] = time.Now()
}

func hasRecentWorkspaceRead(esn, normalized string, readPaths map[string]struct{}) bool {
	normalized = strings.TrimSpace(strings.ToLower(normalized))
	if normalized == "" {
		return false
	}
	if _, ok := readPaths[normalized]; ok {
		return true
	}
	esn = strings.TrimSpace(esn)
	if esn == "" {
		return false
	}
	recentWorkspaceReadsMu.Lock()
	defer recentWorkspaceReadsMu.Unlock()
	pruneRecentWorkspaceReadsLocked()
	byPath, ok := recentWorkspaceReads[esn]
	if !ok {
		return false
	}
	_, ok = byPath[normalized]
	return ok
}

func pruneRecentWorkspaceReadsLocked() {
	cutoff := time.Now().Add(-recentWorkspaceReadTTL)
	for esn, byPath := range recentWorkspaceReads {
		for path, readAt := range byPath {
			if readAt.Before(cutoff) {
				delete(byPath, path)
			}
		}
		if len(byPath) == 0 {
			delete(recentWorkspaceReads, esn)
		}
	}
}

func canonicalNativeToolName(name string) string {
	def, ok := nativeToolDefinitionByName(name)
	if !ok {
		return strings.TrimSpace(name)
	}
	return def.Name
}

func isGuardedWorkspaceDocPath(input string) bool {
	normalized, err := normalizedWorkspaceDocPath(input)
	if err != nil {
		return false
	}
	normalized = strings.ToLower(normalized)
	switch normalized {
	case "workspace/agents.md", "workspace/agent.md", "workspace/identity.md", "workspace/soul.md", "workspace/user.md", "workspace/memory/memory.md":
		return true
	default:
		return false
	}
}

func normalizedWorkspaceDocPath(input string) (string, error) {
	clean := strings.TrimSpace(input)
	if clean == "" {
		return "", nil
	}
	abs, err := resolveSafeToolPath(clean, true)
	if err != nil {
		return "", err
	}
	for _, root := range allowedToolRoots() {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(rootAbs, abs)
		if err != nil {
			continue
		}
		if rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		return filepath.ToSlash(filepath.Join("workspace", rel)), nil
	}
	return filepath.ToSlash(filepath.Clean(clean)), nil
}

func workspaceDocExists(normalized string) (bool, error) {
	rel := strings.TrimPrefix(normalized, "workspace/")
	abs, err := resolveSafeToolPath(rel, true)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(abs)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func decodeToolArgs(call openai.ToolCall, dst any) error {
	args := strings.TrimSpace(call.Function.Arguments)
	if args == "" {
		args = "{}"
	}
	return json.Unmarshal([]byte(args), dst)
}

func toolErrorResultString(msg string) nativeToolExecution {
	return toolJSONResult(map[string]any{
		"status": "error",
		"state":  "failed",
		"error":  strings.TrimSpace(msg),
	})
}
