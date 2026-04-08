package processreqs

import (
	"strings"

	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vtt"
	sr "github.com/wangergou2023/wire-pod/chipper/pkg/wirepod/speechrequest"
	ttr "github.com/wangergou2023/wire-pod/chipper/pkg/wirepod/ttr"
	vector "github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/vector"
)

// This is here for compatibility with 1.6 and older software
func (s *Server) ProcessIntent(req *vtt.IntentRequest) (*vtt.IntentResponse, error) {
	speechReq := sr.ReqToSpeechRequest(req)
	var transcribedText string
	if !isSti {
		var err error
		transcribedText, err = sttHandler(speechReq)
		if err != nil {
			ttr.IntentPass(req, "intent_system_noaudio", "voice processing error: "+err.Error(), map[string]string{"error": err.Error()}, true)
			return nil, nil
		}
		if strings.TrimSpace(transcribedText) == "" {
			ttr.IntentPass(req, "intent_system_noaudio", "", map[string]string{}, false)
			return nil, nil
		}
	} else {
		intent, slots, err := stiHandler(speechReq)
		if err != nil {
			if err.Error() == "inference not understood" {
				logger.Println("No intent was matched")
				ttr.IntentPass(req, "intent_system_unmatched", "voice processing error", map[string]string{"error": err.Error()}, true)
				return nil, nil
			}
			logger.Println(err)
			ttr.IntentPass(req, "intent_system_noaudio", "voice processing error", map[string]string{"error": err.Error()}, true)
			return nil, nil
		}
		_ = intent
		_ = slots
		logger.Println("STI handler is not supported in whisper-only mode.")
		return nil, nil
	}
	logger.Println("Making LLM request for device " + req.Device + "...")
	_, err := vector.StreamingKGSim(req, req.Device, transcribedText, false)
	if err != nil {
		logger.Println("LLM error: " + err.Error())
		logger.LogUI("LLM error: " + err.Error())
		ttr.IntentPass(req, "intent_system_unmatched", transcribedText, map[string]string{"": ""}, false)
	}
	logger.Println("Bot " + speechReq.Device + " request served.")
	return nil, nil
}
