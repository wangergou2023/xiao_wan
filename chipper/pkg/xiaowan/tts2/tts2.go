package tts2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
)

type TTSConfig struct {
	APIKey   string
	Model    string
	Voice    string
	Language string
}

type TTSRequest struct {
	Model string `json:"model"`
	Input struct {
		Text         string `json:"text"`
		Voice        string `json:"voice"`
		LanguageType string `json:"language_type"`
	} `json:"input"`
}

type TTSResponse struct {
	Output struct {
		Audio struct {
			Data      string `json:"data"`
			ExpiresAt int64  `json:"expires_at"`
			ID        string `json:"id"`
			URL       string `json:"url"`
		} `json:"audio"`
		FinishReason string `json:"finish_reason"`
	} `json:"output"`
	Usage struct {
		Characters int `json:"characters"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

// 全局配置
var ttsConfig = TTSConfig{
	APIKey:   vars.APIConfig.Knowledge.Key,
	Model:    "qwen-tts",
	Voice:    "Cherry",
	Language: "Chinese",
}

// Aliyuntts 主函数：将文本转换为语音并保存到指定文件
func Aliyuntts(outputFile string, aiText string) error {
	// 检查输入参数
	if aiText == "" {
		return fmt.Errorf("文本内容不能为空")
	}
	if outputFile == "" {
		outputFile = "tts_output.wav" // 默认文件名
	}

	// 调用TTS API生成语音
	result, err := textToSpeech(aiText)
	if err != nil {
		return fmt.Errorf("TTS转换失败: %v", err)
	}

	// 下载音频文件
	fmt.Printf("正在下载音频到: %s\n", outputFile)
	err = downloadAudio(result.Output.Audio.URL, outputFile)
	if err != nil {
		return fmt.Errorf("下载音频失败: %v", err)
	}

	// 输出成功信息
	fmt.Printf("✅ 语音合成成功!\n")
	fmt.Printf("📁 输出文件: %s\n", outputFile)
	fmt.Printf("📝 文本长度: %d 字符\n", result.Usage.Characters)
	fmt.Printf("🆔 请求ID: %s\n", result.RequestID)
	fmt.Printf("🔗 音频URL: %s\n", result.Output.Audio.URL)
	fmt.Printf("⏰ URL过期时间: %s\n", time.Unix(result.Output.Audio.ExpiresAt, 0).Format("2006-01-02 15:04:05"))

	return nil
}

// textToSpeech 调用阿里云TTS API
func textToSpeech(text string) (*TTSResponse, error) {
	// 创建请求数据
	reqData := TTSRequest{
		Model: ttsConfig.Model,
	}
	reqData.Input.Text = text
	reqData.Input.Voice = ttsConfig.Voice
	reqData.Input.LanguageType = ttsConfig.Language

	// 序列化请求数据
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求数据失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+ttsConfig.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var ttsResp TTSResponse
	err = json.Unmarshal(body, &ttsResp)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return &ttsResp, nil
}

// downloadAudio 下载音频文件
func downloadAudio(url, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载音频失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 创建文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	// 复制数据到文件
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("保存音频文件失败: %v", err)
	}

	return nil
}
