package llm

import (
	"context"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
)

type speechAnimationConfig struct {
	GetIn  string
	Loop   string
	GetOut string
}

type speechAnimationSession struct {
	robot      *vector.Vector
	ctx        context.Context
	config     speechAnimationConfig
	enableLoop bool
	stopLoop   chan struct{}
	loopDone   chan struct{}
}

func newSpeechAnimationConfig(isKG bool) speechAnimationConfig {
	if isKG {
		return speechAnimationConfig{
			GetIn: "anim_knowledgegraph_searching_getout_01",
			Loop:  "anim_knowledgegraph_answer_01",
		}
	}
	return speechAnimationConfig{
		GetIn:  "anim_getin_tts_01",
		Loop:   "anim_tts_loop_02",
		GetOut: "anim_getout_tts_01",
	}
}

func newSpeechAnimationSession(robot *vector.Vector, ctx context.Context, config speechAnimationConfig, enableLoop bool) *speechAnimationSession {
	return &speechAnimationSession{
		robot:      robot,
		ctx:        ctx,
		config:     config,
		enableLoop: enableLoop,
		stopLoop:   make(chan struct{}),
		loopDone:   make(chan struct{}),
	}
}

// Start 在语音播报前播放入场动画，并按需启动循环说话动画。
func (s *speechAnimationSession) Start() {
	if s == nil || s.robot == nil {
		return
	}
	s.playAnimation(s.config.GetIn)
	if !s.enableLoop || s.config.Loop == "" {
		close(s.loopDone)
		return
	}
	go func() {
		defer close(s.loopDone)
		for {
			select {
			case <-s.stopLoop:
				return
			default:
			}
			s.playAnimation(s.config.Loop)
			time.Sleep(60 * time.Millisecond)
		}
	}()
}

// Stop 结束循环动画，并按需播放退场动画。
func (s *speechAnimationSession) Stop(playGetOut bool) {
	if s == nil {
		return
	}
	select {
	case <-s.stopLoop:
	default:
		close(s.stopLoop)
	}
	<-s.loopDone
	if playGetOut {
		s.playAnimation(s.config.GetOut)
	}
}

func (s *speechAnimationSession) playAnimation(name string) {
	if s == nil || s.robot == nil || name == "" {
		return
	}
	if _, err := s.robot.Conn.PlayAnimation(s.ctx, &vectorpb.PlayAnimationRequest{
		Animation: &vectorpb.Animation{Name: name},
		Loops:     1,
	}); err != nil {
		logger.Println("speech animation failed: " + err.Error())
	}
}
