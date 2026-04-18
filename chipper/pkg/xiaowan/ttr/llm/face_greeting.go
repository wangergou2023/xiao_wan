package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	memorypkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/memory"
	robotpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/robot"
	visionpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/vision"
)

const passiveFaceGreetingPrompt = "This is a passive visual greeting trigger. You just recognized your owner standing in front of you. Reply with exactly one short warm spoken sentence in Chinese. Do not use any command syntax like double braces. Do not narrate actions. Do not ask more than one short question."

func init() {
	visionpkg.SetOwnerGreetingFunc(GenerateOwnerGreeting)
}

// GenerateOwnerGreeting 在识别到主人且机器人空闲时，用 LLM 生成一句更自然的专属问候。
func GenerateOwnerGreeting(esn, name string) error {
	logger.Println(fmt.Sprintf("Owner face greeting requested for %s name=%q", esn, name))
	text, err := requestOwnerGreeting(esn, name)
	if err != nil {
		return err
	}
	text = normalizeSpeechText(text)
	if text == "" {
		logger.Println(fmt.Sprintf("Owner face greeting for %s name=%q returned empty text", esn, name))
		return nil
	}
	logger.Println(fmt.Sprintf("Owner face greeting generated for %s name=%q: %q", esn, name, text))
	return robotpkg.KGSim(esn, text)
}

func requestOwnerGreeting(esn, name string) (string, error) {
	basePrompt := strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt)
	if basePrompt == "" {
		basePrompt = "You are a helpful, animated robot called Vector. Keep the response concise yet informative."
	}

	systemPrompt := basePrompt + "\n\n" + passiveFaceGreetingPrompt
	if profilePrompt := strings.TrimSpace(memorypkg.BuildPromptContext(esn)); profilePrompt != "" {
		systemPrompt += "\n\nLong-term user profile:\n" + profilePrompt
	}

	model := getKnowledgeModel(false)
	logKnowledgeModel(model)

	req := openai.ChatCompletionRequest{
		Model:       model,
		MaxTokens:   120,
		Temperature: 0.8,
		TopP:        1,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "You visually recognized the owner named " + name + " standing in front of you right now. Say one short welcoming sentence out loud in Chinese.",
			},
		},
	}

	logger.Println(fmt.Sprintf("Owner face greeting sending LLM request for %s name=%q", esn, name))
	resp, err := newKnowledgeClient().CreateChatCompletion(context.Background(), req)
	if err != nil {
		logger.Println("Owner face greeting request failed: " + err.Error())
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}
