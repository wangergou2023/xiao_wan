package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	memorypkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/memory"
	robotpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/robot"
	visionpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/vision"
)

const passiveOwnerGreetingPrompt = "This is a passive visual greeting trigger. You just recognized your owner standing in front of you. Reply with exactly one short warm spoken sentence in Chinese. Do not use any command syntax like double braces. Do not narrate actions. Do not ask more than one short question."
const passiveKnownFaceGreetingPrompt = "This is a passive visual greeting trigger. You just recognized a familiar named person standing in front of you. Reply with exactly one short warm spoken sentence in Chinese. Do not use any command syntax like double braces. Do not narrate actions. Keep it friendly and brief."

func init() {
	visionpkg.SetOwnerGreetingFunc(GenerateOwnerGreeting)
	visionpkg.SetKnownGreetingFunc(GenerateKnownFaceGreeting)
}

// GenerateOwnerGreeting 在识别到主人且机器人空闲时，用 LLM 生成一句更自然的专属问候。
func GenerateOwnerGreeting(esn, name string) error {
	logger.Println(fmt.Sprintf("Owner face greeting requested for %s name=%q", esn, name))
	text, err := requestPassiveFaceGreeting(esn, name, "owner")
	if err != nil {
		return err
	}
	text = normalizeSpeechText(text)
	if text == "" {
		logger.Println(fmt.Sprintf("Owner face greeting for %s name=%q returned empty text", esn, name))
		return nil
	}
	logger.Println(fmt.Sprintf("Owner face greeting generated for %s name=%q: %q", esn, name, text))
	return robotpkg.KGSim(esn, text)
}

// GenerateKnownFaceGreeting 在识别到普通已命名人脸时，用 LLM 生成一句更自然的短问候。
func GenerateKnownFaceGreeting(esn, name string) error {
	kind := visionpkg.IdentityKindForGreeting(esn, name)
	logger.Println(fmt.Sprintf("Known face greeting requested for %s name=%q kind=%s", esn, name, kind))
	text, err := requestPassiveFaceGreeting(esn, name, kind)
	if err != nil {
		return err
	}
	text = normalizeSpeechText(text)
	if text == "" {
		logger.Println(fmt.Sprintf("Known face greeting for %s name=%q kind=%s returned empty text", esn, name, kind))
		return nil
	}
	logger.Println(fmt.Sprintf("Known face greeting generated for %s name=%q kind=%s: %q", esn, name, kind, text))
	return robotpkg.KGSim(esn, text)
}

func requestPassiveFaceGreeting(esn, name, kind string) (string, error) {
	basePrompt := strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt)
	if basePrompt == "" {
		basePrompt = "You are a helpful, animated robot called Vector. Keep the response concise yet informative."
	}

	systemPrompt := basePrompt + "\n\n" + passivePromptForKind(kind)
	if profilePrompt := strings.TrimSpace(memorypkg.BuildPromptContext(esn)); profilePrompt != "" {
		systemPrompt += "\n\nLong-term user profile:\n" + profilePrompt
	}

	model := getKnowledgeModel(false)
	logKnowledgeModel(model)

	req := openai.ChatCompletionRequest{
		Model:       model,
		MaxTokens:   120,
		Temperature: 0.8,
		TopP:        1,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: passiveGreetingUserPrompt(name, kind),
			},
		},
	}

	logger.Println(fmt.Sprintf("%s face greeting sending LLM request for %s name=%q", passiveGreetingLogLabel(kind), esn, name))
	resp, err := newKnowledgeClient().CreateChatCompletion(context.Background(), req)
	if err != nil {
		logger.Println(passiveGreetingLogLabel(kind) + " face greeting request failed: " + err.Error())
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}

func passivePromptForKind(kind string) string {
	if kind == "known_face" || kind == "user" || kind == "nickname" {
		return passiveKnownFaceGreetingPrompt
	}
	return passiveOwnerGreetingPrompt
}

func passiveGreetingUserPrompt(name, kind string) string {
	switch kind {
	case "user":
		return "You visually recognized the main user named " + name + " standing in front of you right now. Say one short friendly greeting out loud in Chinese."
	case "nickname":
		return "You visually recognized the person you usually address as " + name + " standing in front of you right now. Say one short affectionate greeting out loud in Chinese."
	case "known_face":
		return "You visually recognized a familiar named person called " + name + " standing in front of you right now. Say one short friendly greeting out loud in Chinese."
	}
	return "You visually recognized the owner named " + name + " standing in front of you right now. Say one short welcoming sentence out loud in Chinese."
}

func passiveGreetingLogLabel(kind string) string {
	if kind == "known_face" || kind == "user" || kind == "nickname" {
		return "Known"
	}
	return "Owner"
}
