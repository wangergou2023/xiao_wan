package server

import (
	"strings"
	"time"

	pb "github.com/digital-dream-labs/api/go/chipperpb"
	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vtt"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/stt"
	vector "github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/vector"
)

// StreamingIntentGraph handles intent graph request streams
func (s *Server) StreamingIntentGraph(stream pb.ChipperGrpc_StreamingIntentGraphServer) error {
	recvTime := time.Now()

	req, err := stream.Recv()
	if err != nil {
		logger.Println("Intent graph stream error")
		logger.Println(err)

		return err
	}

	igraphReq := &vtt.IntentGraphRequest{
		Time:       recvTime,
		Stream:     stream,
		Device:     req.DeviceId,
		Session:    req.Session,
		LangString: req.LanguageCode.String(),
		FirstReq:   req,
		AudioCodec: req.AudioEncoding,
	}

	if err = processIntentGraph(igraphReq); err != nil {
		logger.Println("Intent graph processing error")
		logger.Println(err)
		return err
	}

	return nil
}

func processIntentGraph(req *vtt.IntentGraphRequest) error {
	speechReq := stt.NewSpeechRequest(req)
	transcribedText, err := sttHandler(speechReq)
	if err != nil {
		logger.Println("STT error: " + err.Error())
		return nil
	}
	if strings.TrimSpace(transcribedText) == "" {
		logger.Println("STT returned empty text")
		return nil
	}
	logger.Println("Making LLM request for device " + req.Device + "...")
	_, err = vector.StreamingKGSim(req, req.Device, transcribedText, false)
	if err != nil {
		logger.Println("LLM error: " + err.Error())
		logger.LogUI("LLM error: " + err.Error())
	}
	logger.Println("Bot " + speechReq.Device + " request served.")
	return nil
}
