package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
	memorypkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/memory"
	skillspkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/skills"
	robotpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/robot"
	visionpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/vision"
	workspacepkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/workspace"
)

const speechAnimationWICooldown = 1200 * time.Millisecond
const speechMotorGestureCooldown = 900 * time.Millisecond

const (
	// arg: text to say
	// not a command
	ActionSayText = 0
	// arg: animation name
	ActionPlayAnimation = 1
	// arg: animation name
	ActionPlayAnimationWI = 2
	// arg: now
	ActionGetImage   = 3
	ActionNewRequest = 4
	// arg: sound file
	ActionPlaySound = 5
	// arg: now
	ActionHeadUp             = 6
	ActionHeadDown           = 7
	ActionLiftUp             = 8
	ActionLiftDown           = 9
	ActionNod                = 10
	ActionLookDownShy        = 11
	ActionRaiseArmsHappy     = 12
	ActionGoCharge           = 13
	ActionTakePhoto          = 14
	ActionCelebrateFireworks = 15
	ActionBackAway           = 16
)

const (
	llmHeadMoveDuration       = 400 * time.Millisecond
	llmLiftMoveDuration       = 500 * time.Millisecond
	llmNodMoveDuration        = 220 * time.Millisecond
	llmLookDownShyDuration    = 650 * time.Millisecond
	llmRaiseArmsHappyDuration = 650 * time.Millisecond
)

