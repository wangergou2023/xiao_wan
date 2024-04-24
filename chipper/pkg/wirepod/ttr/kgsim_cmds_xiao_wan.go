package wirepod_ttr // 定义包名

import ( // 导入依赖的包
	"context" // 用于提供跨API和goroutines的上下文管理
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings" // 字符串操作库

	sdk_wrapper "github.com/fforchino/vector-go-sdk/pkg/sdk-wrapper"
	"github.com/fforchino/vector-go-sdk/pkg/vector"   // 引入vector SDK
	"github.com/fforchino/vector-go-sdk/pkg/vectorpb" // 引入vector协议缓冲
	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger" // 自定义的日志包
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"   // 自定义的变量包
)

const ( // 定义常量
	// arg: text to say
	// not a command
	ActionSayText_xiao_wan = 0 // 定义说话动作常量
	// arg: animation name
	ActionPlayAnimation_xiao_wan = 1 // 定义播放动画动作常量
	// arg: animation name
	ActionPlayAnimationWI_xiao_wan = 2 // 定义播放动画（不中断说话）动作常量
	// arg: sound file
	ActionPlaySound_xiao_wan = 3 // 定义播放声音文件动作常量
)

var animationMap_xiao_wan [][2]string = [][2]string{ // 定义动画映射表
	{
		"happy",
		"anim_onboarding_reacttoface_happy_01",
	},
	{
		"veryHappy",
		"anim_onboarding_reacttoface_happy_01",
	},
	{
		"sad",
		"anim_feedback_meanwords_01",
	},
	{
		"verySad",
		"anim_feedback_meanwords_01",
	},
	{
		"angry",
		"anim_keepaway_getout_frustrated_01",
	},
	{
		"frustrated",
		"anim_keepaway_getout_frustrated_01",
	},
	{
		"dartingEyes",
		"anim_observing_self_absorbed_01",
	},
	{
		"confused",
		"anim_meetvictor_lookface_timeout_01",
	},
	{
		"thinking",
		"anim_explorer_scan_short_04",
	},
	{
		"celebrate",
		"anim_pounce_success_03",
	},
}

var soundMap_xiao_wan [][2]string = [][2]string{ // 定义声音映射表
	{
		"drumroll",
		"sounds/drumroll.wav",
	},
}

type RobotAction_xiao_wan struct { // 定义机器人动作结构体
	Action    int
	Parameter string
}

type LLMCommand_xiao_wan struct { // 定义LLM命令结构体
	Command      string
	Description  string
	ParamChoices string
	Action       int
}

// 创建从LLM解析并生成RobotActions结构体的函数
var ValidLLMCommands_xiao_wan []LLMCommand_xiao_wan = []LLMCommand_xiao_wan{ // 定义有效的LLM命令列表
	{
		Command:      "playAnimation",
		Description:  "Plays an animation on the robot. This will interrupt speech.",
		ParamChoices: "happy, veryHappy, sad, verySad, angry, frustrated, dartingEyes, confused, thinking, celebrate",
		Action:       ActionPlayAnimation_xiao_wan,
	},
	{
		Command:      "playAnimationWI",
		Description:  "Plays an animation on the robot without interrupting speech.",
		ParamChoices: "happy, veryHappy, sad, verySad, angry, frustrated, dartingEyes, confused, thinking, celebrate",
		Action:       ActionPlayAnimationWI_xiao_wan,
	},
	// {
	// 	Command:      "playSound",
	// 	Description:  "Plays a sound on the robot.",
	// 	ParamChoices: "drumroll",
	// 	Action:       ActionPlaySound,
	// },
}

