package intent

import (
	"strconv"
	"strings"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/support"
)

// parseTimerSecondsFromSpeech 将口语中的时长表达规范成秒数字符串。
func parseTimerSecondsFromSpeech(speechText string) string {
	timerSecs := support.Words2Num(speechText)
	logger.Println("Seconds parsed from speech: " + timerSecs)
	return timerSecs
}

// parseTimerSecondsFromSlots 将 slot 中的数字和单位换算成秒数字符串。
func parseTimerSecondsFromSlots(slots map[string]string) string {
	timerSecs, err := strconv.Atoi(slots["num"])
	if err != nil {
		logger.Println(err)
	}
	if slots["num"] != "" && slots["unit"] != "" {
		if strings.Contains(slots["unit"], "minute") {
			timerSecs *= 60
		} else if strings.Contains(slots["unit"], "hour") {
			timerSecs *= 60 * 60
		}
	}
	logger.Println("Seconds parsed from speech: " + strconv.Itoa(timerSecs))
	return strconv.Itoa(timerSecs)
}
