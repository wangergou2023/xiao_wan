package server

import (
	"fmt"

	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/stt"
)

// speech-to-text
var sttHandler func(stt.SpeechRequest) (string, error)

// InitVoiceProcessor initializes the STT pipeline and stores the handler for intent-graph processing.
func InitVoiceProcessor(initFunc func() error, handler interface{}, voiceProcessor string) error {
	// Decide the TTS language
	if voiceProcessor != "vosk" && voiceProcessor != "whisper.cpp" {
		vars.APIConfig.STT.Language = "en-US"
	}
	sttLanguage := vars.APIConfig.STT.Language
	logger.Println("Initiating " + voiceProcessor + " voice processor with language " + sttLanguage)
	vars.SttInitFunc = initFunc
	if err := initFunc(); err != nil {
		return err
	}

	// Handler must be `func(stt.SpeechRequest) (string, error)`
	if str, is := handler.(func(stt.SpeechRequest) (string, error)); is {
		sttHandler = str
	} else {
		return fmt.Errorf("stthandler not of correct type")
	}

	return nil
}
