package llm

import (
	"strings"

	"github.com/sashabaranov/go-openai"
)

func detectForcedNativeTool(transcribedText string) string {
	text := normalizeDirectActionText(transcribedText)
	if text == "" {
		return ""
	}

	if isDirectFireworksRequest(text) {
		return "celebrate_fireworks"
	}
	if isDirectChargeRequest(text) {
		return "go_charge"
	}
	if isDirectPhotoRequest(text) {
		return "take_photo"
	}
	if isDirectBackAwayRequest(text) {
		return "back_away"
	}
	return ""
}

func buildForcedToolSystemPrompt(toolName string) string {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return ""
	}
	return strings.Join([]string{
		"Direct action guidance for this turn:",
		"- The user made a clear real-world robot action request.",
		"- You must call the native tool `" + toolName + "` for this turn.",
		"- Do not output legacy command text, placeholders, or brace markup.",
		"- You may include a short spoken acknowledgement, but the tool call is required.",
	}, "\n")
}

func forceNativeToolChoice(req openai.ChatCompletionRequest, toolName string) openai.ChatCompletionRequest {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return req
	}
	req.ToolChoice = openai.ToolChoice{
		Type: openai.ToolTypeFunction,
		Function: openai.ToolFunction{
			Name: toolName,
		},
	}
	req.ParallelToolCalls = false
	return req
}

func normalizeDirectActionText(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func isDirectFireworksRequest(text string) bool {
	keywords := []string{
		"放烟花", "烟花", "庆祝一下", "庆祝下", "放个烟花",
		"fireworks", "celebrate",
	}
	return containsAny(text, keywords) && !containsAny(text, []string{"为什么", "怎么", "能不能", "可以吗", "会不会"})
}

func isDirectChargeRequest(text string) bool {
	keywords := []string{
		"回家充电", "回去充电", "去充电", "回充电座", "回家去充电", "回家去吧", "充电去吧",
		"go charge", "go home", "go to charger", "dock",
	}
	return containsAny(text, keywords)
}

func isDirectPhotoRequest(text string) bool {
	keywords := []string{
		"拍张照", "拍照", "拍个照", "拍一张", "给我拍", "照一张",
		"take a photo", "take photo", "snap a photo", "take a picture",
	}
	if !containsAny(text, keywords) {
		return false
	}
	return !containsAny(text, []string{"看看", "你看到", "分析", "识别", "what do you see"})
}

func isDirectBackAwayRequest(text string) bool {
	keywords := []string{
		"后退", "退后", "往后退", "往后一点", "离远一点", "让开点",
		"back away", "move back", "step back",
	}
	return containsAny(text, keywords)
}

func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
