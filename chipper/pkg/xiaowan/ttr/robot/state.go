package robot

import (
	"strings"
	"sync"
	"time"
)

const passiveGreetingQuietPeriod = 12 * time.Second

type robotActivityState struct {
	ActiveCount   int
	LastStartedAt time.Time
	LastEndedAt   time.Time
}

var (
	robotActivityMu     sync.Mutex
	robotActivityStates = map[string]robotActivityState{}
)

// BeginForegroundActivity 标记机器人进入主动交互阶段，返回结束函数供调用方配对释放。
func BeginForegroundActivity(esn string) func() {
	esn = normalizeESN(esn)
	if esn == "" {
		return func() {}
	}

	robotActivityMu.Lock()
	state := robotActivityStates[esn]
	state.ActiveCount++
	state.LastStartedAt = time.Now()
	robotActivityStates[esn] = state
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
			state.LastEndedAt = time.Now()
			robotActivityStates[esn] = state
		})
	}
}

// IsBusyForPassiveGreeting 判断机器人是否正忙，或刚结束一轮交互，不适合插入被动问候。
func IsBusyForPassiveGreeting(esn string) bool {
	esn = normalizeESN(esn)
	if esn == "" {
		return false
	}

	robotActivityMu.Lock()
	defer robotActivityMu.Unlock()

	state := robotActivityStates[esn]
	if state.ActiveCount > 0 {
		return true
	}
	if !state.LastEndedAt.IsZero() && time.Since(state.LastEndedAt) < passiveGreetingQuietPeriod {
		return true
	}
	return false
}

func normalizeESN(esn string) string {
	return strings.TrimSpace(strings.ToLower(esn))
}
