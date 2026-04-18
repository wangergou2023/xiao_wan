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
)

const (
	toolReadMaxBytes       = 12 * 1024
	toolWriteMaxBytes      = 24 * 1024
	toolCommandOutputLimit = 12 * 1024
)

var allowedToolCommands = map[string]struct{}{
	"pwd":  {},
	"ls":   {},
	"cat":  {},
	"rg":   {},
	"sed":  {},
	"head": {},
	"tail": {},
	"wc":   {},
	"date": {},
}

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
	Args    []string `json:"args"`
	Cwd     string   `json:"cwd"`
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
		return toolJSONResult(map[string]any{
			"status":  "ok",
			"path":    absPath,
			"mode":    "bytes",
			"content": "",
			"notice":  "END OF FILE - no content at this offset",
		})
	}

	readEnd := offset + int64(len(data))
	notice := "END OF FILE - no further content."
	if hasMore {
		notice = fmt.Sprintf("TRUNCATED - call readFile again with mode=bytes and offset=%d to continue.", readEnd)
	}

	return toolJSONResult(map[string]any{
		"status":      "ok",
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
		return toolJSONResult(map[string]any{
			"status":  "ok",
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
		notice = fmt.Sprintf("TRUNCATED - byte budget reached. Call readFile again with mode=lines and start_line=%d to continue.", startLine+linesRead)
	} else if !reachedEOF {
		truncated = true
		notice = fmt.Sprintf("PARTIAL - more content remains. Call readFile again with mode=lines and start_line=%d and max_lines=%d to continue.", startLine+linesRead, maxLines)
	}

	return toolJSONResult(map[string]any{
		"status":     "ok",
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

	return toolJSONResult(map[string]any{
		"status":    "ok",
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
		return toolErrorResult(fmt.Errorf("file appears to be binary; editFile only supports UTF-8 text files"))
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

	return toolJSONResult(map[string]any{
		"status":      "ok",
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
	return toolJSONResult(map[string]any{
		"status":  "ok",
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
	if _, ok := allowedToolCommands[command]; !ok {
		return toolErrorResult(fmt.Errorf("command not allowed: %s", command))
	}
	cwd, err := resolveSafeToolDir(args.Cwd)
	if err != nil {
		return toolErrorResult(err)
	}
	cleanArgs := make([]string, 0, len(args.Args))
	for _, arg := range args.Args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		if len(arg) > 200 {
			return toolErrorResult(fmt.Errorf("argument too long"))
		}
		cleanArgs = append(cleanArgs, arg)
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, command, cleanArgs...)
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
	return toolJSONResult(map[string]any{
		"status":    status,
		"command":   command,
		"args":      cleanArgs,
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
		"error":  errorString(err),
	})
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

	candidate := input
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(roots[0], candidate)
	}
	candidate = filepath.Clean(candidate)

	for _, root := range roots {
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
	if wd, err := os.Getwd(); err == nil {
		add(wd)
		add(filepath.Join(wd, "workspace"))
		add(filepath.Join(wd, "chipper", "workspace"))
		parent := filepath.Dir(wd)
		add(filepath.Join(parent, "workspace"))
		add(filepath.Join(parent, "chipper", "workspace"))
	}
	if wirepodHome := strings.TrimSpace(os.Getenv("WIREPOD_HOME")); wirepodHome != "" {
		add(filepath.Join(wirepodHome, "workspace"))
		add(filepath.Join(wirepodHome, "chipper", "workspace"))
	}
	return roots
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
