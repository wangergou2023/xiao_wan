package tts

import (
	"context"
	"io"
	"os"

	openai "github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/config"
)

func OpenAItts(outputFile string, aiText string) {

	var cfg = config.New()
	var err error

	// 配置OpenAI API的密钥和中转地址
	cfg = cfg.SetOpenAiAPIKey(vars.APIConfig.Knowledge.Key)
	cfg = cfg.SetOpenAibaseURL(vars.APIConfig.Knowledge.Endpoint)

	config := openai.DefaultConfig(cfg.OpenAiAPIKey())
	config.BaseURL = cfg.OpenAibaseURL()

	voiceMap := map[string]openai.SpeechVoice{
		"alloy":   openai.VoiceAlloy,
		"onyx":    openai.VoiceOnyx,
		"fable":   openai.VoiceFable,
		"shimmer": openai.VoiceShimmer,
		"nova":    openai.VoiceNova,
		"echo":    openai.VoiceEcho,
		"":        openai.VoiceFable,
	}

	openaiVoice := voiceMap[vars.APIConfig.Knowledge.OpenAIVoice]

	client := openai.NewClientWithConfig(config)
	res, err := client.CreateSpeech(context.Background(), openai.CreateSpeechRequest{
		Model: openai.TTSModel1,
		Input: aiText,
		Voice: openaiVoice,
	})

	if err != nil {
		logger.Println("Error:", err)
		return
	}

	defer res.Close()

	buf, err := io.ReadAll(res)
	if err != nil {
		logger.Println("Error:", err)
		return
	}

	// 检查文件是否存在
	if _, err = os.Stat(outputFile); err == nil {
		// 文件存在，尝试删除
		err = os.Remove(outputFile)
		if err != nil {
			// 删除文件时出错
			logger.Printf("Failed to delete existing file %s: %s", outputFile, err)
			return
		}
		logger.Printf("Existing file %s deleted successfully.\n", outputFile)
	} else if !os.IsNotExist(err) {
		// 访问文件时出现了其他错误
		logger.Printf("Error checking file %s: %s", outputFile, err)
		return
	}

	// 保存 buf 到文件为 mp3
	err = os.WriteFile(outputFile, buf, 0644)
	if err != nil {
		logger.Println("Error:", err)
		return
	}
}
