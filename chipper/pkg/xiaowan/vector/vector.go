package vector

import (
	"context"

	sdk_wrapper "github.com/wangergou2023/wire-pod/chipper/pkg/sdk-wrapper"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vector"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vectorpb"
)

func StreamingKGSim(req interface{}, esn string, transcribedText string, isKG bool) (string, error) {

	sdk_wrapper.InitSDKForWirepod(esn)

	// 初始化匹配标志为假
	matched := false
	var robot *vector.Vector // 声明一个向量机器人类型的指针变量
	var guid string          // 机器人的全局唯一标识符
	var target string        // 机器人的目标IP和端口字符串

	// 遍历所有已知的机器人信息
	for _, bot := range vars.BotInfo.Robots {
		if esn == bot.Esn { // 如果找到与提供的ESN匹配的机器人
			guid = bot.GUID                 // 获取该机器人的GUID
			target = bot.IPAddress + ":443" // 设置目标IP和端口，端口固定为443
			matched = true                  // 设置匹配标志为真
			break                           // 找到匹配项后退出循环
		}
	}

	// 如果成功匹配到机器人
	if matched {
		var err error
		// 尝试创建一个新的机器人连接实例
		robot, err = vector.New(vector.WithSerialNo(esn), vector.WithToken(guid), vector.WithTarget(target))
		if err != nil {
			return err.Error(), err // 如果创建失败，返回错误
		}
	}

	// 获取电池状态，以确保连接成功
	_, err := robot.Conn.BatteryState(context.Background(), &vectorpb.BatteryStateRequest{})
	if err != nil {
		return "", err
	}

	return "", nil
}
