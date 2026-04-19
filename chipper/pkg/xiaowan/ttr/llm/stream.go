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
	forcedToolName := detectForcedNativeTool(transcribedText)
	if forcedToolPrompt := strings.TrimSpace(buildForcedToolSystemPrompt(forcedToolName)); forcedToolPrompt != "" {
		nChat = append(nChat, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: forcedToolPrompt,
		})
	}
	if todoPrompt := strings.TrimSpace(buildAutoTodoSystemPrompt(transcribedText)); todoPrompt != "" {
		nChat = append(nChat, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: todoPrompt,
		})
	}
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
	if vars.APIConfig.Knowledge.CommandsEnable {
		aireq = withNativeTools(aireq)
		if forcedToolName != "" {
			aireq = forceNativeToolChoice(aireq, forcedToolName)
		}
	}
	return aireq
}

func createKnowledgeStream(ctx context.Context, c *openai.Client, transcribedText, esn string, isKG bool) (*openai.ChatCompletionStream, openai.ChatCompletionRequest, error) {
	aireq := CreateAIReq(transcribedText, esn, false, isKG)
	stream, err := c.CreateChatCompletionStream(ctx, aireq)
	if err == nil {
		return stream, aireq, nil
	}

	log.Printf("Error creating chat completion stream: %v", err)
	if strings.Contains(err.Error(), "does not exist") && vars.APIConfig.Knowledge.Provider == "openai" {
		logger.Println("GPT-4 model cannot be accessed with this API key. You likely need to add more than $5 dollars of funds to your OpenAI account.")
		logger.LogUI("GPT-4 model cannot be accessed with this API key. You likely need to add more than $5 dollars of funds to your OpenAI account.")
		aireq = CreateAIReq(transcribedText, esn, true, isKG)
		logger.Println("Falling back to " + aireq.Model)
		logger.LogUI("Falling back to " + aireq.Model)
		stream, err = c.CreateChatCompletionStream(ctx, aireq)
		if err == nil {
			return stream, aireq, nil
		}
	}

	if hasNativeTools(aireq) {
		logger.Println("Native tool call request failed, retrying without native tools: " + err.Error())
		fallbackReq := withoutNativeTools(aireq)
		stream, fallbackErr := c.CreateChatCompletionStream(ctx, fallbackReq)
		if fallbackErr == nil {
			return stream, fallbackReq, nil
		}
		err = fallbackErr
	}

	return nil, aireq, err
}

