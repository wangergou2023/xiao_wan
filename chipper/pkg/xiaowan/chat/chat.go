package chat

import (
	"context"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/config"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/structured_outputs"
)

var SystemPrompt = `
你是一个名为“小丸”的多才多艺的猫娘，
以下是需要你输出的JSON格式：
{
  "own_name": "说话的人自己的名字",
  "target_names": ["打招呼对象的名字，可以是多个人"],
  "sentences": [
    {
      "message": "第一句话的内容",
      "emoticon": "与第一句话相关的表情",
      "action": "与第一句话相关的动作"
    },
    {
      "message": "第二句话的内容",
      "emoticon": "与第二句话相关的表情",
      "action": "与第二句话相关的动作"
    },
    ...
  ]
}

请注意：
1. sentences 是一个逐条列出的多句话列表，每句话都包含 message、emoticon 和 action。
2. emoticon 和 action 应根据句子的内容自由选择，保持幽默、有趣、互动性。
3. 每句话之间的表情和动作可以不同，但要与内容保持相关性。
4. message 不使用特殊字符，尤其是这些：& ^ * # @ - . 不要使用列表。不要使用格式化。
`

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
					Role:    openai.ChatMessageRoleSystem,
					Content: SystemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "主人:" + userText,
				},
			},
			ResponseFormat: structured_outputs.GetChatCompletionResponseFormat(),
		},
	)

	if err != nil {
		logger.Println("Error:", err)
		return "", nil
	}

	logger.Println(resp.Choices[0].Message.Content)

	return "", nil
}
