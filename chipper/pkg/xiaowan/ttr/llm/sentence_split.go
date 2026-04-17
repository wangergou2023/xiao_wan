package llm

import (
	"strings"
	"unicode/utf8"
)

var sentenceSeparators = []string{"...", "。", "？", "！", ".'", ".\"", ".", "?", "!"}

const (
	speechChunkMinRunes  = 18
	speechChunkSoftRunes = 32
	speechChunkHardRunes = 48
)

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

// splitFirstSpeechChunk 先按完整句号切分；如果模型一直不打标点，就按较自然的停顿点分块，
// 避免整段内容一直等到 EOF 才开始说话，或者因为没有句号导致后续不播报。
func splitFirstSpeechChunk(input string) (string, string, bool) {
	if sentence, remainder, ok := splitFirstSentence(input); ok {
		return sentence, remainder, true
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", false
	}

	if boundary, ok := findSpeechChunkBoundary(trimmed); ok {
		return strings.TrimSpace(trimmed[:boundary]), strings.TrimSpace(trimmed[boundary:]), true
	}
	return "", trimmed, false
}

func findSpeechChunkBoundary(input string) (int, bool) {
	inCommand := false
	runeCount := 0
	bestPauseBoundary := -1
	hardBoundary := -1

	for i := 0; i < len(input); {
		switch {
		case strings.HasPrefix(input[i:], "{{"):
			inCommand = true
			i += len("{{")
			continue
		case strings.HasPrefix(input[i:], "}}"):
			inCommand = false
			i += len("}}")
			if runeCount >= speechChunkMinRunes && runeCount <= speechChunkSoftRunes {
				bestPauseBoundary = i
			}
			continue
		}

		r, size := utf8.DecodeRuneInString(input[i:])
		runeCount++
		if !inCommand {
			if runeCount >= speechChunkMinRunes && runeCount <= speechChunkSoftRunes && isSpeechPauseRune(r) {
				bestPauseBoundary = i + size
			}
			if runeCount >= speechChunkHardRunes {
				hardBoundary = i + size
				break
			}
		}
		i += size
	}

	if bestPauseBoundary > 0 {
		return bestPauseBoundary, true
	}
	if hardBoundary > 0 {
		return hardBoundary, true
	}
	return 0, false
}

func isSpeechPauseRune(r rune) bool {
	switch r {
	case '，', '、', ',', '；', ';', '：', ':', ' ':
		return true
	default:
		return false
	}
}

func clipDebugString(input string, max int) string {
	trimmed := strings.TrimSpace(input)
	runes := []rune(trimmed)
	if len(runes) <= max {
		return trimmed
	}
	return string(runes[:max]) + "..."
}
