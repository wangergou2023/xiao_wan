package llm

import (
	"strings"
	"testing"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

func TestCreatePromptBuildsLayeredRuntimeSections(t *testing.T) {
	originalConfig := vars.APIConfig
	t.Cleanup(func() {
		vars.APIConfig = originalConfig
	})

	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.SaveChat = true

	prompt := CreatePrompt("BASE", "glm-5.1", true)

	checks := []string{
		"BASE",
		"Runtime voice rules:",
		"Workspace guidance:",
		"Robot runtime tools and expression rules:",
		"Long-term memory lives in `workspace/memory/MEMORY.md`.",
		"Conversation mode rules:",
		"Valid legacy command catalog:",
	}
	for _, want := range checks {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}

func TestCreatePromptNonConversationModeDisablesNewVoiceRequest(t *testing.T) {
	originalConfig := vars.APIConfig
	t.Cleanup(func() {
		vars.APIConfig = originalConfig
	})

	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.SaveChat = false

	prompt := CreatePrompt("BASE", "glm-5.1", false)
	want := "You are not in conversation mode. Do not ask follow-up questions and do not use newVoiceRequest."
	if !strings.Contains(prompt, want) {
		t.Fatalf("expected non-conversation rule in prompt, got:\n%s", prompt)
	}
}
