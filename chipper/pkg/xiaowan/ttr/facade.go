package ttr

import (
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vtt"
	intentpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/intent"
	llmpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/llm"
	robotpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/robot"
)

// IntentPass 兼容旧调用方，对外暴露 intent 子模块的响应下发能力。
func IntentPass(req interface{}, intentThing string, speechText string, intentParams map[string]string, isParam bool) (interface{}, error) {
	return intentpkg.IntentPass(req, intentThing, speechText, intentParams, isParam)
}

// ProcessTextAll 兼容旧调用方，对外暴露规则 intent 匹配能力。
func ProcessTextAll(req interface{}, voiceText string, intents []vars.JsonIntent, isOpus bool) bool {
	return intentpkg.ProcessTextAll(req, voiceText, intents, isOpus)
}

// KnowledgeGraphResponseIG 兼容旧调用方，对外暴露 KG 响应封装能力。
func KnowledgeGraphResponseIG(req *vtt.IntentGraphRequest, spokenText string, queryText string) error {
	return intentpkg.KnowledgeGraphResponseIG(req, spokenText, queryText)
}

// ParamCheckerSlotsEnUS 兼容旧调用方，对外暴露英文 slot 参数解析能力。
func ParamCheckerSlotsEnUS(req interface{}, intent string, slots map[string]string, isOpus bool, botSerial string) {
	intentpkg.ParamCheckerSlotsEnUS(req, intent, slots, isOpus, botSerial)
}

// StreamingKGSim 兼容旧调用方，对外暴露 LLM 流式回复能力。
func StreamingKGSim(req interface{}, esn string, transcribedText string, isKG bool) (string, error) {
	return llmpkg.StreamingKGSim(req, esn, transcribedText, isKG)
}

// KGSim 兼容旧调用方，对外暴露机器人直接播报能力。
func KGSim(esn string, textToSay string) error {
	return robotpkg.KGSim(esn, textToSay)
}
