package llm

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// isMn 用于过滤不适合机器人播报的组合字符，同时保留越南语必要音调。
func isMn(r rune) bool {
	keepMarks := []rune{'\u0300', '\u0301', '\u0303', '\u0309', '\u0323', '\u0302', '\u031B', '\u0306'}
	if unicode.Is(unicode.Mn, r) {
		for _, mark := range keepMarks {
			if r == mark {
				return false
			}
		}
		return true
	}
	return false
}

// stripLegacyCommandMarkup removes deprecated {{...}} command syntax entirely.
func stripLegacyCommandMarkup(input string) string {
	if !strings.Contains(input, "{{") {
		return input
	}
	var b strings.Builder
	for i := 0; i < len(input); {
		if strings.HasPrefix(input[i:], "{{") {
			end := strings.Index(input[i+2:], "}}")
			if end < 0 {
				break
			}
			i += 2 + end + 2
			continue
		}
		b.WriteByte(input[i])
		i++
	}
	return b.String()
}

// removeSpecialCharacters 把 LLM 输出规整成更适合 TTS 的纯文本。
func removeSpecialCharacters(str string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	result, _, _ := transform.String(t, str)
	re := regexp.MustCompile(`[&^*#@]`)
	result = removeEmojis(re.ReplaceAllString(result, ""))
	result = strings.ReplaceAll(result, "‘", "'")
	result = strings.ReplaceAll(result, "’", "'")
	result = strings.ReplaceAll(result, "“", "\"")
	result = strings.ReplaceAll(result, "”", "\"")
	result = strings.ReplaceAll(result, "—", "-")
	result = strings.ReplaceAll(result, "–", "-")
	result = strings.ReplaceAll(result, "…", "...")
	result = strings.ReplaceAll(result, "\u00A0", " ")
	result = strings.ReplaceAll(result, "•", "*")
	result = strings.ReplaceAll(result, "¼", "1/4")
	result = strings.ReplaceAll(result, "½", "1/2")
	result = strings.ReplaceAll(result, "¾", "3/4")
	result = strings.ReplaceAll(result, "×", "x")
	result = strings.ReplaceAll(result, "÷", "/")
	result = strings.ReplaceAll(result, "ç", "c")
	result = strings.ReplaceAll(result, "©", "(c)")
	result = strings.ReplaceAll(result, "®", "(r)")
	result = strings.ReplaceAll(result, "™", "(tm)")
	result = strings.ReplaceAll(result, "@", "(a)")
	result = strings.ReplaceAll(result, " AI ", " A. I. ")
	return stripLegacyCommandMarkup(result)
}

// removeEmojis 过滤掉机器人播报和动作协议中不稳定的 emoji 字符。
func removeEmojis(input string) string {
	re := regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]|[\x{1F900}-\x{1F9FF}]|[\x{1F004}]|[\x{1F0CF}]|[\x{1F18E}]|[\x{1F191}-\x{1F251}]|[\x{2B50}]`)
	return re.ReplaceAllString(input, "")
}
