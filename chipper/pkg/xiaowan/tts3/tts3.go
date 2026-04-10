package tts3

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

type TTSConfig struct {
	APIKey   string
	Model    string
	Voice    string
	Language string
}

// 流式TTS请求结构
type StreamTTSRequest struct {
	Model string `json:"model"`
	Input struct {
		Text         string `json:"text"`
		Voice        string `json:"voice"`
		LanguageType string `json:"language_type"`
	} `json:"input"`
}

// 流式响应事件结构
type StreamTTSEvent struct {
	Output struct {
		Audio struct {
			Data      string `json:"data"`       // base64编码的音频数据
			ExpiresAt int64  `json:"expires_at"` // 最后一个包包含过期时间
			ID        string `json:"id"`         // 最后一个包包含ID
			URL       string `json:"url"`        // 最后一个包包含完整URL
		} `json:"audio"`
		FinishReason string `json:"finish_reason"` // 完成原因
	} `json:"output"`
	Usage struct {
		Characters int `json:"characters"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

// 流式TTS配置
var streamTTSConfig = TTSConfig{
	APIKey:   vars.APIConfig.Knowledge.Key,
	Model:    "qwen3-tts-flash",
	Voice:    "Sunny",
	Language: "Chinese",
}

// StreamAliyunTTS 流式TTS主函数
func StreamAliyunTTS(outputFile string, aiText string) error {
	if aiText == "" {
		return fmt.Errorf("文本内容不能为空")
	}
	if outputFile == "" {
		outputFile = "tts_stream_output.wav"
	}

	fmt.Printf("开始流式语音合成...\n")
	fmt.Printf("文本: %s\n", aiText)
	fmt.Printf("输出文件: %s\n", outputFile)

	// 创建输出文件
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %v", err)
	}
	defer file.Close()

	// 调用流式TTS API
	audioURL, totalChars, requestID, err := streamTextToSpeech(aiText, file)
	if err != nil {
		return fmt.Errorf("流式TTS转换失败: %v", err)
	}

	// 输出统计信息
	fmt.Printf("✅ 流式语音合成成功!\n")
	fmt.Printf("📁 输出文件: %s\n", outputFile)
	fmt.Printf("📝 文本长度: %d 字符\n", totalChars)
	fmt.Printf("🆔 请求ID: %s\n", requestID)
	if audioURL != "" {
		fmt.Printf("🔗 完整音频URL: %s\n", audioURL)
	}

	return nil
}

// streamTextToSpeech 流式TTS处理
func streamTextToSpeech(text string, outputFile *os.File) (string, int, string, error) {
	// 创建请求数据
	reqData := StreamTTSRequest{
		Model: streamTTSConfig.Model,
	}
	reqData.Input.Text = text
	reqData.Input.Voice = streamTTSConfig.Voice
	reqData.Input.LanguageType = streamTTSConfig.Language

	// 序列化请求数据
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", 0, "", fmt.Errorf("序列化请求数据失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置流式请求头
	req.Header.Set("Authorization", "Bearer "+streamTTSConfig.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-SSE", "enable") // 启用服务器发送事件

	// 发送请求
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, "", fmt.Errorf("API请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 处理流式响应
	scanner := bufio.NewScanner(resp.Body)
	var audioURL string
	var totalChars int
	var requestID string
	chunkCount := 0

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行和事件类型行
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}

		// 解析数据行
		if strings.HasPrefix(line, "data:") {
			jsonData := strings.TrimPrefix(line, "data:")
			var event StreamTTSEvent

			if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
				fmt.Printf("解析事件数据失败: %v\n", err)
				continue
			}

			// 处理音频数据
			if event.Output.Audio.Data != "" {
				chunkCount++
				audioData, err := base64.StdEncoding.DecodeString(event.Output.Audio.Data)
				if err != nil {
					fmt.Printf("Base64解码失败: %v\n", err)
					continue
				}

				// 写入音频数据到文件
				if _, err := outputFile.Write(audioData); err != nil {
					fmt.Printf("写入音频数据失败: %v\n", err)
					continue
				}

				fmt.Printf("收到音频数据块 #%d, 大小: %d 字节\n", chunkCount, len(audioData))
			}

			// 记录使用量信息
			if event.Usage.Characters > 0 {
				totalChars = event.Usage.Characters
			}

			// 记录请求ID
			if event.RequestID != "" {
				requestID = event.RequestID
			}

			// 检查是否是最后一个数据包（包含完整URL）
			if event.Output.Audio.URL != "" {
				audioURL = event.Output.Audio.URL
				fmt.Printf("收到完整音频URL: %s\n", audioURL)
				fmt.Printf("合成完成，原因: %s\n", event.Output.FinishReason)
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", 0, "", fmt.Errorf("读取流式响应失败: %v", err)
	}

	fmt.Printf("流式处理完成，共收到 %d 个音频数据块\n", chunkCount)
	return audioURL, totalChars, requestID, nil
}

// 批量流式TTS处理
func BatchStreamTTS(texts []string, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	for i, text := range texts {
		if text == "" {
			continue
		}

		outputFile := fmt.Sprintf("%s/tts_%d.wav", outputDir, i+1)
		fmt.Printf("\n处理第 %d/%d 个文本...\n", i+1, len(texts))

		if err := StreamAliyunTTS(outputFile, text); err != nil {
			fmt.Printf("第 %d 个文本处理失败: %v\n", i+1, err)
			continue
		}
	}

	fmt.Printf("\n✅ 批量处理完成! 输出目录: %s\n", outputDir)
	return nil
}
