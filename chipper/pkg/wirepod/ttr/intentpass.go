package wirepod_ttr

import (
	"fmt"

	pb "github.com/digital-dream-labs/api/go/chipperpb"
	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vtt"
)

func IntentPass(req interface{}, intentThing string, speechText string, intentParams map[string]string, isParam bool) (interface{}, error) {
	var esn string
	var req1 *vtt.IntentRequest
	var req2 *vtt.IntentGraphRequest
	var isIntentGraph bool
	if str, ok := req.(*vtt.IntentRequest); ok {
		req1 = str
		esn = req1.Device
		isIntentGraph = false
	} else if str, ok := req.(*vtt.IntentGraphRequest); ok {
		req2 = str
		esn = req2.Device
		isIntentGraph = true
	}

	// intercept if not intent graph but intent graph is enabled
	if !isIntentGraph && vars.APIConfig.Knowledge.IntentGraph && intentThing == "intent_system_unmatched" {
		intentThing = "intent_greeting_hello"
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
	intent := pb.IntentResponse{
		IsFinal:      true,
		IntentResult: &intentResult,
	}
	intentGraphSend := pb.IntentGraphResponse{
		ResponseType: pb.IntentGraphMode_INTENT,
		IsFinal:      true,
		IntentResult: &intentResult,
		CommandType:  pb.RobotMode_VOICE_COMMAND.String(),
	}
	if !isIntentGraph {
		if err := req1.Stream.Send(&intent); err != nil {
			return nil, err
		}
		r := &vtt.IntentResponse{
			Intent: &intent,
		}
		logger.Println("Bot " + esn + " Intent Sent: " + intentThing)
		if isParam {
			logger.Println("Bot "+esn+" Parameters Sent:", intentParams)
		} else {
			logger.Println("No Parameters Sent")
		}
		return r, nil
	}
	if err := req2.Stream.Send(&intentGraphSend); err != nil {
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
