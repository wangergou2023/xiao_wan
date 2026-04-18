package robot

import (
	"context"
	"errors"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	sdk_wrapper "github.com/wangergou2023/xiao_wan/chipper/pkg/sdk-wrapper"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
)

const (
	defaultLiftSpeed = 4.0
	defaultHeadSpeed = 1.5
)

// ensureSDKForRobot 确保 sdk-wrapper 已绑定到当前机器人，方便复用更高层的控制封装。
func ensureSDKForRobot(robot *vector.Vector) error {
	if robot == nil {
		return errors.New("robot is nil")
	}
	return sdk_wrapper.InitSDKForWirepod(robot.Cfg.SerialNo)
}

// DriveWheels 使用 sdk-wrapper 控制左右轮速度，适合精细底层运动控制。
func DriveWheels(robot *vector.Vector, leftWheel, rightWheel, leftWheel2, rightWheel2 float32) error {
	if err := ensureSDKForRobot(robot); err != nil {
		return err
	}
	sdk_wrapper.DriveWheelsForward(leftWheel, rightWheel, leftWheel2, rightWheel2)
	return nil
}

// MoveLift 使用 sdk-wrapper 控制手臂升降电机。
func MoveLift(robot *vector.Vector, speed float32) error {
	if err := ensureSDKForRobot(robot); err != nil {
		return err
	}
	sdk_wrapper.MoveLift(speed)
	return nil
}

// MoveHead 使用 sdk-wrapper 控制头部俯仰电机。
func MoveHead(robot *vector.Vector, speed float32) error {
	if err := ensureSDKForRobot(robot); err != nil {
		return err
	}
	sdk_wrapper.MoveHead(speed)
	return nil
}

// StopLift 立即停止手臂电机，适合和定时动作组合使用。
func StopLift(robot *vector.Vector) error {
	return MoveLift(robot, 0)
}

// LiftUp 以默认速度抬起手臂；方向约定基于当前项目实测习惯，可按需要继续校准。
func LiftUp(robot *vector.Vector) error {
	return MoveLift(robot, defaultLiftSpeed)
}

// LiftDown 以默认速度放下手臂。
func LiftDown(robot *vector.Vector) error {
	return MoveLift(robot, -defaultLiftSpeed)
}

// MoveLiftFor 让手臂以给定速度运动一小段时间，再自动停止。
// 这层封装比裸速度控制更适合上层场景与后续 LLM/tool 调用。
func MoveLiftFor(robot *vector.Vector, speed float32, duration time.Duration) error {
	if duration <= 0 {
		return MoveLift(robot, speed)
	}
	if err := MoveLift(robot, speed); err != nil {
		return err
	}
	time.Sleep(duration)
	return StopLift(robot)
}

// LiftUpFor 以默认速度抬手臂一段时间后自动停止。
func LiftUpFor(robot *vector.Vector, duration time.Duration) error {
	return MoveLiftFor(robot, defaultLiftSpeed, duration)
}

// LiftDownFor 以默认速度放手臂一段时间后自动停止。
func LiftDownFor(robot *vector.Vector, duration time.Duration) error {
	return MoveLiftFor(robot, -defaultLiftSpeed, duration)
}

// StopHead 立即停止头部俯仰电机。
func StopHead(robot *vector.Vector) error {
	return MoveHead(robot, 0)
}

// HeadUp 以默认速度抬头。
func HeadUp(robot *vector.Vector) error {
	return MoveHead(robot, defaultHeadSpeed)
}

// HeadDown 以默认速度低头。
func HeadDown(robot *vector.Vector) error {
	return MoveHead(robot, -defaultHeadSpeed)
}

// MoveHeadFor 让头部以给定速度俯仰一小段时间，再自动停止。
func MoveHeadFor(robot *vector.Vector, speed float32, duration time.Duration) error {
	if duration <= 0 {
		return MoveHead(robot, speed)
	}
	if err := MoveHead(robot, speed); err != nil {
		return err
	}
	time.Sleep(duration)
	return StopHead(robot)
}

// HeadUpFor 以默认速度抬头一段时间后自动停止。
func HeadUpFor(robot *vector.Vector, duration time.Duration) error {
	return MoveHeadFor(robot, defaultHeadSpeed, duration)
}

// HeadDownFor 以默认速度低头一段时间后自动停止。
func HeadDownFor(robot *vector.Vector, duration time.Duration) error {
	return MoveHeadFor(robot, -defaultHeadSpeed, duration)
}

// DriveOnCharger 使用 sdk-wrapper 执行上充电座动作。
func DriveOnCharger(robot *vector.Vector) error {
	if err := ensureSDKForRobot(robot); err != nil {
		return err
	}
	sdk_wrapper.DriveOnCharger()
	return nil
}

// DriveOffCharger 使用 sdk-wrapper 执行离开充电座动作。
func DriveOffCharger(robot *vector.Vector) error {
	if err := ensureSDKForRobot(robot); err != nil {
		return err
	}
	sdk_wrapper.DriveOffCharger()
	return nil
}

// PlayAnimationWithSDK 使用 sdk-wrapper 播放动画，减少上层直接接触底层连接细节。
func PlayAnimationWithSDK(robot *vector.Vector, animation string, loops uint32, ignoreBodyTrack bool, ignoreHeadTrack bool, ignoreLiftTrack bool) error {
	if err := ensureSDKForRobot(robot); err != nil {
		return err
	}
	sdk_wrapper.PlayAnimation(animation, loops, ignoreBodyTrack, ignoreHeadTrack, ignoreLiftTrack)
	return nil
}

// SayTextWithSDK 使用 sdk-wrapper 的 TTS 封装播报文本。
func SayTextWithSDK(robot *vector.Vector, text string) error {
	if err := ensureSDKForRobot(robot); err != nil {
		return err
	}
	sdk_wrapper.SayText(text)
	return nil
}

// GoCharge 触发系统级“回家充电”意图，让机器人自行处理找桩与回充行为。
func GoCharge(robot *vector.Vector) error {
	if robot == nil {
		return errors.New("robot is nil")
	}
	logger.Println("Robot app intent sending: intent_system_charger for " + robot.Cfg.SerialNo)
	_, err := robot.Conn.AppIntent(context.Background(), &vectorpb.AppIntentRequest{Intent: "intent_system_charger"})
	if err != nil {
		logger.Println("Robot app intent failed: intent_system_charger for " + robot.Cfg.SerialNo + ": " + err.Error())
		return err
	}
	logger.Println("Robot app intent sent: intent_system_charger for " + robot.Cfg.SerialNo)
	return err
}

// StartKnowledgeQuestion 触发系统级新的语音提问流程。
func StartKnowledgeQuestion(robot *vector.Vector) error {
	if robot == nil {
		return errors.New("robot is nil")
	}
	_, err := robot.Conn.AppIntent(context.Background(), &vectorpb.AppIntentRequest{Intent: "knowledge_question"})
	return err
}
