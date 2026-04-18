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
			Name:    "read_file",
			Content: `{"status":"ok","state":"complete","path":"/tmp/workspace/memory/MEMORY.md","content":"No durable user facts have been confirmed yet."}`,
		},
		{
			Role:    openai.ChatMessageRoleTool,
			Name:    "read_file",
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

func TestLocalToolFollowupFromResultsSummarizesTimeAndWeather(t *testing.T) {
	text := localToolFollowupFromResults([]openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleTool,
			Name:    "get_current_time",
			Content: `{"status":"ok","state":"complete","human_readable":"2026-04-19 10:11:12 CST"}`,
		},
		{
			Role:    openai.ChatMessageRoleTool,
			Name:    "weather",
			Content: `{"status":"ok","state":"complete","summary":"Tokyo weather now: Cloudy, 18°C. Local time: 2026-04-19 10:00:00 JST."}`,
		},
	})

	if !strings.Contains(text, "当前时间") {
		t.Fatalf("expected local follow-up to mention current time, got %q", text)
	}
	if !strings.Contains(text, "天气") {
		t.Fatalf("expected local follow-up to mention weather, got %q", text)
	}
}

func TestLocalToolFollowupFromResultsSummarizesCronList(t *testing.T) {
	text := localToolFollowupFromResults([]openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleTool,
			Name:    "cron_list",
			Content: `{"status":"ok","state":"complete","count":2,"jobs":[]}`,
		},
	})

	if !strings.Contains(text, "定时任务") {
		t.Fatalf("expected local follow-up to mention cron jobs, got %q", text)
	}
}
