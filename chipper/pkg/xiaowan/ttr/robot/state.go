package robot

import (
	"strings"
	"sync"
	"time"
)

const passiveGreetingQuietPeriod = 3 * time.Second

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
	if state.ActiveCount > 0 {
		return "foreground interaction active"
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
