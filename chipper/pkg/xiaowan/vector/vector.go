package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	sdk_wrapper "github.com/wangergou2023/wire-pod/chipper/pkg/sdk-wrapper"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vector"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vectorpb"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/chat"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/structured_outputs"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/tts"
)

func StreamingKGSim(req interface{}, esn string, transcribedText string, isKG bool) (string, error) {

	logger.Println("StreamingKGSim: ", transcribedText)

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

	resp, err := chat.OpenAIchat(transcribedText)
	if err != nil {
		return "", err
	}

	var result structured_outputs.Result

	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return "", err
	}

	for i, sentence := range result.Sentences {
		logger.Println(i, sentence.Message)
		tts.TtsChat(i, resp)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := make(chan bool)
	stop := make(chan bool)

	// BehaviorControl Goroutine
	go func() {
		defer close(start)
		defer close(stop)
		err := sdk_wrapper.Robot.BehaviorControl(ctx, start, stop)
		if err != nil {
			logger.Println("BehaviorControl error:", err)
		}
	}()

	// 等待 BehaviorControl 启动
	select {
	case <-start:
		fileName := fmt.Sprintf("%s_speech.mp3", vars.APIConfig.Knowledge.OpenAIVoice)

		// 确保文件存在
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			logger.Printf("File not found: %s\n", fileName)
			return "", err
		}

		sdk_wrapper.SetMasterVolume(4)
		sdk_wrapper.PlaySound(fileName)

		// 停止 BehaviorControl
		stop <- true
	case <-ctx.Done():
		return "", err
	}

	return "", nil
}
