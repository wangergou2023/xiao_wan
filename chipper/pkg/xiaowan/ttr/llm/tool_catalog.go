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
			Description:     "Play a non-interrupting emotion animation during speech. Prefer this over playAnimation for normal expressive replies. Use at most one nearby, and only with the listed animation names.",
			ParamChoices:    "happy, veryHappy, sad, verySad, angry, frustrated, dartingEyes, confused, thinking, celebrate, love",
			Action:          ActionPlayAnimationWI,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "playAnimation",
			Description:     "Play an interrupting animation. Use only when the user explicitly wants a stronger animation or speech interruption is acceptable.",
			ParamChoices:    "happy, veryHappy, sad, verySad, angry, frustrated, dartingEyes, confused, thinking, celebrate, love",
			Action:          ActionPlayAnimation,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "getImage",
			Description:     "Capture a fresh camera image for visual analysis. Tell the user first, then end the reply with this command. Use this for 'what do you see' style requests, not for saving a normal photo.",
			ParamChoices:    "front, lookingUp",
			Action:          ActionGetImage,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "headUp",
			Description:     "Do a small upward head gesture. Use sparingly for emphasis or attentiveness.",
			ParamChoices:    "now",
			Action:          ActionHeadUp,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "headDown",
			Description:     "Do a small downward head gesture. Use sparingly for thinking, softness, or shyness.",
			ParamChoices:    "now",
			Action:          ActionHeadDown,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "liftUp",
			Description:     "Raise the lift arms briefly as a small gesture. Do not use for repeated motion.",
			ParamChoices:    "now",
			Action:          ActionLiftUp,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "liftDown",
			Description:     "Lower the lift arms briefly as a small gesture. Do not use for repeated motion.",
			ParamChoices:    "now",
			Action:          ActionLiftDown,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "nod",
			Description:     "Do a small nod. Prefer this over raw headUp or headDown for acknowledgement, greeting, or agreement.",
			ParamChoices:    "now",
			Action:          ActionNod,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "lookDownShy",
			Description:     "Briefly look down and recover. Use for shy, gentle, embarrassed, or thoughtful moments.",
			ParamChoices:    "now",
			Action:          ActionLookDownShy,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "raiseArmsHappy",
			Description:     "Raise the lift arms in a cheerful gesture. Use for excitement, pride, or celebration.",
			ParamChoices:    "now",
			Action:          ActionRaiseArmsHappy,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "goCharge",
			Description:     "Actually go back to the charger and start charging. Use this whenever the user asks the robot to go home, dock, or charge.",
			ParamChoices:    "now",
			Action:          ActionGoCharge,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "takePhoto",
			Description:     "Take a real photo and save it to the robot photo gallery. Use for actual photo-taking, not scene analysis.",
			ParamChoices:    "now",
			Action:          ActionTakePhoto,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "celebrateFireworks",
			Description:     "Play the fireworks celebration behavior. Use for celebration, new year, party, or strong congratulations.",
			ParamChoices:    "now",
			Action:          ActionCelebrateFireworks,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "backAway",
			Description:     "Back away a short distance. Use when the user asks the robot to move back or give space.",
			ParamChoices:    "now",
			Action:          ActionBackAway,
			SupportedModels: []string{"all"},
		},
		{
			Name:            "newVoiceRequest",
			Description:     "Start listening for the next voice turn. Use at the end of a reply only when conversation mode allows follow-up input.",
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
