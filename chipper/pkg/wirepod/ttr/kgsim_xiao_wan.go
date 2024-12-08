package wirepod_ttr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	sdk_wrapper "github.com/fforchino/vector-go-sdk/pkg/sdk-wrapper"
	"github.com/fforchino/vector-go-sdk/pkg/vector"
	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/agi_modules_for_go/config"
	"github.com/wangergou2023/agi_modules_for_go/xiao_wan"
)

var cfg = config.New()

const enableTTS = true

func Xiao_wan_start(robot *vector.Vector) {

	targets := map[string]string{
		"1": "小丸",
		"2": "风间",
		"3": "哆啦A梦",
		"4": "旁观对话",
	}

	fmt.Println("xiao wan is starting up... Please wait a moment.")

	if robot == nil {
		fmt.Println("robot is nil")
	}

	cfg = cfg.SetOpenAibaseURL("https://llxspace.website/v1")
	cfg = cfg.SetOpenAiAPIKey(vars.APIConfig.Knowledge.Key)

	config := openai.DefaultConfig(cfg.OpenAiAPIKey())
	//need"/v1"
	config.BaseURL = cfg.OpenAibaseURL()
	openaiClient := openai.NewClientWithConfig(config)
	openaiClient_friend_fengjian := openai.NewClientWithConfig(config)
	openaiClient_friend_duolaameng := openai.NewClientWithConfig(config)

	xiao_wan_chat := xiao_wan.Start(cfg, openaiClient, xiao_wan.SystemPrompt, "plugins/for_chat")
	xiao_wan_friend_fengjian := xiao_wan.Start(cfg, openaiClient_friend_fengjian, xiao_wan.FengjianPrompt, "plugins/for_before_chat")
	xiao_wan_friend_duolaameng := xiao_wan.Start(cfg, openaiClient_friend_duolaameng, xiao_wan.DuolaamengPrompt, "plugins/for_before_chat")

	var xiao_wan_chat_tts xiao_wan.Xiao_wan

	if enableTTS {
		openaiClient_tts := openai.NewClientWithConfig(config)
		xiao_wan_chat_tts = xiao_wan.StartTts(cfg, openaiClient_tts)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Conversation")
	fmt.Println("---------------------")

	var response string
	var result xiao_wan.Result

	for {
		fmt.Println("选择对话对象: 1. 小丸 2. 风间 3. 哆啦A梦 4. 旁观对话 (按回车直接旁观)")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		// 如果选择的是旁观对话，输出最新的对话内容，但不输入新消息
		if choice == "4" || choice == "" {
			if response == "" {
				fmt.Println("当前没有可以旁观的对话。")
				continue
			}
		} else {

			targetName, ok := targets[choice]
			if !ok {
				fmt.Println("无效选择，请重试")
				continue
			}

			// 正常输入对话
			fmt.Print("-> ")
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)

			// 构建Result
			result = xiao_wan.Result{
				TargetNames: []string{targetName},
				Sentences: []xiao_wan.Sentence{
					{
						Message: text, // 单句消息内容
					},
				},
				OwnName: "主人",
			}

			// 将 Result 转换为 JSON 字符串
			resultJSON, err := json.Marshal(result)
			if err != nil {
				fmt.Println("转换为 JSON 失败:", err)
				continue
			}
			response = string(resultJSON)
			// fmt.Printf("zhu ren:%s\r\n", response)
			fmt.Printf("zhu ren:%s\r\n", result)
			xiao_wan.SaveConversationToJSON(response)
		}

		// 继续处理对话中的其他消息
		for _, res := range result.TargetNames {
			if res == "小丸" {
				response, result, _ = xiao_wan_chat.Message(response)
				// fmt.Printf("xiao wan:%s\r\n", response)
				fmt.Printf("xiao wan:%s\r\n", result)
				xiao_wan.SaveConversationToJSON(response)
				if enableTTS {
					for i, sentence := range result.Sentences {
						// 将每句话传递给 TTS 接口
						go xiao_wan_chat_tts.Tts(i, sentence.Message, openai.VoiceAlloy)
					}
				}

			} else if res == "风间" {
				response, result, _ = xiao_wan_friend_fengjian.Message(response)
				// fmt.Printf("feng jian:%s\r\n", response)
				fmt.Printf("feng jian:%s\r\n", result)
				xiao_wan.SaveConversationToJSON(response)
				if enableTTS {
					for i, sentence := range result.Sentences {
						// 将每句话传递给 TTS 接口
						go xiao_wan_chat_tts.Tts(i, sentence.Message, openai.VoiceOnyx)
					}
				}

			} else if res == "哆啦A梦" {
				response, result, _ = xiao_wan_friend_duolaameng.Message(response)
				// fmt.Printf("duolaameng:%s\r\n", response)
				fmt.Printf("duolaameng:%s\r\n", result)
				xiao_wan.SaveConversationToJSON(response)
				if enableTTS {
					for i, sentence := range result.Sentences {
						// 将每句话传递给 TTS 接口
						go xiao_wan_chat_tts.Tts(i, sentence.Message, openai.VoiceFable)
					}
				}
			}
		}
	}
}