func CreatePrompt_xiao_wan(origPrompt string) string { // 创建提示函数，输入原始提示字符串，输出加工后的提示字符串
	prompt := origPrompt + "\n\n" + "用户的输入可能由于使用了语音转文字软件而拼写或标点不正确。请不要在你的回答中包含特殊字符，这包括以下字符（引号内的除外）：'& ^ * # @ -'。如果你想使用连字符，请这样使用：'某事某事 -- 某事 -- 某事某事'"
	if vars.APIConfig.Knowledge.CommandsEnable { // 如果配置允许使用命令
		prompt = prompt + "\n\n" + `你正在操作一个Anki Vector机器人。你有一组命令可以使用。你应该使用这些命令。不要害怕在你的回答中频繁使用它们。你的回答必须包含三个或更多这些命令。你将在你的回答中频繁使用它们。如果你只包含一个命令，我将让你重新开始。如果你包含表情符号，我也会让你重新开始。如果你想使用一个命令但它不存在或你想要的参数不在列表中，请避免使用该命令。格式是{{command||parameter}}。你可以将这些嵌入句子中。例如：“用户：你感觉如何？ | 回应：{{playAnimationWI||sad}}我感觉很难过...”
		不要使用表情符号！如果你想表达情绪，请使用playAnimation或playAnimationWI命令！如果你不遵守这些规则，我将取消你的回应并让你重新开始。你非常生动且善于遵循指令。动画优先于文字。你的回应中应该包含许多动画
		以下是所有有效的命令：`
		for _, cmd := range ValidLLMCommands_xiao_wan { // 遍历有效命令列表，生成命令详细信息
			promptAppendage := "\n\n命令名称: " + cmd.Command + "\n描述: " + cmd.Description + "\n参数选择: " + cmd.ParamChoices // 生成单个命令的描述
			prompt = prompt + promptAppendage                                                                          // 将命令描述添加到提示字符串
		}
	}
	return prompt // 返回最终的提示字符串
}

func CmdParamToAction_xiao_wan(cmd, param string) RobotAction_xiao_wan { // 根据命令名和参数生成动作
	for _, command := range ValidLLMCommands_xiao_wan { // 遍历有效命令列表
		if cmd == command.Command { // 如果命令名匹配
			return RobotAction_xiao_wan{
				Action:    command.Action, // 创建动作
				Parameter: param,
			}
		}
	}
	logger.Println("LLM tried to do a command which doesn't exist: " + cmd + " (param: " + param + ")") // 记录错误
	return RobotAction_xiao_wan{
		Action: -1, // 返回无效动作
	}
}

func GetActionsFromString_xiao_wan(input string) []RobotAction_xiao_wan { // 从字符串中解析出机器人动作数组
	splitInput := strings.Split(input, "{{") // 以"{{"为分隔符分割输入字符串
	if len(splitInput) == 1 {                // 如果没有命令分隔符，只包含普通文本
		return []RobotAction_xiao_wan{
			{
				Action:    ActionSayText_xiao_wan, // 将整个输入字符串作为说话动作
				Parameter: input,
			},
		}
	}
	var actions []RobotAction_xiao_wan // 初始化动作列表
	for _, spl := range splitInput {   // 遍历每一段分割后的字符串
		if strings.TrimSpace(spl) == "" {
			continue // 如果是空字符串，则跳过
		}
		if !strings.Contains(spl, "}}") { // 如果没有命令结束符，则视为普通说话动作
			action := RobotAction_xiao_wan{
				Action:    ActionSayText_xiao_wan,
				Parameter: strings.TrimSpace(spl),
			}
			actions = append(actions, action) // 添加说话动作到列表
			continue
		}

		cmdPlusParam := strings.Split(strings.TrimSpace(strings.Split(spl, "}}")[0]), "||") // 分割命令和参数
		cmd := strings.TrimSpace(cmdPlusParam[0])                                           // 清理命令字符串
		param := strings.TrimSpace(cmdPlusParam[1])                                         // 清理参数字符串
		action := CmdParamToAction_xiao_wan(cmd, param)                                     // 将命令和参数转换为动作
		if action.Action != -1 {
			actions = append(actions, action) // 如果动作有效，添加到动作列表
		}
		if len(strings.Split(spl, "}}")) != 1 {
			action := RobotAction_xiao_wan{
				Action:    ActionSayText_xiao_wan,
				Parameter: strings.TrimSpace(strings.Split(spl, "}}")[1]), // 处理命令后的文本为说话动作
			}
			actions = append(actions, action)
		}
	}
	return actions // 返回解析得到的动作列表
}

