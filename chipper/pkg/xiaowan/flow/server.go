package flow

import (
	"fmt"

	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/stt"
)

// Server stores the config
type Server struct{}

var VoiceProcessor = ""

// speech-to-text
var sttHandler func(stt.SpeechRequest) (string, error)

// New returns a new server
func New(InitFunc func() error, SttHandler interface{}, voiceProcessor string) (*Server, error) {
	// Decide the TTS language
	if voiceProcessor != "vosk" && voiceProcessor != "whisper.cpp" {
		vars.APIConfig.STT.Language = "en-US"
	}
	sttLanguage := vars.APIConfig.STT.Language
	logger.Println("Initiating " + voiceProcessor + " voice processor with language " + sttLanguage)
	vars.SttInitFunc = InitFunc
	err := InitFunc()
	if err != nil {
		return nil, err
	}

	// SttHandler must be `func(stt.SpeechRequest) (string, error)`
	if str, is := SttHandler.(func(stt.SpeechRequest) (string, error)); is {
		sttHandler = str
	} else {
		return nil, fmt.Errorf("stthandler not of correct type")
	}

	// Initiating the chosen voice processor
	VoiceProcessor = voiceProcessor

	return &Server{}, err
}