var animationMap [][2]string = [][2]string{
	//"happy, veryHappy, sad, verySad, angry, dartingEyes, confused, thinking, celebrate"
	{
		"happy",
		"anim_onboarding_reacttoface_happy_01",
	},
	{
		"veryHappy",
		"anim_blackjack_victorwin_01",
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
		"anim_rtpickup_loop_10",
	},
	{
		"frustrated",
		"anim_feedback_shutup_01",
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
	{
		"love",
		"anim_feedback_iloveyou_02",
	},
}

var soundMap [][2]string = [][2]string{
	{
		"drumroll",
		"sounds/drumroll.wav",
	},
}

type RobotAction struct {
	Action    int
	Parameter string
}

type LLMCommand struct {
	Command         string
	Description     string
	ParamChoices    string
	Action          int
	SupportedModels []string
}

// create function which parses from LLM and makes a struct of RobotActions

var ValidLLMCommands []LLMCommand = validLLMCommands()

func ModelIsSupported(cmd LLMCommand, model string) bool {
	for _, str := range cmd.SupportedModels {
		if str == "all" || str == model {
			return true
		}
	}
	return false
}

func CreatePrompt(origPrompt string, model string, isKG bool) string {
	prompt := origPrompt + "\n\n" + "Keep in mind, user input comes from speech-to-text software, so respond accordingly. No special characters, especially these: & ^ * # @ - . No lists. No formatting."
	if workspacePrompt := strings.TrimSpace(workspacepkg.BuildPromptContext()); workspacePrompt != "" {
		prompt = prompt + "\n\nWorkspace guidance:\n" + workspacePrompt
	}
	if skillPrompt := strings.TrimSpace(skillspkg.BuildAutoSkillPrompt()); skillPrompt != "" {
		prompt = prompt + "\n\n" + "Additional active skills:\n" + skillPrompt
	}
	if vars.APIConfig.Knowledge.CommandsEnable {
		prompt = prompt + "\n\n" + "You are running ON an Anki Vector robot. You have a set of commands. If you include an emoji, I will make you start over. If you want to use a command but it doesn't exist or your desired parameter isn't in the list, avoid using the command. The format is {{command||parameter}}. You can embed these in sentences. Example: \"User: How are you feeling? | Response: \"{{playAnimationWI||sad}} I'm feeling sad...\". Square brackets ([]) are not valid.\n\nUse the playAnimation or playAnimationWI commands if you want to express emotion! You are very animated and good at following instructions. Animation takes precendence over words. You are to include many animations in your response. Head and lift commands are only for subtle physical gestures. Use at most one small motor gesture near a sentence, and do not chain them repeatedly. Prefer higher-level gestures like nod or raiseArmsHappy when they fit. For physical task requests, the real task command is more important than emotional gestures.\n\nImportant rules:\n- If the user asks you to go home, return to the charger, go charge, go back to charge, head to the charger, or similar, you MUST include {{goCharge||now}} in your response.\n- If the user asks you to actually take a photo, snap a picture, or capture a photo, use {{takePhoto||now}}. If the user wants visual analysis of the current scene, use getImage instead.\n- If the user asks for fireworks, celebration, or new year style celebration, prefer {{celebrateFireworks||now}}.\n- If the user asks the robot to move back or give space, use {{backAway||now}}.\n\nExamples:\nUser: 回家去充电\nResponse: 好的，我现在回充电座。{{goCharge||now}}\nUser: 给我拍张照\nResponse: 好的，我来拍一张。{{takePhoto||now}}\nUser: 放个烟花庆祝一下\nResponse: 好呀，我们庆祝一下。{{celebrateFireworks||now}}\nUser: 你往后退一点\nResponse: 好的，我退后一点。{{backAway||now}}\n\nHere is every valid command:"
		for _, cmd := range ValidLLMCommands {
			if ModelIsSupported(cmd, model) {
				promptAppendage := "\n\nCommand Name: " + cmd.Command + "\nDescription: " + cmd.Description + "\nParameter choices: " + cmd.ParamChoices
				prompt = prompt + promptAppendage
			}
		}
		if isKG && vars.APIConfig.Knowledge.SaveChat {
			promptAppentage := "\n\nNOTE: You are in 'conversation' mode. If you ask the user a question near the end of your response, you MUST use newVoiceRequest. If you decide you want to end the conversation, you should not use it."
			prompt = prompt + promptAppentage
		} else {
			promptAppentage := "\n\nNOTE: You are NOT in 'conversation' mode. Refrain from asking the user any questions and from using newVoiceRequest."
			prompt = prompt + promptAppentage
		}
	}
	if os.Getenv("DEBUG_PRINT_PROMPT") == "true" {
		logger.Println(prompt)
	}
	return prompt
}

func createPromptWithMemory(origPrompt, model, esn string, isKG bool) string {
	prompt := CreatePrompt(origPrompt, model, isKG)
	if profilePrompt := strings.TrimSpace(memorypkg.BuildPromptContext(esn)); profilePrompt != "" {
		prompt = prompt + "\n\nLong-term user profile:\n" + profilePrompt
	}
	if facePrompt := strings.TrimSpace(visionpkg.BuildPromptContext(esn)); facePrompt != "" {
		prompt = prompt + "\n\nLive face context:\n" + facePrompt
	}
	return prompt
}

func GetActionsFromString(input string) []RobotAction {
	splitInput := strings.Split(input, "{{")
	if len(splitInput) == 1 {
		return []RobotAction{
			{
				Action:    ActionSayText,
				Parameter: input,
			},
		}
	}
	var actions []RobotAction
	for _, spl := range splitInput {
		if strings.TrimSpace(spl) == "" {
			continue
		}
		if !strings.Contains(spl, "}}") {
			// sayText
			action := RobotAction{
				Action:    ActionSayText,
				Parameter: strings.TrimSpace(spl),
			}
			actions = append(actions, action)
			continue
		}

		commandAndTail := strings.SplitN(spl, "}}", 2)
		commandText := strings.TrimSpace(commandAndTail[0])
		cmdPlusParam := strings.SplitN(commandText, "||", 2)
		cmd := strings.TrimSpace(cmdPlusParam[0])
		param := ""
		// 某些流式片段里模型会输出 {{lookDownShy}} 这种无参数命令。
		// 对这类“now”型命令自动补默认参数，避免预取阶段因切片越界崩溃。
		if len(cmdPlusParam) > 1 {
			param = strings.TrimSpace(cmdPlusParam[1])
		} else {
			param = "now"
		}
		action := CmdParamToAction(cmd, param)
		if action.Action != -1 {
			actions = append(actions, action)
		}
		if len(commandAndTail) > 1 {
			action := RobotAction{
				Action:    ActionSayText,
				Parameter: strings.TrimSpace(commandAndTail[1]),
			}
			actions = append(actions, action)
		}
	}
	return actions
}

func CmdParamToAction(cmd, param string) RobotAction {
	for _, command := range ValidLLMCommands {
		if cmd == command.Command {
			return RobotAction{
				Action:    command.Action,
				Parameter: param,
			}
		}
	}
	logger.Println("LLM tried to do a command which doesn't exist: " + cmd + " (param: " + param + ")")
	return RobotAction{
		Action: -1,
	}
}

// DoPlayAnimation 通过 robot 控制层播放会打断语音的动画。
func DoPlayAnimation(animation string, robot *vector.Vector) error {
	for _, animThing := range animationMap {
		if animation == animThing[0] {
			StartAnim_Queue(robot.Cfg.SerialNo)
			err := robotpkg.PlayAnimationWithSDK(robot, animThing[1], 1, false, false, false)
			StopAnim_Queue(robot.Cfg.SerialNo)
			return err
		}
	}
	logger.Println("Animation provided by LLM doesn't exist: " + animation)
	return nil
}

// DoPlayAnimationWI 通过 robot 控制层异步播放不打断语音的动画。
func DoPlayAnimationWI(animation string, robot *vector.Vector) error {
	if robot == nil {
		return nil
	}
	if !allowSpeechAnimationWI(robot.Cfg.SerialNo, animation) {
		return nil
	}
	for _, animThing := range animationMap {
		if animation == animThing[0] {
			go func() {
				StartAnim_Queue(robot.Cfg.SerialNo)
				robotpkg.PlayAnimationWithSDK(robot, animThing[1], 1, false, false, false)
				StopAnim_Queue(robot.Cfg.SerialNo)
			}()
			return nil
		}
	}
	logger.Println("Animation provided by LLM doesn't exist: " + animation)
	return nil
}

func DoPlaySound(sound string, robot *vector.Vector) error {
	for _, soundThing := range soundMap {
		if sound == soundThing[0] {
			logger.Println("Would play sound")
		}
	}
	logger.Println("Sound provided by LLM doesn't exist: " + sound)
	return nil
}

// DoHeadUp 让 LLM 只能触发一个很短的安全抬头动作，避免长时间占用电机。
func DoHeadUp(robot *vector.Vector) error {
	return robotpkg.HeadUpFor(robot, llmHeadMoveDuration)
}

// DoHeadDown 让 LLM 只能触发一个很短的安全低头动作。
func DoHeadDown(robot *vector.Vector) error {
	return robotpkg.HeadDownFor(robot, llmHeadMoveDuration)
}

// DoLiftUp 让 LLM 只能触发一个很短的安全抬臂动作。
func DoLiftUp(robot *vector.Vector) error {
	return robotpkg.LiftUpFor(robot, llmLiftMoveDuration)
}

// DoLiftDown 让 LLM 只能触发一个很短的安全落臂动作。
func DoLiftDown(robot *vector.Vector) error {
	return robotpkg.LiftDownFor(robot, llmLiftMoveDuration)
}

// DoNod 用一个短促的低头再抬头组合，表达确认或回应。
func DoNod(robot *vector.Vector) error {
	if err := robotpkg.HeadDownFor(robot, llmNodMoveDuration); err != nil {
		return err
	}
	return robotpkg.HeadUpFor(robot, llmNodMoveDuration)
}

// DoLookDownShy 让机器人短暂低头，适合害羞、委屈、思考等语气。
func DoLookDownShy(robot *vector.Vector) error {
	if err := robotpkg.HeadDownFor(robot, llmLookDownShyDuration); err != nil {
		return err
	}
	return robotpkg.HeadUpFor(robot, llmHeadMoveDuration)
}

// DoRaiseArmsHappy 通过抬臂再回落做一个简短的开心手势。
func DoRaiseArmsHappy(robot *vector.Vector) error {
	if err := robotpkg.LiftUpFor(robot, llmRaiseArmsHappyDuration); err != nil {
		return err
	}
	return robotpkg.LiftDownFor(robot, llmLiftMoveDuration)
}

// DoGoCharge 触发机器人回充电座，给 LLM 一个真正可执行的高层能力。
func DoGoCharge(robot *vector.Vector) error {
	if robot != nil {
		logger.Println("LLM action executing: goCharge for " + robot.Cfg.SerialNo)
	} else {
		logger.Println("LLM action executing: goCharge for <nil robot>")
	}
	return robotpkg.GoCharge(robot)
}

// DoTakePhoto 触发机器人执行真实拍照。
func DoTakePhoto(robot *vector.Vector) error {
	if robot != nil {
		logger.Println("LLM action executing: takePhoto for " + robot.Cfg.SerialNo)
	}
	return robotpkg.TakePhoto(robot)
}

// DoCelebrateFireworks 播放一个固定烟花庆祝动画。
func DoCelebrateFireworks(robot *vector.Vector) error {
	if robot != nil {
		logger.Println("LLM action executing: celebrateFireworks for " + robot.Cfg.SerialNo)
	}
	return robotpkg.CelebrateFireworks(robot)
}

// DoBackAway 让机器人后退一点。
func DoBackAway(robot *vector.Vector) error {
	if robot != nil {
		logger.Println("LLM action executing: backAway for " + robot.Cfg.SerialNo)
	}
	return robotpkg.BackAway(robot)
}

// DoSayText 统一走文本播报入口，优先复用预生成好的智谱 TTS，失败时再回退到机器人原生播报。
func DoSayText(input string, robot *vector.Vector, prefetch *ttsPrefetchSession) error {
	normalizedInput := normalizeSpeechText(input)
	if normalizedInput == "" {
		return nil
	}

	if bigModelTTSEnabled() {
		if speechBytes, ok, err := prefetch.Take(normalizedInput); ok {
			if err == nil {
				return DoSayText_BigModelWithAudio(robot, speechBytes)
			}
			logger.Println("BigModel TTS prefetched audio failed, falling back to direct request: " + err.Error())
		}
		if err := DoSayText_BigModel(robot, normalizedInput); err == nil {
			return nil
		} else {
			logger.Println("BigModel TTS failed, falling back to SDK voice: " + err.Error())
		}
	}
	return robotpkg.SayTextWithSDK(robot, normalizedInput)
}

func DoGetImage(msgs []openai.ChatCompletionMessage, param string, robot *vector.Vector, stopStop chan bool) {
	stopImaging := false
	go func() {
		for range stopStop {
			stopImaging = true
			break
		}
	}()
	logger.Println("Get image here...")
	// get image
	robot.Conn.EnableMirrorMode(context.Background(), &vectorpb.EnableMirrorModeRequest{
		Enable: true,
	})
	for i := 3; i > 0; i-- {
		if stopImaging {
			return
		}
		time.Sleep(time.Millisecond * 300)
		robot.Conn.SayText(
			context.Background(),
			&vectorpb.SayTextRequest{
				Text:           fmt.Sprint(i),
				UseVectorVoice: true,
				DurationScalar: 1.05,
			},
		)
		if stopImaging {
			return
		}
	}
	resp, _ := robot.Conn.CaptureSingleImage(
		context.Background(),
		&vectorpb.CaptureSingleImageRequest{
			EnableHighResolution: true,
		},
	)
	robot.Conn.EnableMirrorMode(
		context.Background(),
		&vectorpb.EnableMirrorModeRequest{
			Enable: false,
		},
	)
	go func() {
		robot.Conn.PlayAnimation(
			context.Background(),
			&vectorpb.PlayAnimationRequest{
				Animation: &vectorpb.Animation{
					Name: "anim_photo_shutter_01",
				},
				Loops: 1,
			},
		)
	}()
	// encode to base64
	reqBase64 := base64.StdEncoding.EncodeToString(resp.Data)

	// add image to messages
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL:    fmt.Sprintf("data:image/jpeg;base64,%s", reqBase64),
					Detail: openai.ImageURLDetailLow,
				},
			},
		},
	})

	// recreate openai
	var fullRespText string
	var fullfullRespText string
	var fullRespSlice []string
	var isDone bool
	c := newKnowledgeClient()
	ctx := context.Background()
	speakReady := make(chan string)
	prefetch := newTTSPrefetchSession()

	aireq := openai.ChatCompletionRequest{
		MaxTokens:        2048,
		Temperature:      1,
		TopP:             1,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
		Messages:         msgs,
		Stream:           true,
	}
	aireq.Model = getKnowledgeModel(false)
	logKnowledgeModel(aireq.Model)
	if stopImaging {
		return
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
				return
			}
		} else {
			logger.Println("LLM error: " + err.Error())
			return
		}
	}
	//defer stream.Close()

	fmt.Println("LLM stream response: ")
	go func() {
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				isDone = true
				if len(fullRespSlice) == 0 && strings.TrimSpace(fullfullRespText) != "" {
					logger.Println("LLM debug: final response has no sentence punctuation, using raw content")
					finalChunk := strings.TrimSpace(fullfullRespText)
					fullRespSlice = append(fullRespSlice, finalChunk)
					prefetch.PreloadFromRaw(finalChunk)
				}
				logger.Println("LLM final raw: " + clipDebugString(fullfullRespText, 300))
				logger.Println("LLM final slices: " + fmt.Sprint(fullRespSlice))
				if len(fullRespSlice) == 0 {
					logger.Println("LLM returned no response")
					return
				}
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
					Remember(msgs[len(msgs)-1],
						openai.ChatCompletionMessage{
							Role:    openai.ChatMessageRoleAssistant,
							Content: newStr,
						},
						robot.Cfg.SerialNo)
				}
				logger.LogUI("LLM response for " + robot.Cfg.SerialNo + ": " + newStr)
				logger.Println("LLM stream finished")
				return
			}

			if err != nil {
				logger.Println("Stream error: " + err.Error())
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
				select {
				case speakReady <- nextSentence:
				default:
				}
			}
		}
	}()
	numInResp := 0
	for {
		if stopImaging {
			return
		}
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
		logger.Println(respSlice[numInResp])
		acts := GetActionsFromString(respSlice[numInResp])
		PerformActions(msgs, acts, robot, stopStop, prefetch)
		numInResp = numInResp + 1
		if stopImaging {
			return
		}
	}
}

