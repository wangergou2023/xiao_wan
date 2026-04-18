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
)

const recentFaceTTL = 45 * time.Second
const faceReappearanceWindow = 10 * time.Second
const repeatedFaceLogInterval = 15 * time.Second
const repeatedDecisionLogInterval = 8 * time.Second

type observedFaceState struct {
	Name        string
	FaceID      int32
	LastSeenAt  time.Time
	FirstSeenAt time.Time
	Reappeared  bool
}

var (
	faceWatcherMu     sync.Mutex
	faceWatchers      = map[string]bool{}
	observedFacesMu   sync.Mutex
	observedFaces     = map[string]observedFaceState{}
	faceLogMu         sync.Mutex
	lastFaceEventLogs = map[string]time.Time{}
	lastFaceMapLogs   = map[string]time.Time{}
	decisionLogMu     sync.Mutex
	lastDecisionLogs  = map[string]time.Time{}
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
