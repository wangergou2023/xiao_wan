package llm

import (
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
)

type nativeToolDefinition struct {
	Name        string
	Description string
	Action      int
}

var nativeToolDefinitions = []nativeToolDefinition{
	{
		Name:        "goCharge",
		Description: "Send the robot back to its charger right now.",
		Action:      ActionGoCharge,
	},
	{
		Name:        "takePhoto",
		Description: "Take a real photo and save it to the robot photo gallery right now.",
		Action:      ActionTakePhoto,
	},
	{
		Name:        "celebrateFireworks",
		Description: "Play the robot fireworks celebration animation right now.",
		Action:      ActionCelebrateFireworks,
	},
	{
		Name:        "backAway",
		Description: "Make the robot back away a short distance right now.",
		Action:      ActionBackAway,
	},
}

func buildNativeTools() []openai.Tool {
	tools := make([]openai.Tool, 0, len(nativeToolDefinitions))
	for _, def := range nativeToolDefinitions {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        def.Name,
				Description: def.Description,
				Parameters: map[string]any{
					"type":                 "object",
					"properties":           map[string]any{},
					"additionalProperties": false,
				},
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

func nativeToolAction(name string) (RobotAction, bool) {
	for _, def := range nativeToolDefinitions {
		if name == def.Name {
			return RobotAction{
				Action:    def.Action,
				Parameter: "now",
			}, true
		}
	}
	return RobotAction{}, false
}

func nativeToolCallsToDeferredActions(toolCalls []openai.ToolCall, robot *vector.Vector) ([]func(), []openai.ChatCompletionMessage) {
	var deferred []func()
	var toolResults []openai.ChatCompletionMessage
	for _, call := range toolCalls {
		action, ok := nativeToolAction(strings.TrimSpace(call.Function.Name))
		if !ok {
			logger.Println("LLM tried to call an unknown native tool: " + call.Function.Name)
			if call.ID != "" {
				toolResults = append(toolResults, openai.ChatCompletionMessage{
					Role:       "tool",
					ToolCallID: call.ID,
					Name:       strings.TrimSpace(call.Function.Name),
					Content:    `{"status":"unknown_tool"}`,
				})
			}
			continue
		}

		switch action.Action {
		case ActionGoCharge:
			deferred = append(deferred, func() {
				_ = DoGoCharge(robot)
			})
		case ActionTakePhoto:
			deferred = append(deferred, func() {
				_ = DoTakePhoto(robot)
			})
		case ActionCelebrateFireworks:
			deferred = append(deferred, func() {
				_ = DoCelebrateFireworks(robot)
			})
		case ActionBackAway:
			deferred = append(deferred, func() {
				_ = DoBackAway(robot)
			})
		}

		if call.ID != "" {
			toolResults = append(toolResults, openai.ChatCompletionMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       strings.TrimSpace(call.Function.Name),
				Content:    `{"status":"scheduled"}`,
			})
		}
	}
	return deferred, toolResults
}
