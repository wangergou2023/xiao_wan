package robot

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
)

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