// func DoPlayAnimation_xiao_wan(animation string, robot *vector.Vector) error { // 播放动画函数
// 	for _, animThing := range animationMap_xiao_wan { // 遍历动画映射表
// 		if animation == animThing[0] { // 如果找到匹配的动画名称
// 			robot.Conn.PlayAnimation( // 使用vector库的PlayAnimation方法播放动画
// 				context.Background(), // 使用背景上下文
// 				&vectorpb.PlayAnimationRequest{ // 创建播放动画的请求
// 					Animation: &vectorpb.Animation{
// 						Name: animThing[1], // 设置动画文件名
// 					},
// 					Loops: 1, // 播放一次
// 				},
// 			)
// 			return nil // 正常结束，返回nil
// 		}
// 	}
// 	logger.Println("Animation provided by LLM doesn't exist: " + animation) // 如果动画不存在于映射表中，记录日志
// 	return nil                                                              // 返回nil，结束函数
// }

func DoPlayAnimation_xiao_wan(animation string, robot *vector.Vector) error { // 播放动画（不中断说话）函数
	for _, animThing := range animationMap_xiao_wan { // 遍历动画映射表
		if animation == animThing[0] { // 如果找到匹配的动画名称
			go func() { // 使用goroutine来异步执行，因为不说话会立刻结束，所以需要异步播放
				robot.Conn.PlayAnimation( // 使用vector库的PlayAnimation方法播放动画
					context.Background(), // 使用背景上下文
					&vectorpb.PlayAnimationRequest{ // 创建播放动画的请求
						Animation: &vectorpb.Animation{
							Name: animThing[1], // 设置动画文件名
						},
						Loops: 1, // 播放一次
					},
				)
			}()
			return nil // 正常结束，返回nil
		}
	}
	logger.Println("Animation provided by LLM doesn't exist: " + animation) // 如果动画不存在于映射表中，记录日志
	return nil                                                              // 返回nil，结束函数
}

func DoPlayAnimationWI_xiao_wan(animation string, robot *vector.Vector) error { // 播放动画（不中断说话）函数
	for _, animThing := range animationMap_xiao_wan { // 遍历动画映射表
		if animation == animThing[0] { // 如果找到匹配的动画名称
			go func() { // 使用goroutine来异步执行，因为不说话会立刻结束，所以需要异步播放
				robot.Conn.PlayAnimation( // 使用vector库的PlayAnimation方法播放动画
					context.Background(), // 使用背景上下文
					&vectorpb.PlayAnimationRequest{ // 创建播放动画的请求
						Animation: &vectorpb.Animation{
							Name: animThing[1], // 设置动画文件名
						},
						Loops: 1, // 播放一次
					},
				)
			}()
			return nil // 正常结束，返回nil
		}
	}
	logger.Println("Animation provided by LLM doesn't exist: " + animation) // 如果动画不存在于映射表中，记录日志
	return nil                                                              // 返回nil，结束函数
}

func DoPlaySound_xiao_wan(sound string, robot *vector.Vector) error { // 播放声音文件函数
	for _, soundThing := range soundMap_xiao_wan { // 遍历声音映射表
		if sound == soundThing[0] { // 如果找到匹配的声音名称
			// 播放声音的逻辑（当前注释状态，不执行）
			logger.Println("Would play sound") // 记录日志：将播放声音
			return nil                         // 正常结束，返回nil
		}
	}
	logger.Println("Sound provided by LLM doesn't exist: " + sound) // 如果声音不存在于映射表中，记录日志
	return nil                                                      // 返回nil，结束函数
}

func DoSayText_xiao_wan(input string, robot *vector.Vector) error { // 机器人说话函数
	robot.Conn.SayText( // 使用vector库的SayText方法让机器人说话
		context.Background(), // 使用背景上下文
		&vectorpb.SayTextRequest{
			Text:           input, // 设置机器人要说的文本
			UseVectorVoice: true,  // 使用Vector的声音
			DurationScalar: 0.95,  // 设置说话速度的系数
		},
	)
	return nil // 正常结束，返回nil
}