func StreamingKGSim_xiao_wan(req interface{}, esn string, transcribedText string, isKG bool) (string, error) {

	sdk_wrapper.InitSDKForWirepod(esn)

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

	// 获取电池状态，以确保连接成功
	_, err := robot.Conn.BatteryState(context.Background(), &vectorpb.BatteryStateRequest{})
	if err != nil {
		return "", err
	}

	cfg = cfg.SetOpenAibaseURL("https://llxspace.website/v1")
	cfg = cfg.SetOpenAiAPIKey(vars.APIConfig.Knowledge.Key)

	config := openai.DefaultConfig(cfg.OpenAiAPIKey())
	//need"/v1"
	config.BaseURL = cfg.OpenAibaseURL()
	openaiClient := openai.NewClientWithConfig(config)

	xiao_wan_chat := xiao_wan.Start(cfg, openaiClient, xiao_wan.SystemPrompt, "plugins/for_chat")
	xiao_wan_chat_tts := xiao_wan.StartTts(cfg, openaiClient)

	// 构建Result
	result := xiao_wan.Result{
		TargetNames: []string{"小丸"},
		Sentences: []xiao_wan.Sentence{
			{
				Message: transcribedText,
			},
		},
		OwnName: "主人",
	}

	// 将 Result 转换为 JSON 字符串
	resultJSON, err := json.Marshal(result)
	if err != nil {
		fmt.Println("转换为 JSON 失败:", err)
		return "", err
	}
	response := string(resultJSON)
	// fmt.Printf("zhu ren:%s\r\n", response)
	fmt.Printf("zhu ren:%s\r\n", result)
	xiao_wan.SaveConversationToJSON(response)

	response, result, _ = xiao_wan_chat.Message(response)
	// fmt.Printf("xiao wan:%s\r\n", response)
	fmt.Printf("xiao wan:%s\r\n", result)
	xiao_wan.SaveConversationToJSON(response)

	voiceMap := map[string]openai.SpeechVoice{
		"alloy":   openai.VoiceAlloy,
		"onyx":    openai.VoiceOnyx,
		"fable":   openai.VoiceFable,
		"shimmer": openai.VoiceShimmer,
		"nova":    openai.VoiceNova,
		"echo":    openai.VoiceEcho,
		"":        openai.VoiceFable,
	}

	openaiVoice := voiceMap[vars.APIConfig.Knowledge.OpenAIVoice]

	for i, sentence := range result.Sentences {
		// 将每句话传递给 TTS 接口
		xiao_wan_chat_tts.Tts(i, sentence.Message, openaiVoice)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		start := make(chan bool)
		stop := make(chan bool)

		// BehaviorControl Goroutine
		go func() {
			defer close(start)
			defer close(stop)
			err := sdk_wrapper.Robot.BehaviorControl(ctx, start, stop)
			if err != nil {
				fmt.Println("BehaviorControl error:", err)
			}
		}()

		// 等待 BehaviorControl 启动
		select {
		case <-start:
			fileName := fmt.Sprintf("%s_speech_%d.mp3", vars.APIConfig.Knowledge.OpenAIVoice, i)

			// 确保文件存在
			if _, err := os.Stat(fileName); os.IsNotExist(err) {
				fmt.Printf("File not found: %s\n", fileName)
				return "", fmt.Errorf("TTS file not ready")
			}

			// 播放调整音量后的 mp3
			sdk_wrapper.PlaySound(fileName)

			// 停止 BehaviorControl
			stop <- true
		case <-ctx.Done():
			return "", fmt.Errorf("context canceled")
		}
	}

	return "", nil
}
