package wirepod_ttr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/fforchino/vector-go-sdk/pkg/vector"
	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	xiao_wan "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan"
	xiao_wan_config "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/config"
)

func getChat(esn string) vars.RememberedChat {
	for _, chat := range vars.RememberedChats {
		if chat.ESN == esn {
			return chat
		}
	}
	return vars.RememberedChat{
		ESN: esn,
	}
}

func placeChat(chat vars.RememberedChat) {
	for i, achat := range vars.RememberedChats {
		if achat.ESN == chat.ESN {
			vars.RememberedChats[i] = chat
			return
		}
	}
	vars.RememberedChats = append(vars.RememberedChats, chat)
	vars.SaveChats()
}

// remember last 16 lines of chat
func remember(user, ai, esn string) {
	chatAppend := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleUser,
			Content: user,
		},
		{
			Role:    openai.ChatMessageRoleAssistant,
			Content: ai,
		},
	}
	currentChat := getChat(esn)
	if len(currentChat.Chats) == 16 {
		var newChat vars.RememberedChat
		newChat.ESN = currentChat.ESN
		for i, chat := range currentChat.Chats {
			if i < 2 {
				continue
			}
			newChat.Chats = append(newChat.Chats, chat)
		}
		currentChat = newChat
	}
	currentChat.ESN = esn
	currentChat.Chats = append(currentChat.Chats, chatAppend...)
	placeChat(currentChat)
}

var cfg = xiao_wan_config.New()

func Xiao_wan_test(transcribedText string) (string, error) {

	config := openai.DefaultConfig(cfg.OpenAiAPIKey())
	//need"/v1"
	config.BaseURL = cfg.OpenAibaseURL()
	openaiClient := openai.NewClientWithConfig(config)

	xiao_wan_vector := xiao_wan.Start(cfg, openaiClient)

	xiao_wan_vector.Message(transcribedText)

	return "", nil
}

func StreamingKGSim_xiao_wan(req interface{}, esn string, transcribedText string) (string, error) {

	config := openai.DefaultConfig(cfg.OpenAiAPIKey())
	//need"/v1"
	config.BaseURL = cfg.OpenAibaseURL()
	openaiClient := openai.NewClientWithConfig(config)

	xiao_wan_vector := xiao_wan.Start(cfg, openaiClient)

	xiao_wan_vector.Message(transcribedText)

	return "", nil
}