// StreamingKGSim 处理 LLM 流式回复，并把文本、动作和机器人行为串成一条完整链路。
func StreamingKGSim(req interface{}, esn string, transcribedText string, isKG bool) (string, error) {
	endActivity := robotpkg.BeginForegroundActivity(esn, "llm_stream")
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
	var finalToolCalls []openai.ToolCall
	var isDone bool
	var deferredActions []func()
	c := newKnowledgeClient()
	speakReady := make(chan string)
	successIntent := make(chan bool, 1)
	prefetch := newTTSPrefetchSession()
	var toolCallStream streamedToolCallAccumulator
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

	stream, aireq, err := createKnowledgeStream(ctx, c, transcribedText, esn, isKG)
	if err != nil {
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
	nChat := aireq.Messages
	nChat = append(nChat, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
	})
	fmt.Println("LLM stream response: ")
	go func() {
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				finalToolCalls = toolCallStream.Calls()
				if len(fullRespSlice) == 0 && strings.TrimSpace(fullfullRespText) != "" {
					logger.Println("LLM debug: final response has no sentence punctuation, using raw content")
					finalChunk := strings.TrimSpace(fullfullRespText)
					fullRespSlice = append(fullRespSlice, finalChunk)
					prefetch.PreloadFromRaw(finalChunk)
					notifySuccess()
				}

				newStr := strings.TrimSpace(strings.Join(fullRespSlice, " "))
				if len(fullRespSlice) > 0 && strings.TrimSpace(newStr) != strings.TrimSpace(fullfullRespText) {
					logger.Println("LLM debug: there is content after the last punctuation mark")
					extraBit := strings.TrimSpace(strings.TrimPrefix(fullRespText, newStr))
					if extraBit != "" {
						fullRespSlice = append(fullRespSlice, extraBit)
						newStr = strings.TrimSpace(strings.Join(fullRespSlice, " "))
					}
				}

				toolMessages := []openai.ChatCompletionMessage{}
				if len(finalToolCalls) > 0 {
					logger.Println("LLM native tool calls: " + fmt.Sprint(finalToolCalls))
					nChat[len(nChat)-1].ToolCalls = append([]openai.ToolCall(nil), finalToolCalls...)
					nChat[len(nChat)-1].Content = newStr
					var needFollowUp bool
					deferredFromTools, toolResults, followUpNeeded := executeNativeToolCalls(finalToolCalls, nativeToolContext{Robot: robot, ESN: esn})
					deferredActions = append(deferredActions, deferredFromTools...)
					toolMessages = append(toolMessages, toolResults...)
					needFollowUp = followUpNeeded || (len(fullRespSlice) == 0 && len(toolResults) > 0)
					if len(toolResults) > 0 {
						nChat = append(nChat, toolResults...)
					}
					if needFollowUp && len(toolResults) > 0 {
						followupText, followErr := createToolFollowup(ctx, c, aireq, nChat)
						if followErr != nil {
							logger.Println("LLM tool follow-up failed: " + followErr.Error())
							followupText = strings.TrimSpace(localToolFollowupFromResults(toolResults))
							if followupText != "" {
								logger.Println("Using local tool follow-up fallback")
							}
						}
						if strings.TrimSpace(followupText) != "" {
							fullRespSlice = append(fullRespSlice, followupText)
							if strings.TrimSpace(fullfullRespText) == "" {
								fullfullRespText = followupText
							} else {
								fullfullRespText = strings.TrimSpace(fullfullRespText + " " + followupText)
							}
							newStr = strings.TrimSpace(strings.Join(fullRespSlice, " "))
							prefetch.PreloadFromRaw(followupText)
							notifySuccess()
							select {
							case speakReady <- followupText:
							default:
							}
							toolMessages = append(toolMessages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: followupText})
						}
					}
					notifySuccess()
				} else {
					nChat[len(nChat)-1].Content = newStr
				}

				logger.Println("LLM final raw: " + clipDebugString(fullfullRespText, 300))
				logger.Println("LLM final slices: " + fmt.Sprint(fullRespSlice))
				if len(fullRespSlice) == 0 && len(finalToolCalls) == 0 {
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
				if vars.APIConfig.Knowledge.SaveChat {
					messagesToRemember := []openai.ChatCompletionMessage{{
						Role:    openai.ChatMessageRoleUser,
						Content: transcribedText,
					}, nChat[len(nChat)-1]}
					if len(toolMessages) > 0 {
						messagesToRemember = append(messagesToRemember, toolMessages...)
					}
					RememberMessages(messagesToRemember, esn)
				}
				if newStr != "" {
					logger.LogUI("LLM response for " + esn + ": " + newStr)
				}
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

			if len(response.Choices[0].Delta.ToolCalls) > 0 {
				toolCallStream.AddDelta(response.Choices[0].Delta.ToolCalls)
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
	for {
		_, ok := <-start
		if !ok {
			break
		}
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
					if len(respSlice) == 0 {
						time.Sleep(50 * time.Millisecond)
						continue
					}
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
			var newDeferred []func()
			disconnect, newDeferred = PerformActions(nChat, acts, robot, stopStop, prefetch)
			deferredActions = append(deferredActions, newDeferred...)
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
			time.Sleep(250 * time.Millisecond)
			for _, deferred := range deferredActions {
				deferred()
			}
		}
		break
	}
	return "", nil
}

// KGSim 是对机器人简短播报能力的兼容封装。
func KGSim(esn string, textToSay string) error {
	return robotpkg.KGSim(esn, textToSay)
}
