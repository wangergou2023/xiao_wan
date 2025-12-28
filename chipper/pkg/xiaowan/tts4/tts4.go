package tts4

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// =======================
// 配置
// =======================

type LocalTTSConfig struct {
	BaseURL string
	Voice   string
}

// 全局配置（可改成 vars）
var localTTSConfig = LocalTTSConfig{
	BaseURL: "http://192.168.1.102:9966",
	Voice:   "3333",
}

// =======================
// 返回结构
// =======================

type LocalTTSResponse struct {
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	URL        string `json:"url"`
	Filename   string `json:"filename"`
	Relative   string `json:"relative_url"`
	AudioFiles []struct {
		AudioDuration float64 `json:"audio_duration"`
		InferenceTime float64 `json:"inference_time"`
		Filename      string  `json:"filename"`
		URL           string  `json:"url"`
		RelativeURL   string  `json:"relative_url"`
	} `json:"audio_files"`
}

// =======================
// 主函数：文本转语音
// =======================

func LocalTTS(outputFile string, text string) error {
	if text == "" {
		return fmt.Errorf("文本不能为空")
	}
	if outputFile == "" {
		outputFile = "local_tts.wav"
	}

	ttsResp, err := callLocalTTS(text)
	if err != nil {
		return err
	}

	audioURL := ttsResp.URL
	if audioURL == "" && len(ttsResp.AudioFiles) > 0 {
		audioURL = ttsResp.AudioFiles[0].URL
	}

	if audioURL == "" {
		return fmt.Errorf("未返回音频URL")
	}

	fmt.Println("🎧 音频URL:", audioURL)

	err = downloadAudio(audioURL, outputFile)
	if err != nil {
		return err
	}

	fmt.Println("✅ 本地 TTS 成功")
	fmt.Println("📁 输出文件:", outputFile)

	return nil
}

// =======================
// 调用 /tts 接口
// =======================

func callLocalTTS(text string) (*LocalTTSResponse, error) {
	form := url.Values{}
	form.Set("text", text)
	form.Set("prompt", "")
	form.Set("voice", localTTSConfig.Voice)
	form.Set("temperature", "0.3")
	form.Set("top_p", "0.7")
	form.Set("top_k", "20")
	form.Set("skip_refine", "0")
	form.Set("custom_voice", "0")

	req, err := http.NewRequest(
		"POST",
		localTTSConfig.BaseURL+"/tts",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var ttsResp LocalTTSResponse
	err = json.Unmarshal(body, &ttsResp)
	if err != nil {
		return nil, err
	}

	if ttsResp.Code != 0 {
		return nil, fmt.Errorf("TTS失败: %s", ttsResp.Msg)
	}

	return &ttsResp, nil
}

// =======================
// 下载音频
// =======================

func downloadAudio(url, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %d", resp.StatusCode)
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}