func StreamingKGSim_test(req interface{}, esn string, transcribedText string) (string, error) {
	var fullRespText string
	var fullRespSlice []string
	var isDone bool
	var c *openai.Client
	if vars.APIConfig.Knowledge.Provider == "together" {
		if vars.APIConfig.Knowledge.Model == "" {
			vars.APIConfig.Knowledge.Model = "meta-llama/Llama-2-70b-chat-hf"
			vars.WriteConfigToDisk()
		}
		conf := openai.DefaultConfig(vars.APIConfig.Knowledge.Key)
		conf.BaseURL = "https://api.together.xyz/v1"
		c = openai.NewClientWithConfig(conf)
	} else if vars.APIConfig.Knowledge.Provider == "openai" {
		c = openai.NewClient(vars.APIConfig.Knowledge.Key)
	}
	ctx := context.Background()
	speakReady := make(chan string)

	var robName string
	if vars.APIConfig.Knowledge.RobotName != "" {
		robName = vars.APIConfig.Knowledge.RobotName
	} else {
		robName = "Vector"
	}
	defaultPrompt := "You are a helpful robot called " + robName + ". The prompt may not be punctuated or spelled correctly as the STT model is small. The answer will be put through TTS, so it should be a speakable string. Keep the answer concise yet informative."

	var nChat []openai.ChatCompletionMessage

	smsg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
	}
	if strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt) != "" {
		smsg.Content = strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt)
	} else {
		smsg.Content = defaultPrompt
	}

	nChat = append(nChat, smsg)
	if vars.APIConfig.Knowledge.SaveChat {
		rchat := getChat(esn)
		logger.Println("Using remembered chats, length of " + fmt.Sprint(len(rchat.Chats)) + " messages")
		nChat = append(nChat, rchat.Chats...)
	}
	nChat = append(nChat, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: transcribedText,
	})

	aireq := openai.ChatCompletionRequest{
		MaxTokens: 2048,
		Messages:  nChat,
		Stream:    true,
	}
	if vars.APIConfig.Knowledge.Provider == "openai" {
		aireq.Model = openai.GPT4Turbo1106
		logger.Println("Using " + aireq.Model)
	} else {
		logger.Println("Using " + vars.APIConfig.Knowledge.Model)
		aireq.Model = vars.APIConfig.Knowledge.Model
	}
	stream, err := c.CreateChatCompletionStream(ctx, aireq)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") && vars.APIConfig.Knowledge.Provider == "openai" {
			logger.Println("GPT-4 model cannot be accessed with this API key. You likely need to add more than $5 dollars of funds to your OpenAI account.")
			logger.LogUI("GPT-4 model cannot be accessed with this API key. You likely need to add more than $5 dollars of funds to your OpenAI account.")
			aireq.Model = openai.GPT3Dot5Turbo
			logger.Println("Falling back to " + aireq.Model)
			logger.LogUI("Falling back to " + aireq.Model)
			stream, err = c.CreateChatCompletionStream(ctx, aireq)
			if err != nil {
				logger.Println("OpenAI still not returning a response even after falling back. Erroring.")
				return "", err
			}
		} else {
			return "", err
		}
	}
	//defer stream.Close()

	fmt.Println("LLM stream response: ")
	// 启动一个新的goroutine来异步处理流数据。
	go func() {
		// 无限循环，持续监听和处理从流中接收的数据。
		for {
			// 从流中接收数据。每次调用Recv()将等待并获取一个响应，或返回错误。
			response, err := stream.Recv()

			// 检查是否收到了文件结束标记（EOF），这表示流已经关闭。
			if errors.Is(err, io.EOF) {
				isDone = true // 设置标志，表示处理完毕。
				// 检查是否有响应片段可用以构造最终响应
				var newStr string
				if len(fullRespSlice) > 0 {
					newStr = fullRespSlice[0] // 安全地初始化newStr为第一个响应片段。
					for i, str := range fullRespSlice {
						if i == 0 {
							continue // 跳过第一个元素，因为它已经被初始化到newStr中。
						}
						// 将剩余的响应片段拼接到newStr中。
						newStr = newStr + " " + str
					}
				} else {
					newStr = "" // 处理fullRespSlice为空的情况
					logger.Println("Warning: fullRespSlice is empty, no data to process.")
				}
				// 如果配置中启用了保存聊天功能，则保存转录文本和响应。
				if vars.APIConfig.Knowledge.SaveChat {
					remember(transcribedText, newStr, esn)
				}
				// 向用户界面日志输出完整响应和ESN标识。
				logger.LogUI("LLM response for " + esn + ": " + newStr)
				logger.Println("LLM stream finished") // 日志记录流处理完成。
				return                                // 结束goroutine。
			}

			// 如果接收数据时发生错误，记录错误并结束goroutine。
			if err != nil {
				logger.Println("Stream error: " + err.Error())
				return
			}
			// 日志打印接收到的内容
			logger.Println("Received content: ", response.Choices[0].Delta.Content)
			// 将接收到的内容添加到完整响应文本中。
			fullRespText = fullRespText + response.Choices[0].Delta.Content

			// 检查完整响应文本中是否包含预定义的句末标点。
			if strings.Contains(fullRespText, "...") || strings.Contains(fullRespText, ".'") || strings.Contains(fullRespText, ".\"") ||
				strings.Contains(fullRespText, ".") || strings.Contains(fullRespText, "?") || strings.Contains(fullRespText, "!") || strings.Contains(fullRespText, "，") || strings.Contains(fullRespText, "。") || strings.Contains(fullRespText, "？") || strings.Contains(fullRespText, "！") {
				var sepStr string // 定义用于分割响应的字符串变量。
				// 根据包含的句末标点确定分割符。
				if strings.Contains(fullRespText, "...") {
					sepStr = "..."
				} else if strings.Contains(fullRespText, ".'") {
					sepStr = ".'"
				} else if strings.Contains(fullRespText, ".\"") {
					sepStr = ".\""
				} else if strings.Contains(fullRespText, ".") {
					sepStr = "."
				} else if strings.Contains(fullRespText, "?") {
					sepStr = "?"
				} else if strings.Contains(fullRespText, "!") {
					sepStr = "!"
				} else if strings.Contains(fullRespText, "，") {
					sepStr = "，"
				} else if strings.Contains(fullRespText, "。") {
					sepStr = "。"
				} else if strings.Contains(fullRespText, "？") {
					sepStr = "？"
				} else if strings.Contains(fullRespText, "！") {
					sepStr = "！"
				}
				// 根据分割符将完整响应文本分割为两部分。
				splitResp := strings.Split(strings.TrimSpace(fullRespText), sepStr)
				// 将分割后的第一部分（完整的句子）添加到响应片段数组中。
				fullRespSlice = append(fullRespSlice, strings.TrimSpace(splitResp[0])+sepStr)
				// 更新完整响应文本为剩余的部分。
				fullRespText = splitResp[1]
				// 尝试向speakReady通道发送已处理的完整句子，如果通道不可接收则跳过。
				select {
				case speakReady <- strings.TrimSpace(splitResp[0]) + sepStr:
				default:
				}
			}
		}
	}() // 监听 'speakReady' 通道，一旦接收到信号，就处理一次问候意图。
	for range speakReady {
		IntentPass(req, "intent_greeting_hello", transcribedText, map[string]string{}, false)
		break // 处理完后立即退出循环
	}

	// 初始化匹配标志为假
	matched := false
	var robot *vector.Vector // 声明一个向量机器人类型的指针变量
	var guid string          // 机器人的全局唯一标识符
	var target string        // 机器人的目标IP和端口字符串

	// 遍历所有已知的机器人信息
	for _, bot := range vars.BotInfo.Robots {
		if esn == bot.Esn { // 如果找到与提供的ESN匹配的机器人
			guid = bot.GUID                 // 获取该机器人的GUID
			target = bot.IPAddress + ":443" // 设置目标IP和端口，端口固定为443
			matched = true                  // 设置匹配标志为真
			break                           // 找到匹配项后退出循环
		}
	}

	// 如果成功匹配到机器人
	if matched {
		var err error
		// 尝试创建一个新的机器人连接实例
		robot, err = vector.New(vector.WithSerialNo(esn), vector.WithToken(guid), vector.WithTarget(target))
		if err != nil {
			return err.Error(), err // 如果创建失败，返回错误
		}
	}

	// 设置行为控制请求的数据结构
	controlRequest := &vectorpb.BehaviorControlRequest{
		RequestType: &vectorpb.BehaviorControlRequest_ControlRequest{
			ControlRequest: &vectorpb.ControlRequest{
				Priority: vectorpb.ControlRequest_OVERRIDE_BEHAVIORS, // 请求优先级：覆盖当前行为
			},
		},
	}

	// 初始化开始和停止信号的通道
	start := make(chan bool)
	stop := make(chan bool)

	// 启动一个goroutine来处理行为控制逻辑
	go func() {
		// 开始行为控制会话
		r, err := robot.Conn.BehaviorControl(
			ctx,
		)
		if err != nil {
			log.Println(err)
			return
		}

		// 发送行为控制请求
		if err := r.Send(controlRequest); err != nil {
			log.Println(err)
			return
		}

		// 等待控制权限确认
		for {
			ctrlresp, err := r.Recv()
			if err != nil {
				log.Println(err)
				return
			}
			if ctrlresp.GetControlGrantedResponse() != nil {
				start <- true // 接收到控制权限后，发送开始信号
				break
			}
		}

		// 持续监听停止信号，以便随时释放控制
		for {
			select {
			case <-stop:
				logger.Println("KGSim: releasing behavior control (interrupt)")
				if err := r.Send(
					&vectorpb.BehaviorControlRequest{
						RequestType: &vectorpb.BehaviorControlRequest_ControlRelease{
							ControlRelease: &vectorpb.ControlRelease{}, // 发送行为控制释放请求
						},
					},
				); err != nil {
					logger.Println(err)
					return
				}
				return
			default:
				continue
			}
		}
	}()

	// 控制文本到语音(TTS)循环的变量
	var stopTTSLoop bool
	TTSLoopStopped := make(chan bool)

	// 监听开始信号，一旦收到，执行以下操作
	for range start {
		time.Sleep(time.Millisecond * 300) // 稍作延迟
		// 播放一次动画
		robot.Conn.PlayAnimation(
			ctx,
			&vectorpb.PlayAnimationRequest{
				Animation: &vectorpb.Animation{
					Name: "anim_getin_tts_01",
				},
				Loops: 1,
			},
		)
		// 启动一个新的goroutine来持续播放TTS动画
		go func() {
			for {
				if stopTTSLoop {
					TTSLoopStopped <- true // 发送TTS循环停止信号
					break
				}
				// 播放循环动画
				robot.Conn.PlayAnimation(
					ctx,
					&vectorpb.PlayAnimationRequest{
						Animation: &vectorpb.Animation{
							Name: "anim_tts_loop_02",
						},
						Loops: 1,
					},
				)
			}
		}()
		// 初始化响应处理计数器
		numInResp := 0
		// 持续处理响应片段
		for {
			respSlice := fullRespSlice        // 获取当前的响应片段列表
			if len(respSlice)-1 < numInResp { // 如果没有新的响应片段可处理
				if !isDone {
					fmt.Println("waiting...") // 打印等待信息
					// 等待新的speakReady信号
					for range speakReady {
						respSlice = fullRespSlice
						break
					}
				} else {
					break
				}
			}
			// 输出当前处理的响应片段
			logger.Println(respSlice[numInResp])
			// 机器人发声
			_, err := robot.Conn.SayText(
				ctx,
				&vectorpb.SayTextRequest{
					Text:           respSlice[numInResp],
					UseVectorVoice: true,
					DurationScalar: 1.0,
				},
			)
			if err != nil {
				logger.Println("KG SayText error: " + err.Error())
				stop <- true // 发送停止信号
				break
			}
			numInResp++ // 处理下一个响应片段
		}
		stopTTSLoop = true // 设置停止TTS循环标志
		// 等待TTS循环停止
		for range TTSLoopStopped {
			time.Sleep(time.Millisecond * 100)
			// 播放成功动画
			robot.Conn.PlayAnimation(
				ctx,
				&vectorpb.PlayAnimationRequest{
					Animation: &vectorpb.Animation{
						Name: "anim_knowledgegraph_success_01",
					},
					Loops: 1,
				},
			)
			// 发送停止信号
			stop <- true
		}
	}

	return "", nil
}

