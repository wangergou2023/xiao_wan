package flow

import (
	"strings"

	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vtt"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/stt"
	vector "github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/vector"
)

func (s *Server) ProcessIntentGraph(req *vtt.IntentGraphRequest) (*vtt.IntentGraphResponse, error) {
	speechReq := stt.NewSpeechRequest(req)
	transcribedText, err := sttHandler(speechReq)
	if err != nil {
		logger.Println("STT error: " + err.Error())
		return nil, nil
	}
	if strings.TrimSpace(transcribedText) == "" {
		logger.Println("STT returned empty text")
		return nil, nil
	}
	logger.Println("Making LLM request for device " + req.Device + "...")
	_, err = vector.StreamingKGSim(req, req.Device, transcribedText, false)
	if err != nil {
		logger.Println("LLM error: " + err.Error())
		logger.LogUI("LLM error: " + err.Error())
	}
	logger.Println("Bot " + speechReq.Device + " request served.")
	return nil, nil
}
