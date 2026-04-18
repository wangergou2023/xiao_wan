package llm

import (
	"path/filepath"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

func TestRememberPersistsChatToDisk(t *testing.T) {
	tempDir := t.TempDir()
	oldConfigPath := vars.ApiConfigPath
	oldChats := vars.RememberedChats
	vars.ApiConfigPath = filepath.Join(tempDir, "apiConfig.json")
	vars.RememberedChats = nil
	t.Cleanup(func() {
		vars.ApiConfigPath = oldConfigPath
		vars.RememberedChats = oldChats
	})

	Remember(
		openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "你好"},
		openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "你好呀"},
		"0dd1c497",
	)

	vars.RememberedChats = nil
	chat := GetChat("0dd1c497")
	if len(chat.Chats) != 2 {
		t.Fatalf("expected persisted chat messages, got %d", len(chat.Chats))
	}
	if chat.Chats[0].Content != "你好" || chat.Chats[1].Content != "你好呀" {
		t.Fatalf("unexpected persisted chat contents: %#v", chat.Chats)
	}
}
