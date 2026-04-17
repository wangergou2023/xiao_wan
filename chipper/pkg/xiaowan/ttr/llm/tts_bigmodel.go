package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
)

const (
	bigModelTTSEndpoint     = "https://open.bigmodel.cn/api/paas/v4/audio/speech"
	defaultBigModelTTSModel = "glm-tts"
	defaultBigModelTTSVoice = "tongtong"
)

type bigModelTTSRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
	Volume         float64 `json:"volume,omitempty"`
	ResponseFormat string  `json:"response_format"`
}

type bigModelTTSError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
	Message string `json:"message"`
}

func bigModelTTSEnabled() bool {
	return strings.TrimSpace(getBigModelToken()) != ""
}

func getBigModelToken() string {
	if strings.TrimSpace(vars.APIConfig.BigModel.Key) != "" {
		return strings.TrimSpace(vars.APIConfig.BigModel.Key)
	}
	return strings.TrimSpace(os.Getenv("BIGMODEL_API_TOKEN"))
}

func getBigModelTTSModel() string {
	if model := normalizeBigModelTTSModel(strings.TrimSpace(vars.APIConfig.BigModel.TTSModel)); model != "" {
		return model
	}
	if model := strings.TrimSpace(vars.APIConfig.BigModel.TTSModel); model != "" {
		return normalizeBigModelTTSModel(model)
	}
	if model := strings.TrimSpace(os.Getenv("BIGMODEL_TTS_MODEL")); model != "" {
		return normalizeBigModelTTSModel(model)
	}
	return defaultBigModelTTSModel
}

func normalizeBigModelTTSModel(model string) string {
	if model == "" {
		return ""
	}
	lowerModel := strings.ToLower(strings.TrimSpace(model))
	// 防止把聊天模型误填到 TTS 模型里，导致语音接口按聊天协议报 messages 参数非法。
	if strings.Contains(lowerModel, "glm-5") || strings.Contains(lowerModel, "glm-4.5") || strings.Contains(lowerModel, "chat") {
		logger.Println("BigModel TTS model looks like a chat model, falling back to glm-tts: " + model)
		return defaultBigModelTTSModel
	}
	return model
}

func getBigModelTTSVoice() string {
	if voice := strings.TrimSpace(vars.APIConfig.BigModel.TTSVoice); voice != "" {
		return voice
	}
	if voice := strings.TrimSpace(os.Getenv("BIGMODEL_TTS_VOICE")); voice != "" {
		return voice
	}
	return defaultBigModelTTSVoice
}

func getBigModelTTSFloat(name string, fallback float64) float64 {
	switch name {
	case "BIGMODEL_TTS_SPEED":
		if vars.APIConfig.BigModel.TTSSpeed > 0 {
			return vars.APIConfig.BigModel.TTSSpeed
		}
	case "BIGMODEL_TTS_VOLUME":
		if vars.APIConfig.BigModel.TTSVolume > 0 {
			return vars.APIConfig.BigModel.TTSVolume
		}
	}
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func requestBigModelSpeech(input string) ([]byte, error) {
	payload := bigModelTTSRequest{
		Model:          getBigModelTTSModel(),
		Input:          input,
		Voice:          getBigModelTTSVoice(),
		Speed:          getBigModelTTSFloat("BIGMODEL_TTS_SPEED", 1.0),
		Volume:         getBigModelTTSFloat("BIGMODEL_TTS_VOLUME", 1.0),
		ResponseFormat: "pcm",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, bigModelTTSEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+getBigModelToken())
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr bigModelTTSError
		if err := json.Unmarshal(respBody, &apiErr); err == nil {
			if apiErr.Error.Message != "" {
				return nil, fmt.Errorf("bigmodel tts failed: %s", apiErr.Error.Message)
			}
			if apiErr.Message != "" {
				return nil, fmt.Errorf("bigmodel tts failed: %s", apiErr.Message)
			}
		}
		return nil, fmt.Errorf("bigmodel tts failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if len(respBody) == 0 {
		return nil, fmt.Errorf("bigmodel tts returned empty audio")
	}
	return respBody, nil
}

// DoSayText_BigModel 使用智谱 TTS 合成 24k PCM，然后推给机器人播放。
func DoSayText_BigModel(robot *vector.Vector, input string) error {
	speechBytes, err := requestBigModelSpeech(input)
	if err != nil {
		return err
	}
	return playPCM24kOnRobot(robot, speechBytes)
}
