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
const autoGreetCooldown = 3 * time.Minute

type observedFaceState struct {
	Name       string
	FaceID     int32
	LastSeenAt time.Time
}

var (
	faceWatcherMu   sync.Mutex
	faceWatchers    = map[string]bool{}
	observedFacesMu sync.Mutex
	observedFaces   = map[string]observedFaceState{}
	autoGreetMu     sync.Mutex
	lastAutoGreets  = map[string]time.Time{}
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
	observedFaces[esn] = observedFaceState{
		Name:       name,
		FaceID:     faceID,
		LastSeenAt: time.Now(),
	}
	observedFacesMu.Unlock()

	if name != "" {
		maybeAutoGreetKnownFace(esn, name)
	}
}

// maybeAutoGreetKnownFace 对已命名人脸做低频自动问候，避免重复路过时一直说话。
func maybeAutoGreetKnownFace(esn, name string) {
	if !vars.APIConfig.Vision.AutoGreetKnownFaces {
		return
	}
	if robotpkg.IsBusyForPassiveGreeting(esn) {
		return
	}
	if !shouldAutoGreet(esn, name) {
		return
	}
	go func() {
		if err := speakAutoGreeting(esn, name); err != nil {
			logger.Println("Auto face greeting failed for " + esn + ": " + err.Error())
		}
	}()
}

func shouldAutoGreet(esn, name string) bool {
	key := strings.ToLower(strings.TrimSpace(esn)) + "::" + strings.ToLower(strings.TrimSpace(name))
	if key == "::" || strings.HasSuffix(key, "::") {
		return false
	}

	autoGreetMu.Lock()
	defer autoGreetMu.Unlock()

	lastAt := lastAutoGreets[key]
	if !lastAt.IsZero() && time.Since(lastAt) < autoGreetCooldown {
		return false
	}
	lastAutoGreets[key] = time.Now()
	return true
}

func speakAutoGreeting(esn, name string) error {
	profile := memorypkg.LoadProfile(esn)
	text := buildAutoGreetingText(profile, name)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return robotpkg.KGSim(esn, text)
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
		return name + "，你好呀，很高兴见到你。"
	case "nickname":
		return name + "，你好呀，我认出你啦。"
	default:
		return name + "，你好呀。"
	}
}
