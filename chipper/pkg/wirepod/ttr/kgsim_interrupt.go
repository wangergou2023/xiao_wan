package wirepod_ttr

import (
	"context"
	"time"

	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vector"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vectorpb"
)

func InterruptKGSimWhenTouchedOrWaked(rob *vector.Vector, stop chan bool, stopStop chan bool) bool {
	strm, err := rob.Conn.EventStream(
		context.Background(),
		&vectorpb.EventRequest{
			ListType: &vectorpb.EventRequest_WhiteList{
				WhiteList: &vectorpb.FilterList{
					List: []string{"robot_state", "wake_word"},
				},
			},
		},
	)
	if err != nil {
		logger.Println("Couldn't make an event stream: " + err.Error())
		return false
	}
	var stopFunc bool
	go func() {
		for range stopStop {
			logger.Println("KG Interrupter has been stopped")
			stopFunc = true
			break
		}
	}()
	var origTouchValue uint32
	var origValueGotten bool
	var valsAboveValue int
	var valsAboveValueMax int = 5
	var stopResponse bool
	for {
		var resp *vectorpb.EventResponse
		resp, err = strm.Recv()
		if err != nil {
			break
		}
		switch resp.Event.EventType.(type) {
		case *vectorpb.Event_RobotState:
			origTouchValue = resp.Event.GetRobotState().TouchData.GetRawTouchValue()
			origValueGotten = true
		default:
		}
		if origValueGotten {
			break
		}
	}
	if origValueGotten {
		for {
			var resp *vectorpb.EventResponse
			resp, err = strm.Recv()
			if err != nil {
				logger.Println("Event stream error: " + err.Error())
				return false
			}
			switch resp.Event.EventType.(type) {
			case *vectorpb.Event_RobotState:
				if resp.Event.GetRobotState().TouchData.GetRawTouchValue() > origTouchValue+50 {
					valsAboveValue++
				} else {
					valsAboveValue = 0
				}
			case *vectorpb.Event_WakeWord:
				logger.Println("Interrupting LLM response (source: wake word)")
				stopResponse = true
			default:
			}
			if valsAboveValue > valsAboveValueMax {
				logger.Println("Interrupting LLM response (source: touch sensor)")
				stopResponse = true
			}
			if stopResponse {
				stop <- true
				time.Sleep(time.Second / 4)
				return true
			}
			if stopFunc {
				strm.CloseSend()
				return false
			}
		}
	}
	return false
}