func DoNewRequest(robot *vector.Vector) {
	time.Sleep(time.Second / 3)
	_ = robotpkg.StartKnowledgeQuestion(robot)
}

func PerformActions(msgs []openai.ChatCompletionMessage, actions []RobotAction, robot *vector.Vector, stopStop chan bool, prefetch *ttsPrefetchSession) (bool, []func()) {
	// assuming we have behavior control already
	stopPerforming := false
	allowWIDuringThisSentence := true
	allowMotorGestureDuringThisSentence := true
	var deferredActions []func()
	go func() {
		for range stopStop {
			stopPerforming = true
		}
	}()
	for _, action := range actions {
		if stopPerforming {
			return false, deferredActions
		}
		switch {
		case action.Action == ActionSayText:
			DoSayText(action.Parameter, robot, prefetch)
		case action.Action == ActionPlayAnimation:
			DoPlayAnimation(action.Parameter, robot)
		case action.Action == ActionPlayAnimationWI:
			if allowWIDuringThisSentence {
				DoPlayAnimationWI(action.Parameter, robot)
				allowWIDuringThisSentence = false
			}
		case action.Action == ActionHeadUp:
			if allowMotorGestureDuringThisSentence && allowSpeechMotorGesture(robot.Cfg.SerialNo, "headUp") {
				DoHeadUp(robot)
				allowMotorGestureDuringThisSentence = false
			}
		case action.Action == ActionHeadDown:
			if allowMotorGestureDuringThisSentence && allowSpeechMotorGesture(robot.Cfg.SerialNo, "headDown") {
				DoHeadDown(robot)
				allowMotorGestureDuringThisSentence = false
			}
		case action.Action == ActionLiftUp:
			if allowMotorGestureDuringThisSentence && allowSpeechMotorGesture(robot.Cfg.SerialNo, "liftUp") {
				DoLiftUp(robot)
				allowMotorGestureDuringThisSentence = false
			}
		case action.Action == ActionLiftDown:
			if allowMotorGestureDuringThisSentence && allowSpeechMotorGesture(robot.Cfg.SerialNo, "liftDown") {
				DoLiftDown(robot)
				allowMotorGestureDuringThisSentence = false
			}
		case action.Action == ActionNod:
			if allowMotorGestureDuringThisSentence && allowSpeechMotorGesture(robot.Cfg.SerialNo, "nod") {
				DoNod(robot)
				allowMotorGestureDuringThisSentence = false
			}
		case action.Action == ActionLookDownShy:
			if allowMotorGestureDuringThisSentence && allowSpeechMotorGesture(robot.Cfg.SerialNo, "lookDownShy") {
				DoLookDownShy(robot)
				allowMotorGestureDuringThisSentence = false
			}
		case action.Action == ActionRaiseArmsHappy:
			if allowMotorGestureDuringThisSentence && allowSpeechMotorGesture(robot.Cfg.SerialNo, "raiseArmsHappy") {
				DoRaiseArmsHappy(robot)
				allowMotorGestureDuringThisSentence = false
			}
		case action.Action == ActionGoCharge:
			deferredActions = append(deferredActions, func() {
				DoGoCharge(robot)
			})
		case action.Action == ActionTakePhoto:
			DoTakePhoto(robot)
		case action.Action == ActionCelebrateFireworks:
			DoCelebrateFireworks(robot)
		case action.Action == ActionBackAway:
			DoBackAway(robot)
		case action.Action == ActionNewRequest:
			go DoNewRequest(robot)
			return true, deferredActions
		case action.Action == ActionGetImage:
			DoGetImage(msgs, action.Parameter, robot, stopStop)
			return true, deferredActions
		case action.Action == ActionPlaySound:
			DoPlaySound(action.Parameter, robot)
		}
	}
	WaitForAnim_Queue(robot.Cfg.SerialNo)
	return false, deferredActions
}

