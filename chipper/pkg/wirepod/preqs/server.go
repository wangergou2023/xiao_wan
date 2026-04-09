package processreqs

import (
	"fmt"

	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	sr "github.com/wangergou2023/wire-pod/chipper/pkg/wirepod/speechrequest"
)

// Server stores the config
type Server struct{}

var VoiceProcessor = ""

type JsonIntent struct {
	Name              string   `json:"name"`
	Keyphrases        []string `json:"keyphrases"`
	RequireExactMatch bool     `json:"requiresexact"`
}

var sttLanguage string = "en-US"

// speech-to-text
var sttHandler func(sr.SpeechRequest) (string, error)

// New returns a new server
func New(InitFunc func() error, SttHandler interface{}, voiceProcessor string) (*Server, error) {

	// Decide the TTS language
	if voiceProcessor != "vosk" && voiceProcessor != "whisper.cpp" {
		vars.APIConfig.STT.Language = "en-US"
	}
	sttLanguage = vars.APIConfig.STT.Language
	logger.Println("Initiating " + voiceProcessor + " voice processor with language " + sttLanguage)
	vars.SttInitFunc = InitFunc
	err := InitFunc()
	if err != nil {
		return nil, err
	}

	// SttHandler must be `func(sr.SpeechRequest) (string, error)`
	if str, is := SttHandler.(func(sr.SpeechRequest) (string, error)); is {
		sttHandler = str
	} else {
		return nil, fmt.Errorf("stthandler not of correct type")
	}

	// Initiating the chosen voice processor and load intents from json
	VoiceProcessor = voiceProcessor

	return &Server{}, err
}
