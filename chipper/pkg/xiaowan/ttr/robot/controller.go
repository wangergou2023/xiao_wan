package robot

import (
	"context"
	"errors"

	sdk_wrapper "github.com/wangergou2023/xiao_wan/chipper/pkg/sdk-wrapper"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
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
	_, err := robot.Conn.AppIntent(context.Background(), &vectorpb.AppIntentRequest{Intent: "intent_system_charger"})
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