func WaitForAnim_Queue(esn string) {
	for i, q := range AnimationQueues {
		if q.ESN == esn {
			if q.AnimCurrentlyPlaying {
				for range AnimationQueues[i].AnimDone {
					break
				}
				return
			}
		}
	}
}

func StartAnim_Queue(esn string) {
	// if animation is already playing, just wait for it to be done
	for i, q := range AnimationQueues {
		if q.ESN == esn {
			if q.AnimCurrentlyPlaying {
				for range AnimationQueues[i].AnimDone {
					logger.Println("(waiting for animation to be done...)")
					break
				}
			} else {
				AnimationQueues[i].AnimCurrentlyPlaying = true
			}
			return
		}
	}
	var aq AnimationQueue
	aq.AnimCurrentlyPlaying = true
	aq.AnimDone = make(chan bool)
	aq.ESN = esn
	AnimationQueues = append(AnimationQueues, aq)
}

func StopAnim_Queue(esn string) {
	for i, q := range AnimationQueues {
		if q.ESN == esn {
			AnimationQueues[i].AnimCurrentlyPlaying = false
			select {
			case AnimationQueues[i].AnimDone <- true:
			default:
			}
		}
	}
}

type AnimationQueue struct {
	ESN                  string
	AnimDone             chan bool
	AnimCurrentlyPlaying bool
}

