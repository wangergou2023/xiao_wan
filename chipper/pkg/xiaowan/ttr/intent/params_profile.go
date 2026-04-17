package intent

import (
	"strings"

	lcztn "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/localization"
)

// parseGivenNameFromSpeech 从“for xxx”类短语中提取名字。
func parseGivenNameFromSpeech(speechText string) string {
	if !strings.Contains(speechText, lcztn.GetText(lcztn.STR_FOR)) {
		return ""
	}
	splitPhrase := strings.SplitAfter(speechText, lcztn.GetText(lcztn.STR_FOR))
	givenName := strings.TrimSpace(splitPhrase[1])
	if len(splitPhrase) == 3 {
		givenName += " " + strings.TrimSpace(splitPhrase[2])
	} else if len(splitPhrase) >= 4 {
		givenName += " " + strings.TrimSpace(splitPhrase[2]) + " " + strings.TrimSpace(splitPhrase[3])
	}
	return givenName
}
