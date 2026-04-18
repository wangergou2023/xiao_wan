package llm

import (
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

func TestCreateAIReqAddsNativeTools(t *testing.T) {
	originalConfig := vars.APIConfig
	t.Cleanup(func() {
		vars.APIConfig = originalConfig
	})

	vars.APIConfig.Knowledge.Enable = true
	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.Provider = "bigmodel"
	vars.APIConfig.Knowledge.Model = "glm-5.1"
	vars.APIConfig.Knowledge.SaveChat = false

	req := CreateAIReq("回家充电去", "test-esn", false, false)
	if len(req.Tools) != len(nativeToolDefinitions) {
		t.Fatalf("expected %d native tools, got %d", len(nativeToolDefinitions), len(req.Tools))
	}
	if req.ToolChoice != "auto" {
		t.Fatalf("expected tool_choice auto, got %#v", req.ToolChoice)
	}
	if req.ParallelToolCalls != false {
		t.Fatalf("expected parallel tool calls disabled, got %#v", req.ParallelToolCalls)
	}
}

func TestStreamedToolCallAccumulatorMergesChunks(t *testing.T) {
	acc := streamedToolCallAccumulator{}
	idx := 0

	acc.AddDelta([]openai.ToolCall{
		{
			Index: &idx,
			ID:    "call_1",
			Type:  openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name: "go",
			},
		},
	})
	acc.AddDelta([]openai.ToolCall{
		{
			Index: &idx,
			Function: openai.FunctionCall{
				Name:      "Charge",
				Arguments: "{}",
			},
		},
	})

	calls := acc.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "goCharge" {
		t.Fatalf("expected merged function name goCharge, got %q", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments != "{}" {
		t.Fatalf("expected merged arguments, got %q", calls[0].Function.Arguments)
	}
}
