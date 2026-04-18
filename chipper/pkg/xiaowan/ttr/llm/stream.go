package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/intent"
	robotpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/robot"
)

// CreateAIReq 根据当前配置、模型和会话记忆构建流式聊天请求。
func CreateAIReq(transcribedText, esn string, gpt3tryagain, isKG bool) openai.ChatCompletionRequest {
	defaultPrompt := "You are a helpful, animated robot called Vector. Keep the response concise yet informative."

	var nChat []openai.ChatCompletionMessage

	smsg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
	}
	if strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt) != "" {
		smsg.Content = strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt)
	} else {
		smsg.Content = defaultPrompt
	}

	var model string

	model = getKnowledgeModel(gpt3tryagain)
	logKnowledgeModel(model)

	smsg.Content = createPromptWithMemory(smsg.Content, model, esn, isKG)

	nChat = append(nChat, smsg)
	if vars.APIConfig.Knowledge.SaveChat {
		rchat := GetChat(esn)
		logger.Println("Using remembered chats, length of " + fmt.Sprint(len(rchat.Chats)) + " messages")
		nChat = append(nChat, rchat.Chats...)
	}
	nChat = append(nChat, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: transcribedText,
	})

	aireq := openai.ChatCompletionRequest{
		Model:            model,
		MaxTokens:        2048,
		Temperature:      1,
		TopP:             1,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
		Messages:         nChat,
		Stream:           true,
	}
	return aireq
}

