package llm

import (
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestLocalToolFollowupFromResultsSummarizesMemoryReads(t *testing.T) {
	text := localToolFollowupFromResults([]openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleTool,
			Name:    "readFile",
			Content: `{"status":"ok","state":"complete","path":"/tmp/workspace/memory/MEMORY.md","content":"No durable user facts have been confirmed yet."}`,
		},
		{
			Role:    openai.ChatMessageRoleTool,
			Name:    "readFile",
			Content: `{"status":"ok","state":"complete","path":"/tmp/workspace/USER.md","content":"The robot should present itself as 小丸."}`,
		},
	})

	if !strings.Contains(text, "长期记忆文件") {
		t.Fatalf("expected local follow-up to mention memory file, got %q", text)
	}
	if !strings.Contains(text, "用户设定文件") {
		t.Fatalf("expected local follow-up to mention user file, got %q", text)
	}
}
