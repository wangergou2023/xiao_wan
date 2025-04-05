package sdk_wrapper

import "github.com/wangergou2023/wire-pod/chipper/pkg/vectorpb"

func DriveWheelsForward(lw float32, rw float32, lwtwo float32, rwtwo float32) {
	_, _ = Robot.Conn.DriveWheels(
		ctx,
		&vectorpb.DriveWheelsRequest{
			LeftWheelMmps:   lw,
			RightWheelMmps:  rw,
			LeftWheelMmps2:  lwtwo,
			RightWheelMmps2: rwtwo,
		},
	)
}

func MoveLift(speed float32) {
	_, _ = Robot.Conn.MoveLift(
		ctx,
		&vectorpb.MoveLiftRequest{
			SpeedRadPerSec: speed,
		},
	)
}

func MoveHead(speed float32) {
	_, _ = Robot.Conn.MoveHead(
		ctx,
		&vectorpb.MoveHeadRequest{
			SpeedRadPerSec: speed,
		},
	)
}

func DriveOffCharger() {
	_, _ = Robot.Conn.DriveOffCharger(
		ctx,
		&vectorpb.DriveOffChargerRequest{},
	)
}

func DriveOnCharger() {
	_, _ = Robot.Conn.DriveOnCharger(
		ctx,
		&vectorpb.DriveOnChargerRequest{},
	)
}
