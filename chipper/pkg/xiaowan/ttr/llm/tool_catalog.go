package llm

// ToolSpec 描述一个可以暴露给 LLM 的工具能力。
// 这里借鉴 PicoClaw 的“工具目录”思路，把提示词里暴露的能力集中管理，
// 后续新增机器人动作时只需要补一条定义，不用到处改 prompt。
type ToolSpec struct {
	Name            string
	Description     string
	ParamChoices    string
	Action          int
	SupportedModels []string
}

func builtinToolSpecs() []ToolSpec {
	return []ToolSpec{
		{
			Name:            "playAnimationWI",
			Description:     "Plays an animation on the robot without interrupting speech. This should be used FAR more than the playAnimation command. This is great for storytelling and making any normal response animated. Don't put two of these right next to each other. Use this MANY times. The param choices are the only choices you have. You can't create any.",
			ParamChoices:    "happy, veryHappy, sad, verySad, angry, frustrated, dartingEyes, confused, thinking, celebrate, love",
			Action:          ActionPlayAnimationWI,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "playAnimation",
			Description:     "Plays an animation on the robot. This will interrupt speech. Only use this if you are directed to play an animaion.",
			ParamChoices:    "happy, veryHappy, sad, verySad, angry, frustrated, dartingEyes, confused, thinking, celebrate, love",
			Action:          ActionPlayAnimation,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "getImage",
			Description:     "Gets an image from the robot's camera and places it in the next message. If you want to do this, tell the user what you are about to do THEN use the command. This command should END a sentence. Your response will be stopped when this command is recognized. If a user says something like 'what do you see', you should assume that you need to take a new photo. Do NOT automatically assume that you are analyzing a previous photo.",
			ParamChoices:    "front, lookingUp",
			Action:          ActionGetImage,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "headUp",
			Description:     "Makes the robot raise its head a little bit as a subtle gesture. Use it sparingly to support emotion or emphasis, not as repeated motion.",
			ParamChoices:    "now",
			Action:          ActionHeadUp,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "headDown",
			Description:     "Makes the robot lower its head a little bit as a subtle gesture. Use it sparingly to support emotion, thinking, or shyness.",
			ParamChoices:    "now",
			Action:          ActionHeadDown,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "liftUp",
			Description:     "Makes the robot raise its lift arms a little bit. Use it as a short expressive gesture, not for continuous motion.",
			ParamChoices:    "now",
			Action:          ActionLiftUp,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "liftDown",
			Description:     "Makes the robot lower its lift arms a little bit. Use it as a short expressive gesture, not for continuous motion.",
			ParamChoices:    "now",
			Action:          ActionLiftDown,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "nod",
			Description:     "Makes the robot do a small nod. Prefer this over raw headUp or headDown when you want agreement, greeting, or acknowledgement.",
			ParamChoices:    "now",
			Action:          ActionNod,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "lookDownShy",
			Description:     "Makes the robot briefly look down and recover. Use it for shy, embarrassed, gentle, or thoughtful moments.",
			ParamChoices:    "now",
			Action:          ActionLookDownShy,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "raiseArmsHappy",
			Description:     "Makes the robot raise its lift arms and settle back down in a cheerful way. Use it for excitement, pride, or celebration.",
			ParamChoices:    "now",
			Action:          ActionRaiseArmsHappy,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "goCharge",
			Description:     "Makes the robot actually go back to its charger/home and start charging. This is the required command for real charging behavior. If the user asks you to go charge, go home, return to the charger, or head back to the dock, you MUST use this command. Only talking about charging without this command does not complete the request.",
			ParamChoices:    "now",
			Action:          ActionGoCharge,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "newVoiceRequest",
			Description:     "Starts a new voice command from the robot. Use this if you want more input from the user after your response/if you want to carry out a conversation. Below this, there should be a NOTE telling you whether you are in conversation mode or not. If you are, DONT BE AFRAID TO USE THIS COMMAND! This goes at the end of your response, if you use it.",
			ParamChoices:    "now",
			Action:          ActionNewRequest,
			SupportedModels: []string{"all"},
		},
	}
}

func validLLMCommands() []LLMCommand {
	specs := builtinToolSpecs()
	commands := make([]LLMCommand, 0, len(specs))
	for _, spec := range specs {
		commands = append(commands, LLMCommand{
			Command:         spec.Name,
			Description:     spec.Description,
			ParamChoices:    spec.ParamChoices,
			Action:          spec.Action,
			SupportedModels: spec.SupportedModels,
		})
	}
	return commands
}
