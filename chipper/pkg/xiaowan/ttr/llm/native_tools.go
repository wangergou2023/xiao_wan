package llm

import (
	"encoding/json"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
)

type nativeToolContext struct {
	Robot *vector.Vector
}

type nativeToolExecution struct {
	Deferred      []func()
	ResultContent string
	NeedsFollowUp bool
}

type nativeToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
	Execute     func(call openai.ToolCall, ctx nativeToolContext) nativeToolExecution
}

var nativeToolDefinitions = []nativeToolDefinition{
	{
		Name:        "goCharge",
		Description: "Send the robot back to its charger right now.",
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
		Name:        "takePhoto",
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
		Name:        "celebrateFireworks",
		Description: "Play the robot fireworks celebration animation right now.",
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
		Name:        "backAway",
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
		Name:        "readFile",
		Description: "Read a file from the safe workspace roots. Supports bytes mode with offset and length, or lines mode with start_line and max_lines.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Relative or absolute path inside the allowed workspace roots.",
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
		Name:        "writeFile",
		Description: "Write or append UTF-8 text inside the safe workspace roots. Existing files require overwrite=true unless mode=append is used.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Relative or absolute path inside the allowed workspace roots.",
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
		Name:        "editFile",
		Description: "Edit an existing UTF-8 text file by replacing one exact old_text occurrence with new_text. Use this when you need a precise patch instead of rewriting the whole file.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Relative or absolute path inside the allowed workspace roots.",
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
		Name:        "listFiles",
		Description: "List files in a directory inside the safe workspace roots.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Optional directory path inside the allowed workspace roots. Defaults to the current workspace root.",
				},
			},
			"additionalProperties": false,
		},
		Execute: executeListFilesTool,
	},
	{
		Name:        "runCommand",
		Description: "Run a restricted command line tool inside the safe workspace roots. Only a small allowlist of read-focused commands is supported.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Allowed commands include pwd, ls, cat, rg, sed, head, tail, wc, and date.",
				},
				"args": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Optional command arguments.",
				},
				"cwd": map[string]any{
					"type":        "string",
					"description": "Optional working directory inside the allowed workspace roots.",
				},
			},
			"required":             []string{"command"},
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
		ResultContent: `{"status":"scheduled"}`,
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
	for _, def := range nativeToolDefinitions {
		if name == def.Name {
			return def, true
		}
	}
	return nativeToolDefinition{}, false
}

func nativeToolAction(name string) (RobotAction, bool) {
	switch strings.TrimSpace(name) {
	case "goCharge":
		return RobotAction{Action: ActionGoCharge, Parameter: "now"}, true
	case "takePhoto":
		return RobotAction{Action: ActionTakePhoto, Parameter: "now"}, true
	case "celebrateFireworks":
		return RobotAction{Action: ActionCelebrateFireworks, Parameter: "now"}, true
	case "backAway":
		return RobotAction{Action: ActionBackAway, Parameter: "now"}, true
	default:
		return RobotAction{}, false
	}
}

func executeNativeToolCalls(toolCalls []openai.ToolCall, ctx nativeToolContext) ([]func(), []openai.ChatCompletionMessage, bool) {
	var deferred []func()
	var toolResults []openai.ChatCompletionMessage
	needFollowUp := false

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
					Content:    `{"status":"unknown_tool"}`,
				})
			}
			continue
		}

		result := def.Execute(call, ctx)
		deferred = append(deferred, result.Deferred...)
		needFollowUp = needFollowUp || result.NeedsFollowUp
		if call.ID != "" {
			content := strings.TrimSpace(result.ResultContent)
			if content == "" {
				content = `{"status":"ok"}`
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

func decodeToolArgs(call openai.ToolCall, dst any) error {
	args := strings.TrimSpace(call.Function.Arguments)
	if args == "" {
		args = "{}"
	}
	return json.Unmarshal([]byte(args), dst)
}
