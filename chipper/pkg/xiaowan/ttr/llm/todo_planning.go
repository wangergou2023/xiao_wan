package llm

import (
	"strings"

	workspacepkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/workspace"
)

func buildAutoTodoSystemPrompt(transcribedText string) string {
	text := strings.TrimSpace(transcribedText)
	if text == "" {
		return ""
	}

	state, err := workspacepkg.LoadTodoState()
	hasActivePlan := err == nil && state.ActivePlan != nil

	switch {
	case hasActivePlan && looksLikeResumeRequest(text):
		return strings.Join([]string{
			"Todo planning guidance for this turn:",
			"- There is already an active plan in `workspace/TODO.md`.",
			"- Before other multi-step work, call `todo_read` to inspect the current plan.",
			"- If the plan still matches the user's request, continue it and update progress with `todo_update` after each meaningful step.",
			"- If the user changed goals, replace the plan with `todo_write` before proceeding.",
		}, "\n")
	case hasActivePlan && looksLikeComplexTask(text):
		return strings.Join([]string{
			"Todo planning guidance for this turn:",
			"- There is already an active plan in `workspace/TODO.md`.",
			"- Reuse it when appropriate: call `todo_read`, then keep it current with `todo_update` while you work.",
			"- If the request starts a different multi-step task, replace the old plan with `todo_write` first.",
		}, "\n")
	case looksLikeComplexTask(text):
		return strings.Join([]string{
			"Todo planning guidance for this turn:",
			"- This request looks like multi-step work, so plan before acting.",
			"- Before memory, file, scheduling, or other multi-step tool work, call `todo_write` with a short goal and 2-6 concise steps.",
			"- Keep the plan current with `todo_update` as steps finish, and call `todo_clear` when the task is done or abandoned.",
			"- Skip todo planning only for simple chat or a single direct robot action.",
		}, "\n")
	default:
		return ""
	}
}

func looksLikeResumeRequest(input string) bool {
	text := normalizeTodoHeuristicText(input)
	if text == "" {
		return false
	}
	keywords := []string{
		"继续", "接着", "然后", "下一步", "恢复", "接下来", "上次", "刚才那个", "继续做",
		"continue", "resume", "pick up", "next step",
	}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func looksLikeComplexTask(input string) bool {
	text := normalizeTodoHeuristicText(input)
	if text == "" {
		return false
	}
	if looksLikeSimpleDirectAction(text) {
		return false
	}

	complexKeywords := []string{
		"todo", "计划", "规划", "清单", "列表", "安排", "步骤", "分步骤",
		"记住", "记下来", "长期记忆", "memory", "偏好", "资料", "档案",
		"读取", "读取文件", "查看文件", "修改文件", "写入", "保存到文件", "编辑文件",
		"workspace", "user.md", "memory.md", "identity.md", "soul.md",
		"提醒", "定时", "稍后", "每天", "每周", "cron", "schedule", "remind",
		"命令行", "shell", "command", "system", "执行命令",
		"分析", "整理", "总结", "归纳", "重构", "配置", "设置",
	}
	for _, kw := range complexKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}

	sequenceKeywords := []string{
		"然后", "再", "接着", "之后", "并且", "顺便", "先", "最后",
		"and then", "after that", "first", "next", "finally",
	}
	matches := 0
	for _, kw := range sequenceKeywords {
		if strings.Contains(text, kw) {
			matches++
		}
	}
	return matches >= 2
}

func looksLikeSimpleDirectAction(input string) bool {
	text := normalizeTodoHeuristicText(input)
	simpleKeywords := []string{
		"回家充电", "回去充电", "去充电", "充电去", "放个烟花", "烟花", "拍张照", "拍照",
		"后退", "退后", "抬手", "抬手臂", "抬起来", "点头", "低头", "抬头",
		"你好", "hello", "hi", "放烟花", "take a photo", "go charge", "back away",
	}
	for _, kw := range simpleKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func normalizeTodoHeuristicText(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}
