package intent

import (
	"fmt"

	pb "github.com/digital-dream-labs/api/go/chipperpb"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vtt"
)

// IntentPass 将匹配到的 intent 结果写回 IntentGraph 流。
func IntentPass(req interface{}, intentThing string, speechText string, intentParams map[string]string, isParam bool) (interface{}, error) {
	var esn string
	var req1 *vtt.IntentGraphRequest
	if str, ok := req.(*vtt.IntentGraphRequest); ok {
		req1 = str
		esn = req1.Device
	}

	var intentResult pb.IntentResult
	if isParam {
		intentResult = pb.IntentResult{
			QueryText:  speechText,
			Action:     intentThing,
			Parameters: intentParams,
		}
	} else {
		intentResult = pb.IntentResult{
			QueryText: speechText,
			Action:    intentThing,
		}
	}
	logger.LogUI("Intent matched: " + intentThing + ", transcribed text: '" + speechText + "', device: " + esn)
	if isParam {
		logger.LogUI("Parameters sent: " + fmt.Sprint(intentParams))
	}
	intentGraphSend := pb.IntentGraphResponse{
		ResponseType: pb.IntentGraphMode_INTENT,
		IsFinal:      true,
		IntentResult: &intentResult,
		CommandType:  pb.RobotMode_VOICE_COMMAND.String(),
	}
	if err := req1.Stream.Send(&intentGraphSend); err != nil {
		return nil, err
	}
	r := &vtt.IntentGraphResponse{
		Intent: &intentGraphSend,
	}
	logger.Println("Bot " + esn + " Intent Sent: " + intentThing)
	if isParam {
		logger.Println("Bot "+esn+" Parameters Sent:", intentParams)
	} else {
		logger.Println("No Parameters Sent")
	}
	return r, nil
}

// KnowledgeGraphResponseIG 将知识回答包装成 IntentGraph 的 KG 响应。
func KnowledgeGraphResponseIG(req *vtt.IntentGraphRequest, spokenText string, queryText string) error {
	intentResult := pb.IntentResult{
		QueryText: queryText,
		Action:    "intent_knowledge_response_extend_bypass",
	}

	intentGraphSend := pb.IntentGraphResponse{
		ResponseType: pb.IntentGraphMode_KNOWLEDGE_GRAPH,
		IsFinal:      true,
		IntentResult: &intentResult,
		SpokenText:   spokenText,
		QueryText:    queryText,
		CommandType:  pb.RobotMode_VOICE_COMMAND.String(),
	}

	if err := req.Stream.Send(&intentGraphSend); err != nil {
		return err
	}
	return nil
}
