package llm

import (
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

func TestCreateAIReqForcesCelebrateFireworksTool(t *testing.T) {
	originalConfig := vars.APIConfig
	t.Cleanup(func() {
		vars.APIConfig = originalConfig
	})

	vars.APIConfig.Knowledge.Enable = true
	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.Provider = "bigmodel"
	vars.APIConfig.Knowledge.Model = "glm-4.7"
	vars.APIConfig.Knowledge.SaveChat = false

	req := CreateAIReq("放烟花", "test-esn", false, false)
	tc, ok := req.ToolChoice.(openai.ToolChoice)
	if !ok {
		t.Fatalf("expected concrete tool choice, got %#v", req.ToolChoice)
	}
	if tc.Function.Name != "celebrate_fireworks" {
		t.Fatalf("expected celebrate_fireworks tool choice, got %#v", tc)
	}

	foundPrompt := false
	for _, msg := range req.Messages {
		if msg.Role == openai.ChatMessageRoleSystem && strings.Contains(msg.Content, "must call the native tool `celebrate_fireworks`") {
			foundPrompt = true
			break
		}
	}
	if !foundPrompt {
		t.Fatalf("expected forced direct-action system prompt, got %#v", req.Messages)
	}
}

func TestCreateAIReqForcesChargeTool(t *testing.T) {
	originalConfig := vars.APIConfig
	t.Cleanup(func() {
		vars.APIConfig = originalConfig
	})

	vars.APIConfig.Knowledge.Enable = true
	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.Provider = "bigmodel"
	vars.APIConfig.Knowledge.Model = "glm-4.7"
	vars.APIConfig.Knowledge.SaveChat = false

	req := CreateAIReq("回家充电去吧", "test-esn", false, false)
	tc, ok := req.ToolChoice.(openai.ToolChoice)
	if !ok {
		t.Fatalf("expected concrete tool choice, got %#v", req.ToolChoice)
	}
	if tc.Function.Name != "go_charge" {
		t.Fatalf("expected go_charge tool choice, got %#v", tc)
	}
}

func TestCreateAIReqKeepsAutoToolChoiceForComplexTask(t *testing.T) {
	originalConfig := vars.APIConfig
	t.Cleanup(func() {
		vars.APIConfig = originalConfig
	})

	vars.APIConfig.Knowledge.Enable = true
	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.Provider = "bigmodel"
	vars.APIConfig.Knowledge.Model = "glm-4.7"
	vars.APIConfig.Knowledge.SaveChat = false

	req := CreateAIReq("帮我记住我喜欢吃苹果并更新长期记忆", "test-esn", false, false)
	if req.ToolChoice != "auto" {
		t.Fatalf("expected auto tool choice for complex non-direct task, got %#v", req.ToolChoice)
	}
}