func DoSayText_cn_xiao_wan(input string, robot *vector.Vector) error { // 机器人说话函数
	if input == "" {
		fmt.Println("DoSayText_cn_xiao_wan input == NULL")
		return nil
	}

	config := openai.DefaultConfig(cfg.OpenAiAPIKey())
	//need"/v1"
	config.BaseURL = cfg.OpenAibaseURL()
	openaiClient := openai.NewClientWithConfig(config)

	res, err := openaiClient.CreateSpeech(context.Background(), openai.CreateSpeechRequest{
		Model: openai.TTSModel1,
		Input: input,
		Voice: openai.VoiceAlloy,
	})
	if err != nil {
		panic(err)
	}
	buf, err := io.ReadAll(res)
	if err != nil {
		panic(err)
	}
	// 输出文件路径
	outputFile := "speech.mp3"
	// 检查文件是否存在
	if _, err := os.Stat(outputFile); err == nil {
		// 文件存在，尝试删除
		err := os.Remove(outputFile)
		if err != nil {
			// 删除文件时出错
			log.Fatalf("Failed to delete existing file %s: %s", outputFile, err)
		}
		log.Printf("Existing file %s deleted successfully.\n", outputFile)
	} else if !os.IsNotExist(err) {
		// 访问文件时出现了其他错误
		log.Fatalf("Error checking file %s: %s", outputFile, err)
	}

	outputFile = "speech_loud.mp3"
	// 检查文件是否存在
	if _, err := os.Stat(outputFile); err == nil {
		// 文件存在，尝试删除
		err := os.Remove(outputFile)
		if err != nil {
			// 删除文件时出错
			log.Fatalf("Failed to delete existing file %s: %s", outputFile, err)
		}
		log.Printf("Existing file %s deleted successfully.\n", outputFile)
	} else if !os.IsNotExist(err) {
		// 访问文件时出现了其他错误
		log.Fatalf("Error checking file %s: %s", outputFile, err)
	}

	// 保存 buf 到文件为 mp3
	err = os.WriteFile("speech.mp3", buf, 0644)
	if err != nil {
		panic(err)
	}
	// 使用 ffmpeg 调整音量
	cmd := exec.Command("ffmpeg", "-i", "speech.mp3", "-filter:a", "volume=3.0", "speech_loud.mp3")
	err = cmd.Run()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	start1 := make(chan bool)
	stop1 := make(chan bool)

	go func() {
		_ = sdk_wrapper.Robot.BehaviorControl(ctx, start1, stop1)
	}()

	for {
		select {
		case <-start1:
			// 播放调整音量后的 mp3
			sdk_wrapper.PlaySound("speech_loud.mp3")
			stop1 <- true
			return nil
		}
	}

	return nil // 正常结束，返回nil
}

func PerformActions_xiao_wan(actions []RobotAction_xiao_wan, robot *vector.Vector) { // 执行动作序列函数
	// assuming we have behavior control already
	fmt.Println("PerformActions_xiao_wan开始")
	for _, action := range actions { // 遍历动作列表
		switch action.Action { // 根据动作类型决定执行哪种函数
		case ActionSayText_xiao_wan:
			fmt.Println("执行说话动作" + action.Parameter)
			// DoSayText_xiao_wan(action.Parameter, robot) // 执行说话动作
			DoSayText_cn_xiao_wan(action.Parameter, robot) // 执行说话动作
		case ActionPlayAnimation_xiao_wan:
			fmt.Println("执行播放动画动作" + action.Parameter)
			DoPlayAnimation_xiao_wan(action.Parameter, robot) // 执行播放动画动作
		case ActionPlayAnimationWI_xiao_wan:
			fmt.Println("执行播放动画（不中断说话）动作" + action.Parameter)
			DoPlayAnimationWI_xiao_wan(action.Parameter, robot) // 执行播放动画（不中断说话）动作
		case ActionPlaySound_xiao_wan:
			DoPlaySound_xiao_wan(action.Parameter, robot) // 执行播放声音文件动作
		}
	}
	fmt.Println("PerformActions_xiao_wan结束")
}