var AnimationQueues []AnimationQueue

type speechAnimationWIState struct {
	LastAnimation string
	LastPlayedAt  time.Time
}

type speechMotorGestureState struct {
	LastGesture  string
	LastPlayedAt time.Time
}

var (
	speechAnimationWIMu      sync.Mutex
	speechAnimationWIStates  = map[string]speechAnimationWIState{}
	speechMotorGestureMu     sync.Mutex
	speechMotorGestureStates = map[string]speechMotorGestureState{}
)

// allowSpeechAnimationWI 对说话期的 WI 动作做轻量节流，避免一句里动作太密导致排队抢占。
func allowSpeechAnimationWI(esn, animation string) bool {
	speechAnimationWIMu.Lock()
	defer speechAnimationWIMu.Unlock()

	state := speechAnimationWIStates[esn]
	now := time.Now()
	if !state.LastPlayedAt.IsZero() && now.Sub(state.LastPlayedAt) < speechAnimationWICooldown {
		return false
	}
	if state.LastAnimation == animation && !state.LastPlayedAt.IsZero() && now.Sub(state.LastPlayedAt) < 3*time.Second {
		return false
	}

	speechAnimationWIStates[esn] = speechAnimationWIState{
		LastAnimation: animation,
		LastPlayedAt:  now,
	}
	return true
}

// allowSpeechMotorGesture 对头部/手臂小动作做节流，避免一句话里连续点头抬臂。
func allowSpeechMotorGesture(esn, gesture string) bool {
	speechMotorGestureMu.Lock()
	defer speechMotorGestureMu.Unlock()

	state := speechMotorGestureStates[esn]
	now := time.Now()
	if !state.LastPlayedAt.IsZero() && now.Sub(state.LastPlayedAt) < speechMotorGestureCooldown {
		return false
	}
	if state.LastGesture == gesture && !state.LastPlayedAt.IsZero() && now.Sub(state.LastPlayedAt) < 2500*time.Millisecond {
		return false
	}

	speechMotorGestureStates[esn] = speechMotorGestureState{
		LastGesture:  gesture,
		LastPlayedAt: now,
	}
	return true
}
