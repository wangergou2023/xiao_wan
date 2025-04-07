package chat

import (
	"context"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/config"
)

func OpenAIchat(userText string) (string, error) {

	var cfg = config.New()

	// 配置OpenAI API的密钥和中转地址
	cfg = cfg.SetOpenAiAPIKey(vars.APIConfig.Knowledge.Key)
	cfg = cfg.SetOpenAibaseURL(vars.APIConfig.Knowledge.Endpoint)

	config := openai.DefaultConfig(cfg.OpenAiAPIKey())
	config.BaseURL = cfg.OpenAibaseURL()

	client := openai.NewClientWithConfig(config)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userText,
				},
			},
		},
	)

	if err != nil {
		logger.Println("Error:", err)
		return "", nil
	}

	logger.Println(resp.Choices[0].Message.Content)

	return "", nil
}
