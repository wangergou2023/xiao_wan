package processreqs

import (
	pb "github.com/digital-dream-labs/api/go/chipperpb"
	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vtt"
	sr "github.com/wangergou2023/wire-pod/chipper/pkg/wirepod/speechrequest"
	vector "github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/vector"
)

var NoResult string = "NoResultCommand"

func streamingKG(req *vtt.KnowledgeGraphRequest, speechReq sr.SpeechRequest) {
	transcribedText, err := sttHandler(speechReq)
	if err != nil {
		logger.Println("STT error: " + err.Error())
		return
	}
	kg := pb.KnowledgeGraphResponse{
		Session:     req.Session,
		DeviceId:    req.Device,
		CommandType: NoResult,
		SpokenText:  "processing",
	}
	_ = req.Stream.Send(&kg)
	_, err = vector.StreamingKGSim(req, req.Device, transcribedText, true)
	if err != nil {
		logger.Println("LLM error: " + err.Error())
	}
	logger.Println("(KG) Bot " + speechReq.Device + " request served.")
}

func (s *Server) ProcessKnowledgeGraph(req *vtt.KnowledgeGraphRequest) (*vtt.KnowledgeGraphResponse, error) {
	speechReq := sr.ReqToSpeechRequest(req)
	streamingKG(req, speechReq)
	return nil, nil

}
