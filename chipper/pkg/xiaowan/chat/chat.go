package chat

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/sashabaranov/go-openai"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/kercre123/wire-pod/chipper/pkg/xiaowan/config"
	"github.com/kercre123/wire-pod/chipper/pkg/xiaowan/structured_outputs"
)

var systemPrompt = `
你需要了解一些关于系统和宿主的小说。
这类小说通常涉及一个主角（宿主）获取一个强大的系统，从而在某个世界中变得更加强大或实现特定目标。
这些系统可能提供任务、奖励、技能提升等元素，帮助宿主在剧情中取得优势。
这类作品包括了不同的题材，如玄幻、科幻、都市、历史等，比较受欢迎的作品有《全职高手》（虽然其主要内容并非系统文，但也包含一些类似概念的元素），以及一些网络小说如《史上最强店主》、《无限恐怖》等。
请你现在扮演系统。宿主是一个人类，宿主的名字是“宿主”，你需要根据宿主的名字和打招呼对象的名字，生成一段幽默、有趣、互动性强的对话。
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

	// OpenAI 配置
	cfg = cfg.SetOpenAiAPIKey(vars.APIConfig.Knowledge.Key)
	cfg = cfg.SetOpenAibaseURL(vars.APIConfig.Knowledge.Endpoint)

	openaiCfg := openai.DefaultConfig(cfg.OpenAiAPIKey())
	openaiCfg.BaseURL = cfg.OpenAibaseURL()

	client := openai.NewClientWithConfig(openaiCfg)

	// 1️⃣ 读取并编码图片
	base64Image, err := encodeImageToBase64("camera.jpg")
	if err != nil {
		return "", err
	}

	// ⚠️ mimeType 一定要是 image/jpeg 或 image/png
	dataURL := createDataURL(base64Image, "image/jpeg")

	// 2️⃣ 构造多模态 User 消息
	userMessage := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{
				Type: openai.ChatMessagePartTypeText,
				Text: "宿主:" + userText,
			},
			{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL:    dataURL,
					Detail: openai.ImageURLDetailAuto,
				},
			},
		},
	}

	// 3️⃣ 调用 OpenAI
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: vars.APIConfig.Knowledge.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				userMessage,
			},
			ResponseFormat: structured_outputs.GetChatCompletionResponseFormat(),
			MaxTokens:      500,
		},
	)

	if err != nil {
		logger.Println("OpenAI error:", err)
		return "", err
	}

	logger.Println(resp.Choices[0].Message.Content)
	return resp.Choices[0].Message.Content, nil
}

// encodeImageToBase64 reads an image file and returns a base64 encoded string
func encodeImageToBase64(filePath string) (string, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read the file content into a byte slice
	var buf bytes.Buffer
	_, err = io.Copy(&buf, file)
	if err != nil {
		return "", err
	}

	// Encode the file content to base64
	encodedString := base64.StdEncoding.EncodeToString(buf.Bytes())
	return encodedString, nil
}

// createDataURL creates a data URL from the base64 encoded image
func createDataURL(base64Image string, mimeType string) string {
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)
}
