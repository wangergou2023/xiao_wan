package robot

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
)

const passiveGreetingQuietPeriod = 3 * time.Second
const staleForegroundActivityTimeout = 30 * time.Second

type robotActivityState struct {
	ActiveCount   int
	LastStartedAt time.Time
	LastEndedAt   time.Time
	ActiveSources map[string]int
}

var (
	robotActivityMu     sync.Mutex
	robotActivityStates = map[string]robotActivityState{}
)

// BeginForegroundActivity 标记机器人进入主动交互阶段，返回结束函数供调用方配对释放。
func BeginForegroundActivity(esn, source string) func() {
	esn = normalizeESN(esn)
	source = strings.TrimSpace(source)
	if source == "" {
		source = "unknown"
	}
	if esn == "" {
		return func() {}
	}

	robotActivityMu.Lock()
	state := robotActivityStates[esn]
	state.ActiveCount++
	state.LastStartedAt = time.Now()
	if state.ActiveSources == nil {
		state.ActiveSources = map[string]int{}
	}
	state.ActiveSources[source]++
	robotActivityStates[esn] = state
	logger.Println(fmt.Sprintf("Foreground activity begin for %s source=%s active_count=%d sources=%s", esn, source, state.ActiveCount, formatActiveSources(state.ActiveSources)))
	robotActivityMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			robotActivityMu.Lock()
			defer robotActivityMu.Unlock()
			state := robotActivityStates[esn]
			if state.ActiveCount > 0 {
				state.ActiveCount--
			}
			if state.ActiveSources != nil {
				if state.ActiveSources[source] > 1 {
					state.ActiveSources[source]--
				} else {
					delete(state.ActiveSources, source)
				}
			}
			state.LastEndedAt = time.Now()
			robotActivityStates[esn] = state
			logger.Println(fmt.Sprintf("Foreground activity end for %s source=%s active_count=%d sources=%s", esn, source, state.ActiveCount, formatActiveSources(state.ActiveSources)))
		})
	}
}

// PassiveGreetingBlockReason 返回为什么当前不适合插入被动问候。
// 这里只拦真正的前台交互，避免把机器人桌面闲逛也当成“忙”。
func PassiveGreetingBlockReason(esn string) string {
	esn = normalizeESN(esn)
	if esn == "" {
		return ""
	}

	robotActivityMu.Lock()
	defer robotActivityMu.Unlock()

	state := robotActivityStates[esn]
	if state.ActiveCount > 0 && !state.LastStartedAt.IsZero() && time.Since(state.LastStartedAt) > staleForegroundActivityTimeout {
		logger.Println(fmt.Sprintf(
			"Foreground activity stale auto-release for %s after %s sources=%s",
			esn,
			time.Since(state.LastStartedAt).Round(time.Second),
			formatActiveSources(state.ActiveSources),
		))
		state.ActiveCount = 0
		state.ActiveSources = map[string]int{}
		state.LastEndedAt = time.Now()
		robotActivityStates[esn] = state
	}
	if state.ActiveCount > 0 {
		return "foreground interaction active: " + formatActiveSources(state.ActiveSources)
	}
	if !state.LastEndedAt.IsZero() && time.Since(state.LastEndedAt) < passiveGreetingQuietPeriod {
		return "recent foreground interaction cooldown"
	}
	return ""
}

// IsBusyForPassiveGreeting 判断机器人当前是否需要暂缓被动问候。
func IsBusyForPassiveGreeting(esn string) bool {
	return PassiveGreetingBlockReason(esn) != ""
}

func normalizeESN(esn string) string {
	return strings.TrimSpace(strings.ToLower(esn))
}

func formatActiveSources(sources map[string]int) string {
	if len(sources) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(sources))
	for source, count := range sources {
		parts = append(parts, fmt.Sprintf("%s=%d", source, count))
	}
	return strings.Join(parts, ",")
}
