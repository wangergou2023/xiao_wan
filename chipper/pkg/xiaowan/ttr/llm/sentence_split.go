package llm

import "strings"

var sentenceSeparators = []string{"...", "。", "？", "！", ".'", ".\"", ".", "?", "!"}

func splitFirstSentence(input string) (string, string, bool) {
	trimmed := strings.TrimSpace(input)
	for _, sep := range sentenceSeparators {
		if idx := strings.Index(trimmed, sep); idx >= 0 {
			end := idx + len(sep)
			return strings.TrimSpace(trimmed[:end]), strings.TrimSpace(trimmed[end:]), true
		}
	}
	return "", trimmed, false
}

func clipDebugString(input string, max int) string {
	trimmed := strings.TrimSpace(input)
	runes := []rune(trimmed)
	if len(runes) <= max {
		return trimmed
	}
	return string(runes[:max]) + "..."
}
