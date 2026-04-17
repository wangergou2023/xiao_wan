package llm

import (
	"strings"
	"unicode"
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
	input = normalizeSpeechChunkInput(input)
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
	bestBreakBoundary := -1
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
			if runeCount >= speechChunkMinRunes && runeCount <= speechChunkHardRunes {
				if boundary, ok := matchStructuredBoundary(input, i); ok {
					bestBreakBoundary = boundary
				}
			}
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

	if bestBreakBoundary > 0 {
		return bestBreakBoundary, true
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
	case '，', '、', ',', '；', ';', '：', ':':
		return true
	default:
		return false
	}
}

func normalizeSpeechChunkInput(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	var b strings.Builder
	var lastSpace bool
	var newlineCount int

	for _, r := range trimmed {
		switch {
		case r == '\r':
			continue
		case r == '\n':
			if newlineCount >= 2 {
				continue
			}
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
				b.WriteRune('\n')
			}
			newlineCount++
			lastSpace = false
		case unicode.IsSpace(r):
			if lastSpace || newlineCount > 0 {
				continue
			}
			b.WriteRune(' ')
			lastSpace = true
		default:
			b.WriteRune(r)
			lastSpace = false
			newlineCount = 0
		}
	}

	return strings.TrimSpace(b.String())
}

func matchStructuredBoundary(input string, idx int) (int, bool) {
	rest := input[idx:]

	if strings.HasPrefix(rest, "\n\n") {
		return idx, true
	}
	if strings.HasPrefix(rest, "\n") {
		next := strings.TrimLeft(rest, "\n")
		if hasListPrefix(next) {
			return idx, true
		}
	}
	if strings.HasPrefix(rest, " ") {
		next := strings.TrimLeft(rest, " ")
		if hasListPrefix(next) {
			return idx, true
		}
	}
	return 0, false
}

func hasListPrefix(input string) bool {
	prefixes := []string{
		"第一", "第二", "第三", "第四", "第五", "第六", "第七", "第八", "第九", "第十",
		"1.", "2.", "3.", "4.", "5.", "1、", "2、", "3、", "4、", "5、",
		"一是", "二是", "三是", "首先", "其次", "最后",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(input, prefix) {
			return true
		}
	}
	return false
}

func clipDebugString(input string, max int) string {
	trimmed := strings.TrimSpace(input)
	runes := []rune(trimmed)
	if len(runes) <= max {
		return trimmed
	}
	return string(runes[:max]) + "..."
}
