package vision

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
	memorypkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/memory"
	robotpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/robot"
)

const recentFaceTTL = 45 * time.Second
const autoGreetCooldown = 20 * time.Second
const faceReappearanceWindow = 10 * time.Second
const repeatedFaceLogInterval = 15 * time.Second
const repeatedDecisionLogInterval = 8 * time.Second
const activeGreetingHoldWindow = 12 * time.Second

type observedFaceState struct {
	Name        string
	FaceID      int32
	LastSeenAt  time.Time
	FirstSeenAt time.Time
	Reappeared  bool
}

type pendingGreetingState struct {
	Name     string
	QueuedAt time.Time
	Watching bool
}

type activeGreetingState struct {
	Name      string
	StartedAt time.Time
}

var (
	faceWatcherMu     sync.Mutex
	faceWatchers      = map[string]bool{}
	observedFacesMu   sync.Mutex
	observedFaces     = map[string]observedFaceState{}
	autoGreetMu       sync.Mutex
	lastAutoGreets    = map[string]time.Time{}
	pendingGreetMu    sync.Mutex
	pendingGreets     = map[string]pendingGreetingState{}
	activeGreetMu     sync.Mutex
	activeGreets      = map[string]activeGreetingState{}
	faceLogMu         sync.Mutex
	lastFaceEventLogs = map[string]time.Time{}
	lastFaceMapLogs   = map[string]time.Time{}
	decisionLogMu     sync.Mutex
	lastDecisionLogs  = map[string]time.Time{}
	ownerGreetingFunc func(esn, name string) error
	knownGreetingFunc func(esn, name string) error
)

// BuildPromptContext 返回最近一次识别到的人脸上下文，供 LLM 判断当前面对的是谁。
func BuildPromptContext(esn string) string {
	if !vars.APIConfig.Vision.EnableFaceContext {
		return ""
	}
	ensureFaceWatcher(esn)

	observedFacesMu.Lock()
	defer observedFacesMu.Unlock()

	state, ok := observedFaces[esn]
	if !ok || state.LastSeenAt.IsZero() || time.Since(state.LastSeenAt) > recentFaceTTL {
		return ""
	}

	if strings.TrimSpace(state.Name) != "" {
		return buildIdentityPrompt(esn, state)
	}

	return "Current visual context: The robot recently saw a person in front of it, but could not match a saved name."
}

func buildIdentityPrompt(esn string, state observedFaceState) string {
	name := strings.TrimSpace(state.Name)
	if name == "" {
		return ""
	}

	profile := memorypkg.LoadProfile(esn)
	switch {
	case samePerson(name, profile.OwnerName):
		return fmt.Sprintf("Current visual context: The robot recently recognized %s in front of it. This face matches the saved owner identity.", name)
	case samePerson(name, profile.UserName):
		return fmt.Sprintf("Current visual context: The robot recently recognized %s in front of it. This face matches the saved user identity.", name)
	case samePerson(name, profile.Nickname):
		return fmt.Sprintf("Current visual context: The robot recently recognized %s in front of it. This may be the person who prefers to be addressed as %s.", name, profile.Nickname)
	default:
		return "Current visual context: The robot recently recognized a face named " + name + " standing in front of it."
	}
}

func identityKind(profile memorypkg.UserProfile, name string) string {
	switch {
	case samePerson(name, profile.OwnerName):
		return "owner"
	case samePerson(name, profile.UserName):
		return "user"
	case samePerson(name, profile.Nickname):
		return "nickname"
	default:
		return "known_face"
	}
}

func identityKindForESN(esn, name string) string {
	return identityKind(memorypkg.LoadProfile(esn), name)
}

// IdentityKindForGreeting 返回当前识别名字对应的长期身份类型。
func IdentityKindForGreeting(esn, name string) string {
	return identityKindForESN(esn, name)
}

func samePerson(faceName, profileName string) bool {
	faceName = strings.TrimSpace(strings.ToLower(faceName))
	profileName = strings.TrimSpace(strings.ToLower(profileName))
	return faceName != "" && profileName != "" && faceName == profileName
}

// ensureFaceWatcher 为每个机器人只启动一个后台观察器，避免重复占用事件流。
func ensureFaceWatcher(esn string) {
	esn = strings.TrimSpace(strings.ToLower(esn))
	if esn == "" {
		return
	}

	faceWatcherMu.Lock()
	defer faceWatcherMu.Unlock()
	if faceWatchers[esn] {
		return
	}
	faceWatchers[esn] = true

	go watchFaces(esn)
}

