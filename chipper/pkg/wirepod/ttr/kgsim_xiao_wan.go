package wirepod_ttr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/agi_modules_for_go/config"
	"github.com/wangergou2023/agi_modules_for_go/xiao_wan"
)

var cfg = config.New()

const enableTTS = true

func Xiao_wan_start(transcribedText string) (string, error) {

	targets := map[string]string{
		"1": "小丸",
		"2": "风间",
		"3": "哆啦A梦",
		"4": "旁观对话",
	}

	fmt.Println("xiao wan is starting up... Please wait a moment.")

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
				Message:     text,
				OwnName:     "主人",
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
					go xiao_wan_chat_tts.Tts(result.Message, openai.VoiceAlloy)
				}

			} else if res == "风间" {
				response, result, _ = xiao_wan_friend_fengjian.Message(response)
				// fmt.Printf("feng jian:%s\r\n", response)
				fmt.Printf("feng jian:%s\r\n", result)
				xiao_wan.SaveConversationToJSON(response)
				if enableTTS {
					go xiao_wan_chat_tts.Tts(result.Message, openai.VoiceOnyx)
				}

			} else if res == "哆啦A梦" {
				response, result, _ = xiao_wan_friend_duolaameng.Message(response)
				// fmt.Printf("duolaameng:%s\r\n", response)
				fmt.Printf("duolaameng:%s\r\n", result)
				xiao_wan.SaveConversationToJSON(response)
				if enableTTS {
					go xiao_wan_chat_tts.Tts(result.Message, openai.VoiceFable)
				}
			}
		}
	}
	return "", nil
}

func StreamingKGSim_xiao_wan(req interface{}, esn string, transcribedText string) (string, error) {

	return "", nil
}