func KGSim_xiao_wan(esn string, textToSay string) error {
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
		start := make(chan bool)
		stop := make(chan bool)

		go func() {
			// * begin - modified from official vector-go-sdk
			r, err := robot.Conn.BehaviorControl(
				ctx,
			)
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
					if err := r.Send(
						&vectorpb.BehaviorControlRequest{
							RequestType: &vectorpb.BehaviorControlRequest_ControlRelease{
								ControlRelease: &vectorpb.ControlRelease{},
							},
						},
					); err != nil {
						log.Println(err)
						return
					}
					return
				default:
					continue
				}
			}
			// * end - modified from official vector-go-sdk
		}()

		var stopTTSLoop bool
		var TTSLoopStopped bool
		for range start {
			time.Sleep(time.Millisecond * 300)
			robot.Conn.PlayAnimation(
				ctx,
				&vectorpb.PlayAnimationRequest{
					Animation: &vectorpb.Animation{
						Name: "anim_getin_tts_01",
					},
					Loops: 1,
				},
			)
			go func() {
				for {
					if stopTTSLoop {
						TTSLoopStopped = true
						break
					}
					robot.Conn.PlayAnimation(
						ctx,
						&vectorpb.PlayAnimationRequest{
							Animation: &vectorpb.Animation{
								Name: "anim_tts_loop_02",
							},
							Loops: 1,
						},
					)
				}
			}()
			textToSaySplit := strings.Split(textToSay, ". ")
			for _, str := range textToSaySplit {
				_, err := robot.Conn.SayText(
					ctx,
					&vectorpb.SayTextRequest{
						Text:           str + ".",
						UseVectorVoice: true,
						DurationScalar: 1.0,
					},
				)
				if err != nil {
					logger.Println("KG SayText error: " + err.Error())
					stop <- true
					break
				}
			}
			stopTTSLoop = true
			for {
				if TTSLoopStopped {
					break
				} else {
					time.Sleep(time.Millisecond * 10)
				}
			}
			time.Sleep(time.Millisecond * 100)
			robot.Conn.PlayAnimation(
				ctx,
				&vectorpb.PlayAnimationRequest{
					Animation: &vectorpb.Animation{
						Name: "anim_knowledgegraph_success_01",
					},
					Loops: 1,
				},
			)
			//time.Sleep(time.Millisecond * 3300)
			stop <- true
		}
	}()
	return nil
}
