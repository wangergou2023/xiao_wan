package robot

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
)

// KGSim 用于在不经过完整 LLM 流程时，让机器人直接播报一段短文本。
func KGSim(esn string, textToSay string) error {
	endActivity := BeginForegroundActivity(esn, "kgsim_voice")
	ctx := context.Background()
	matched := false
	var robot *vector.Vector
	var guid string
	var target string
	for _, bot := range vars.BotInfo.Robots {
		if esn == bot.Esn {
			guid = bot.GUID
			target = bot.IPAddress + ":443"
			matched = true
			break
		}
	}
	if matched {
		var err error
		robot, err = vector.New(vector.WithSerialNo(esn), vector.WithToken(guid), vector.WithTarget(target))
		if err != nil {
			endActivity()
			return err
		}
	}
	controlRequest := &vectorpb.BehaviorControlRequest{
		RequestType: &vectorpb.BehaviorControlRequest_ControlRequest{
			ControlRequest: &vectorpb.ControlRequest{
				Priority: vectorpb.ControlRequest_OVERRIDE_BEHAVIORS,
			},
		},
	}
	go func() {
		defer endActivity()

		start := make(chan bool)
		stop := make(chan bool)

		go func() {
			r, err := robot.Conn.BehaviorControl(ctx)
			if err != nil {
				log.Println(err)
				return
			}

			if err := r.Send(controlRequest); err != nil {
				log.Println(err)
				return
			}

			for {
				ctrlresp, err := r.Recv()
				if err != nil {
					log.Println(err)
					return
				}
				if ctrlresp.GetControlGrantedResponse() != nil {
					start <- true
					break
				}
			}

			for {
				select {
				case <-stop:
					logger.Println("KGSim: releasing behavior control (interrupt)")
					if err := r.Send(&vectorpb.BehaviorControlRequest{
						RequestType: &vectorpb.BehaviorControlRequest_ControlRelease{
							ControlRelease: &vectorpb.ControlRelease{},
						},
					}); err != nil {
						log.Println(err)
						return
					}
					return
				default:
					continue
				}
			}
		}()

		var stopTTSLoop bool
		var TTSLoopStopped bool
		for range start {
			time.Sleep(time.Millisecond * 300)
			robot.Conn.PlayAnimation(ctx, &vectorpb.PlayAnimationRequest{
				Animation: &vectorpb.Animation{Name: "anim_getin_tts_01"},
				Loops:     1,
			})
			go func() {
				for {
					if stopTTSLoop {
						TTSLoopStopped = true
						break
					}
					robot.Conn.PlayAnimation(ctx, &vectorpb.PlayAnimationRequest{
						Animation: &vectorpb.Animation{Name: "anim_tts_loop_02"},
						Loops:     1,
					})
				}
			}()
			textToSaySplit := strings.Split(textToSay, ". ")
			for _, str := range textToSaySplit {
				_, err := robot.Conn.SayText(ctx, &vectorpb.SayTextRequest{
					Text:           str,
					UseVectorVoice: true,
					DurationScalar: 0.95,
				})
				if err != nil {
					log.Println(err)
				}
			}
			stopTTSLoop = true
			for !TTSLoopStopped {
				time.Sleep(time.Millisecond * 25)
			}
			robot.Conn.PlayAnimation(ctx, &vectorpb.PlayAnimationRequest{
				Animation: &vectorpb.Animation{Name: "anim_getout_tts_01"},
				Loops:     1,
			})
			stop <- true
		}
	}()
	return nil
}
