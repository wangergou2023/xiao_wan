package vision

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
)

const recentFaceTTL = 45 * time.Second
const faceReappearanceWindow = 10 * time.Second

type observedFaceState struct {
	Name        string
	FaceID      int32
	LastSeenAt  time.Time
	FirstSeenAt time.Time
	Reappeared  bool
}

var (
	faceWatcherMu   sync.Mutex
	faceWatchers    = map[string]bool{}
	observedFacesMu sync.Mutex
	observedFaces   = map[string]observedFaceState{}
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
	_ = esn
	name := strings.TrimSpace(state.Name)
	if name == "" {
		return ""
	}
	return "Current visual context: The robot recently recognized a face named " + name + " standing in front of it."
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

	_ = faceID
	_ = name
}
