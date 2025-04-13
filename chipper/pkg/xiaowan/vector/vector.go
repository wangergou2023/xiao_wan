package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	sdk_wrapper "github.com/wangergou2023/wire-pod/chipper/pkg/sdk-wrapper"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vector"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vectorpb"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/chat"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/structured_outputs"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/tts"
)

func clearMP3Files() error {
	// 使用当前目录
	dir := "."

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 检查文件扩展名是否为 .mp3
		if !info.IsDir() && filepath.Ext(path) == ".mp3" {
			err = os.Remove(path)
			if err != nil {
				return fmt.Errorf("failed to delete file %s: %w", path, err)
			}
			fmt.Printf("Deleted file: %s\n", path)
		}
		// 检查文件扩展名是否为 .wav
		if !info.IsDir() && filepath.Ext(path) == ".wav" {
			err = os.Remove(path)
			if err != nil {
				return fmt.Errorf("failed to delete file %s: %w", path, err)
			}
			fmt.Printf("Deleted file: %s\n", path)
		}
		// 检查文件扩展名是否为 .pcm
		if !info.IsDir() && filepath.Ext(path) == ".pcm" {
			err = os.Remove(path)
			if err != nil {
				return fmt.Errorf("failed to delete file %s: %w", path, err)
			}
			fmt.Printf("Deleted file: %s\n", path)
		}
		return nil
	})
}
func playSound(fileName string) (string, error) {
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
		sdk_wrapper.PlaySound(fileName)
		// 停止 BehaviorControl
		stop <- true
	case <-ctx.Done():
		return "", fmt.Errorf("context canceled")
	}
	return "", nil
}

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

	// 清理当前目录下的 MP3 文件
	if err := clearMP3Files(); err != nil {
		logger.Println("Error:", err)
		return "", err
	}

	// 处理每个句子
	for i, sentence := range result.Sentences {
		// 使用 goroutine 异步处理每个句子
		go func(i int, message string) {
			// 生成音频文件名
			fileName := fmt.Sprintf("%s_speech%d.mp3", vars.APIConfig.Knowledge.OpenAIVoice, i)
			// 使用 OpenAI TTS 生成音频文件
			tts.OpenAItts(fileName, message)
			// 确保文件存在
			if _, err := os.Stat(fileName); os.IsNotExist(err) {
				logger.Printf("File not found: %s\n", fileName)
				return
			}
			// 使用 ffmpeg 转换音频文件格式
			tmpFileName := fmt.Sprintf("%s_speech%d.pcm", vars.APIConfig.Knowledge.OpenAIVoice, i)
			_, err := exec.Command("ffmpeg", "-y", "-i", fileName, "-af", "volume=3", "-f", "s16le", "-acodec", "pcm_s16le", "-ar", "16000", "-ac", "1", tmpFileName).Output()
			if err != nil {
				logger.Println("Error:", err)
				return
			}

			// 检查前面的文件是否存在,存在则等待前面的文件播放完成
			if i != 0 {
				allFilesPlayed := false
				for !allFilesPlayed {
					allFilesPlayed = true // 假设所有文件都已经播放完

					// 检查前面的所有文件
					for j := 0; j < i; j++ {
						fileNameBefore := fmt.Sprintf("%s_speech%d.mp3", vars.APIConfig.Knowledge.OpenAIVoice, j)
						// 检查文件是否存在
						if _, err := os.Stat(fileNameBefore); err == nil {
							// 文件存在
							allFilesPlayed = false
							logger.Printf("等待文件 %s 播放完成...\n", fileNameBefore)
							time.Sleep(1 * time.Second) // 等待 1 秒后再次检查
							break                       // 只要找到一个文件存在，暂停检查
						}
					}
				}
			}

			logger.Printf("当前正在播放%d ” %s “\n", i, message)
			// 播放音频文件
			if _, err := playSound(tmpFileName); err != nil {
				logger.Printf("Error playing sound: %s\n", err)
				return
			}
			// 等待 1 秒
			time.Sleep(1 * time.Second)
			// 播放完成后，删除文件
			if err := os.Remove(fileName); err != nil {
				logger.Printf("Error deleting file: %s\n", err)
				return
			}
			if err := os.Remove(tmpFileName); err != nil {
				logger.Printf("Error deleting file: %s\n", err)
				return
			}
		}(i, sentence.Message)
		// 等待 1 秒
		time.Sleep(1 * time.Second)
	}

	return "", nil
}