// StreamingKGSim 处理 LLM 流式回复，并把文本、动作和机器人行为串成一条完整链路。
func StreamingKGSim(req interface{}, esn string, transcribedText string, isKG bool) (string, error) {
	endActivity := robotpkg.BeginForegroundActivity(esn)
	defer endActivity()

	start := make(chan bool)
	stop := make(chan bool)
	stopStop := make(chan bool)
	kgReadyToAnswer := make(chan bool)
	kgStopLooping := false
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
			return err.Error(), err
		}
	}
	_, err := robot.Conn.BatteryState(context.Background(), &vectorpb.BatteryStateRequest{})
	if err != nil {
		return "", err
	}
	if isKG {
		robotpkg.BControl(robot, ctx, start, stop)
		go func() {
			for {
				if kgStopLooping {
					kgReadyToAnswer <- true
					break
				}
				robot.Conn.PlayAnimation(ctx, &vectorpb.PlayAnimationRequest{
					Animation: &vectorpb.Animation{
						Name: "anim_knowledgegraph_searching_01",
					},
					Loops: 1,
				})
				time.Sleep(time.Second / 3)
			}
		}()
	}
	var fullRespText string
	var fullfullRespText string
	var fullRespSlice []string
	var isDone bool
	c := newKnowledgeClient()
	speakReady := make(chan string)
	successIntent := make(chan bool, 1)
	prefetch := newTTSPrefetchSession()
	intentAnnounced := false
	notifySuccess := func() {
		if intentAnnounced {
			return
		}
		intentAnnounced = true
		select {
		case successIntent <- true:
		default:
		}
	}

	aireq := CreateAIReq(transcribedText, esn, false, isKG)

	stream, err := c.CreateChatCompletionStream(ctx, aireq)
	if err != nil {
		log.Printf("Error creating chat completion stream: %v", err)
		if strings.Contains(err.Error(), "does not exist") && vars.APIConfig.Knowledge.Provider == "openai" {
			logger.Println("GPT-4 model cannot be accessed with this API key. You likely need to add more than $5 dollars of funds to your OpenAI account.")
			logger.LogUI("GPT-4 model cannot be accessed with this API key. You likely need to add more than $5 dollars of funds to your OpenAI account.")
			aireq := CreateAIReq(transcribedText, esn, true, isKG)
			logger.Println("Falling back to " + aireq.Model)
			logger.LogUI("Falling back to " + aireq.Model)
			stream, err = c.CreateChatCompletionStream(ctx, aireq)
			if err != nil {
				logger.Println("OpenAI still not returning a response even after falling back. Erroring.")
				return "", err
			}
		} else {
			if isKG {
				kgStopLooping = true
				for range kgReadyToAnswer {
					break
				}
				stop <- true
				time.Sleep(time.Second / 3)
				robotpkg.KGSim(esn, "There was an error getting data from the L. L. M.")
			}
			return "", err
		}
	}
	nChat := aireq.Messages
	nChat = append(nChat, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
	})
	fmt.Println("LLM stream response: ")
	go func() {
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// prevents a crash
				if len(fullRespSlice) == 0 && strings.TrimSpace(fullfullRespText) != "" {
					logger.Println("LLM debug: final response has no sentence punctuation, using raw content")
					finalChunk := strings.TrimSpace(fullfullRespText)
					fullRespSlice = append(fullRespSlice, finalChunk)
					prefetch.PreloadFromRaw(finalChunk)
					notifySuccess()
				}
				logger.Println("LLM final raw: " + clipDebugString(fullfullRespText, 300))
				logger.Println("LLM final slices: " + fmt.Sprint(fullRespSlice))
				if len(fullRespSlice) == 0 {
					logger.Println("LLM returned no response")
					successIntent <- false
					if isKG {
						kgStopLooping = true
						for range kgReadyToAnswer {
							break
						}
						stop <- true
						time.Sleep(time.Second / 3)
						robotpkg.KGSim(esn, "There was an error getting data from the L. L. M.")
					}
					break
				}
				isDone = true
				// if fullRespSlice != fullRespText, add that missing bit to fullRespSlice
				newStr := fullRespSlice[0]
				for i, str := range fullRespSlice {
					if i == 0 {
						continue
					}
					newStr = newStr + " " + str
				}
				if strings.TrimSpace(newStr) != strings.TrimSpace(fullfullRespText) {
					logger.Println("LLM debug: there is content after the last punctuation mark")
					extraBit := strings.TrimPrefix(fullRespText, newStr)
					fullRespSlice = append(fullRespSlice, extraBit)
				}
				if vars.APIConfig.Knowledge.SaveChat {
					Remember(openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: transcribedText,
					},
						openai.ChatCompletionMessage{
							Role:    openai.ChatMessageRoleAssistant,
							Content: newStr,
						},
						esn)
				}
				logger.LogUI("LLM response for " + esn + ": " + newStr)
				logger.Println("LLM stream finished")
				return
			}

			if err != nil {
				logger.Println("Stream error: " + err.Error())
				return
			}

			if len(response.Choices) == 0 {
				logger.Println("Empty response")
				return
			}

			deltaText := removeSpecialCharacters(response.Choices[0].Delta.Content)
			if strings.TrimSpace(deltaText) != "" {
				logger.Println("LLM delta: " + clipDebugString(deltaText, 80))
			}
			fullfullRespText = fullfullRespText + deltaText
			fullRespText = fullRespText + deltaText
			if nextSentence, remainder, ok := splitFirstSpeechChunk(fullRespText); ok {
				fullRespSlice = append(fullRespSlice, nextSentence)
				fullRespText = remainder
				prefetch.PreloadFromRaw(nextSentence)
				notifySuccess()
				select {
				case speakReady <- nextSentence:
				default:
				}
			}
		}
	}()
	for is := range successIntent {
		if is {
			if !isKG {
				intent.IntentPass(req, "intent_greeting_hello", transcribedText, map[string]string{}, false)
			}
			break
		} else {
			return "", errors.New("llm returned no response")
		}
	}
	time.Sleep(time.Millisecond * 200)
	if !isKG {
		robotpkg.BControl(robot, ctx, start, stop)
	}
	interrupted := false
	go func() {
		interrupted = robotpkg.InterruptKGSimWhenTouchedOrWaked(robot, stop, stopStop)
	}()
	for range start {
		if isKG {
			kgStopLooping = true
			for range kgReadyToAnswer {
				break
			}
		} else {
			time.Sleep(time.Millisecond * 300)
		}
		speechSession := newSpeechAnimationSession(robot, ctx, newSpeechAnimationConfig(isKG), !vars.APIConfig.Knowledge.CommandsEnable)
		speechSession.Start()
		var disconnect bool
		numInResp := 0
		for {
			respSlice := fullRespSlice
			if len(respSlice)-1 < numInResp {
				if !isDone {
					logger.Println("Waiting for more content from LLM...")
					for range speakReady {
						respSlice = fullRespSlice
						break
					}
				} else {
					break
				}
			}
			if interrupted {
				break
			}
			logger.Println(respSlice[numInResp])
			acts := GetActionsFromString(respSlice[numInResp])
			nChat[len(nChat)-1].Content = fullRespText
			disconnect = PerformActions(nChat, acts, robot, stopStop, prefetch)
			if disconnect {
				break
			}
			numInResp = numInResp + 1
		}
		speechSession.Stop(!interrupted)
		time.Sleep(time.Millisecond * 100)
		// if isKG {
		// 	robot.Conn.PlayAnimation(
		// 		ctx,
		// 		&vectorpb.PlayAnimationRequest{
		// 			Animation: &vectorpb.Animation{
		// 				Name: "anim_knowledgegraph_success_01",
		// 			},
		// 			Loops: 1,
		// 		},
		// 	)
		// 	time.Sleep(time.Millisecond * 3300)
		// }
		if !interrupted {
			stopStop <- true
			stop <- true
		}
	}
	return "", nil
}

// KGSim 是对机器人简短播报能力的兼容封装。
func KGSim(esn string, textToSay string) error {
	return robotpkg.KGSim(esn, textToSay)
}