func watchFaces(esn string) {
	for {
		if err := watchFacesOnce(esn); err != nil {
			if errors.Is(err, context.Canceled) {
				// 视觉模式被机器人自动关闭时，后台观察器会主动重连；这是预期路径，不当成错误刷日志。
				logger.Println("Face watcher for " + esn + " restarting after vision mode reset")
			} else {
				logger.Println("Face watcher for " + esn + " stopped, retrying: " + err.Error())
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func watchFacesOnce(esn string) error {
	robot, err := vector.NewWP(esn)
	if err != nil {
		return err
	}

	_, err = robot.Conn.EnableFaceDetection(context.Background(), &vectorpb.EnableFaceDetectionRequest{
		Enable: true,
	})
	if err != nil {
		return err
	}

	stream, err := robot.Conn.EventStream(
		context.Background(),
		&vectorpb.EventRequest{
			ListType: &vectorpb.EventRequest_WhiteList{
				WhiteList: &vectorpb.FilterList{
					List: []string{"robot_observed_face", "robot_changed_observed_face_id", "vision_modes_auto_disabled"},
				},
			},
		},
	)
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}

		switch resp.Event.EventType.(type) {
		case *vectorpb.Event_RobotObservedFace:
			face := resp.Event.GetRobotObservedFace()
			if face == nil {
				continue
			}
			maybeLogFaceEvent(esn, face.GetFaceId(), strings.TrimSpace(face.GetName()))
			updateObservedFace(esn, face.GetFaceId(), face.GetName())
		case *vectorpb.Event_VisionModesAutoDisabled:
			// 某些情况下机器人会自动关掉视觉能力，这里立刻退出并走外层重连重开。
			return context.Canceled
		}
	}
}

func updateObservedFace(esn string, faceID int32, name string) {
	name = strings.TrimSpace(name)

	observedFacesMu.Lock()
	prev := observedFaces[esn]
	now := time.Now()
	firstSeenAt := now
	reappeared := false
	if strings.EqualFold(strings.TrimSpace(prev.Name), name) && !prev.LastSeenAt.IsZero() && now.Sub(prev.LastSeenAt) < faceReappearanceWindow {
		firstSeenAt = prev.FirstSeenAt
		if firstSeenAt.IsZero() {
			firstSeenAt = prev.LastSeenAt
		}
	} else if strings.EqualFold(strings.TrimSpace(prev.Name), name) && !prev.LastSeenAt.IsZero() && now.Sub(prev.LastSeenAt) >= faceReappearanceWindow {
		reappeared = true
	}
	observedFaces[esn] = observedFaceState{
		Name:        name,
		FaceID:      faceID,
		LastSeenAt:  now,
		FirstSeenAt: firstSeenAt,
		Reappeared:  reappeared,
	}
	observedFacesMu.Unlock()

	if name == "" {
		logDecisionOnce(esn, fmt.Sprintf("unknown-face:%d", faceID), fmt.Sprintf("Face watcher for %s saw face id=%d but did not match a saved name", esn, faceID))
		return
	}

	profile := memorypkg.LoadProfile(esn)
	maybeLogFaceMapping(esn, faceID, name, identityKind(profile, name))

	if name != "" {
		maybeAutoGreetKnownFace(esn, name)
	}
}

// maybeAutoGreetKnownFace 对已命名人脸做低频自动问候，避免重复路过时一直说话。
func maybeAutoGreetKnownFace(esn, name string) {
	if !vars.APIConfig.Vision.AutoGreetKnownFaces {
		logDecisionOnce(esn, "feature-disabled:"+strings.ToLower(name), fmt.Sprintf("Auto face greeting skipped for %s name=%q: feature disabled", esn, name))
		return
	}
	if isGreetingActiveForFace(esn, name) {
		logDecisionOnce(esn, "active-same-face:"+strings.ToLower(name), fmt.Sprintf("Auto face greeting skipped for %s name=%q: same-face greeting already active", esn, name))
		return
	}
	if reason := robotpkg.PassiveGreetingBlockReason(esn); reason != "" {
		logDecisionOnce(esn, "delayed:"+strings.ToLower(name)+":"+reason, fmt.Sprintf("Auto face greeting delayed for %s name=%q: %s", esn, name, reason))
		queuePendingAutoGreeting(esn, name)
		return
	}
	if !shouldAutoGreet(esn, name) {
		return
	}
	startAutoGreeting(esn, name)
}

func shouldAutoGreet(esn, name string) bool {
	key := strings.ToLower(strings.TrimSpace(esn)) + "::" + strings.ToLower(strings.TrimSpace(name))
	if key == "::" || strings.HasSuffix(key, "::") {
		logDecisionOnce(esn, "invalid-key:"+strings.ToLower(name), fmt.Sprintf("Auto face greeting skipped for %s name=%q: invalid identity key", esn, name))
		return false
	}

	isReappeared := hasFaceReappeared(esn, name)

	autoGreetMu.Lock()
	defer autoGreetMu.Unlock()

	lastAt := lastAutoGreets[key]
	if !lastAt.IsZero() && time.Since(lastAt) < autoGreetCooldown {
		if !isReappeared {
			logDecisionOnce(esn, "cooldown:"+strings.ToLower(name), fmt.Sprintf("Auto face greeting skipped for %s name=%q: cooldown active", esn, name))
			return false
		}
		logDecisionOnce(esn, "reappear-allow:"+strings.ToLower(name), fmt.Sprintf("Auto face greeting allowed for %s name=%q: face reappeared after cooldown gap", esn, name))
	}
	if isReappeared {
		logDecisionOnce(esn, "reappeared:"+strings.ToLower(name), fmt.Sprintf("Auto face greeting treating %s name=%q as reappeared", esn, name))
	}
	if !lastAt.IsZero() && time.Since(lastAt) < 2*time.Second {
		logDecisionOnce(esn, "duplicate:"+strings.ToLower(name), fmt.Sprintf("Auto face greeting skipped for %s name=%q: duplicate trigger guard", esn, name))
		return false
	}
	lastAutoGreets[key] = time.Now()
	return true
}

func maybeLogFaceEvent(esn string, faceID int32, name string) {
	key := fmt.Sprintf("%s:%d:%s", esn, faceID, strings.ToLower(strings.TrimSpace(name)))

	faceLogMu.Lock()
	defer faceLogMu.Unlock()

	lastAt := lastFaceEventLogs[key]
	if !lastAt.IsZero() && time.Since(lastAt) < repeatedFaceLogInterval {
		return
	}
	lastFaceEventLogs[key] = time.Now()
	logger.Println(fmt.Sprintf("Face watcher for %s observed face event: id=%d, name=%q", esn, faceID, name))
}

func maybeLogFaceMapping(esn string, faceID int32, name, kind string) {
	key := fmt.Sprintf("%s:%d:%s:%s", esn, faceID, strings.ToLower(strings.TrimSpace(name)), kind)

	faceLogMu.Lock()
	defer faceLogMu.Unlock()

	lastAt := lastFaceMapLogs[key]
	if !lastAt.IsZero() && time.Since(lastAt) < repeatedFaceLogInterval {
		return
	}
	lastFaceMapLogs[key] = time.Now()
	logger.Println(fmt.Sprintf("Face watcher for %s mapped face id=%d name=%q as %s", esn, faceID, name, kind))
}

func logDecisionOnce(esn, key, msg string) {
	fullKey := esn + "::" + key

	decisionLogMu.Lock()
	defer decisionLogMu.Unlock()

	lastAt := lastDecisionLogs[fullKey]
	if !lastAt.IsZero() && time.Since(lastAt) < repeatedDecisionLogInterval {
		return
	}
	lastDecisionLogs[fullKey] = time.Now()
	logger.Println(msg)
}

func speakAutoGreeting(esn, name string) error {
	markGreetingActive(esn, name)
	defer clearGreetingActive(esn, name)

	profile := memorypkg.LoadProfile(esn)
	kind := identityKind(profile, name)
	if kind == "owner" && ownerGreetingFunc != nil {
		logger.Println(fmt.Sprintf("Auto face greeting branch for %s name=%q: owner -> llm greeting", esn, name))
		return ownerGreetingFunc(esn, name)
	}
	if kind == "user" && knownGreetingFunc != nil {
		logger.Println(fmt.Sprintf("Auto face greeting branch for %s name=%q: user -> llm greeting", esn, name))
		return knownGreetingFunc(esn, name)
	}
	if kind == "nickname" && knownGreetingFunc != nil {
		logger.Println(fmt.Sprintf("Auto face greeting branch for %s name=%q: nickname -> llm greeting", esn, name))
		return knownGreetingFunc(esn, name)
	}
	text := buildAutoGreetingText(profile, name)
	if strings.TrimSpace(text) == "" {
		logger.Println(fmt.Sprintf("Auto face greeting branch for %s name=%q: %s -> empty template", esn, name, kind))
		return nil
	}
	logger.Println(fmt.Sprintf("Auto face greeting speaking text for %s name=%q: %s", esn, name, text))
	logger.Println(fmt.Sprintf("Auto face greeting branch for %s name=%q: %s -> template greeting", esn, name, kind))
	return robotpkg.KGSim(esn, text)
}

// SetOwnerGreetingFunc 注入“识别到主人后用 LLM 生成问候”的实现，避免 vision 直接依赖 llm 包。
func SetOwnerGreetingFunc(fn func(esn, name string) error) {
	ownerGreetingFunc = fn
}

// SetKnownGreetingFunc 注入“识别到普通已命名人脸后用 LLM 生成问候”的实现。
func SetKnownGreetingFunc(fn func(esn, name string) error) {
	knownGreetingFunc = fn
}

func startAutoGreeting(esn, name string) {
	go func() {
		logger.Println(fmt.Sprintf("Auto face greeting starting for %s name=%q", esn, name))
		if err := speakAutoGreeting(esn, name); err != nil {
			logger.Println("Auto face greeting failed for " + esn + ": " + err.Error())
			return
		}
		logger.Println(fmt.Sprintf("Auto face greeting finished for %s name=%q", esn, name))
	}()
}

func queuePendingAutoGreeting(esn, name string) {
	pendingGreetMu.Lock()
	state := pendingGreets[esn]
	state.Name = name
	state.QueuedAt = time.Now()
	shouldStartWatcher := !state.Watching
	state.Watching = true
	pendingGreets[esn] = state
	pendingGreetMu.Unlock()

	if !shouldStartWatcher {
		return
	}

	go flushPendingAutoGreeting(esn)
}

func flushPendingAutoGreeting(esn string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pendingGreetMu.Lock()
		state, ok := pendingGreets[esn]
		if !ok {
			pendingGreetMu.Unlock()
			return
		}

		if !vars.APIConfig.Vision.AutoGreetKnownFaces {
			delete(pendingGreets, esn)
			pendingGreetMu.Unlock()
			logger.Println(fmt.Sprintf("Pending auto face greeting cleared for %s: feature disabled", esn))
			return
		}

		if time.Since(state.QueuedAt) > recentFaceTTL {
			delete(pendingGreets, esn)
			pendingGreetMu.Unlock()
			logger.Println(fmt.Sprintf("Pending auto face greeting expired for %s name=%q", esn, state.Name))
			return
		}

		if reason := robotpkg.PassiveGreetingBlockReason(esn); reason != "" {
			pendingGreetMu.Unlock()
			continue
		}

		name := state.Name
		delete(pendingGreets, esn)
		pendingGreetMu.Unlock()

		if isGreetingActiveForFace(esn, name) {
			logger.Println(fmt.Sprintf("Pending auto face greeting dropped for %s name=%q: same-face greeting still active", esn, name))
			return
		}

		if !isFaceStillPresent(esn, name) {
			logger.Println(fmt.Sprintf("Pending auto face greeting dropped for %s name=%q: face no longer present", esn, name))
			return
		}

		if !shouldAutoGreet(esn, name) {
			logger.Println(fmt.Sprintf("Pending auto face greeting skipped for %s name=%q after unblock", esn, name))
			return
		}

		logger.Println(fmt.Sprintf("Pending auto face greeting released for %s name=%q", esn, name))
		startAutoGreeting(esn, name)
		return
	}
}

func isFaceStillPresent(esn, name string) bool {
	observedFacesMu.Lock()
	defer observedFacesMu.Unlock()

	state, ok := observedFaces[esn]
	if !ok {
		return false
	}
	if time.Since(state.LastSeenAt) > recentFaceTTL {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(state.Name), strings.TrimSpace(name))
}

func hasFaceReappeared(esn, name string) bool {
	observedFacesMu.Lock()
	defer observedFacesMu.Unlock()

	state, ok := observedFaces[esn]
	if !ok {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(state.Name), strings.TrimSpace(name)) {
		return false
	}
	return state.Reappeared
}

func isGreetingActiveForFace(esn, name string) bool {
	activeGreetMu.Lock()
	defer activeGreetMu.Unlock()

	state, ok := activeGreets[esn]
	if !ok {
		return false
	}
	if time.Since(state.StartedAt) > activeGreetingHoldWindow {
		delete(activeGreets, esn)
		return false
	}
	return strings.EqualFold(strings.TrimSpace(state.Name), strings.TrimSpace(name))
}

func markGreetingActive(esn, name string) {
	activeGreetMu.Lock()
	defer activeGreetMu.Unlock()

	activeGreets[esn] = activeGreetingState{
		Name:      strings.TrimSpace(name),
		StartedAt: time.Now(),
	}
}

func clearGreetingActive(esn, name string) {
	activeGreetMu.Lock()
	defer activeGreetMu.Unlock()

	state, ok := activeGreets[esn]
	if !ok {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(state.Name), strings.TrimSpace(name)) {
		return
	}
	delete(activeGreets, esn)
}

func buildAutoGreetingText(profile memorypkg.UserProfile, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	switch identityKind(profile, name) {
	case "owner":
		if strings.Contains(profile.PreferredGreeting, "Japanese") || strings.Contains(profile.PreferredGreeting, "日语") {
			return "主人，こんにちは。欢迎回来。"
		}
		if strings.Contains(profile.PreferredGreeting, "master") || strings.Contains(profile.PreferredGreeting, "主人") {
			return "主人，你好呀，我看到你啦。"
		}
		return name + "，你好呀，我看到你回来啦。"
	case "user":
		return name + "，欢迎回来呀，见到你真开心。"
	case "nickname":
		return name + "，我一眼就认出你啦。"
	default:
		return name + "，你好呀，又见到你啦。"
	}
}
