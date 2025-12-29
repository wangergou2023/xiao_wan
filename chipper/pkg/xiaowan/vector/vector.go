package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	sdk_wrapper "github.com/wangergou2023/wire-pod/chipper/pkg/sdk-wrapper"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vector"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vectorpb"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/chat"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/structured_outputs"
	"github.com/wangergou2023/wire-pod/chipper/pkg/xiaowan/tts4"
)

// AudioTask 表示音频任务结构体，包含索引、消息、WAV文件、PCM文件和错误信息
type AudioTask struct {
	Index   int
	Message string
	WavFile string
	PcmFile string
	Err     error
}

// clearAudioFiles 清理当前目录下的 mp3、wav 和 pcm 文件
// 返回: 清理过程中的错误，如果有的话
func clearAudioFiles() error {
	dir := "."
	exts := map[string]bool{".mp3": true, ".wav": true, ".pcm": true}

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if exts[filepath.Ext(path)] {
			if rmErr := os.Remove(path); rmErr != nil {
				return fmt.Errorf("failed to delete file %s: %w", path, rmErr)
			}
			fmt.Printf("Deleted file: %s\n", path)
		}
		return nil
	})
}

// playSound 播放指定的音频文件
// fileName: 要播放的音频文件路径
// 返回: 播放结果字符串和可能的错误
func playSound(fileName string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := make(chan bool)
	stop := make(chan bool)

	go func() {
		defer close(start)
		defer close(stop)
		err := sdk_wrapper.Robot.BehaviorControl(ctx, start, stop)
		if err != nil {
			logger.Println("BehaviorControl error:", err)
		}
	}()

	select {
	case <-start:
		sdk_wrapper.PlaySound(fileName)
		stop <- true
	case <-ctx.Done():
		return "", fmt.Errorf("context canceled")
	}
	return "", nil
}

// StreamingKGSim 处理流式知识图谱模拟请求，包括连接机器人、拍照、LLM聊天、生成音频并播放
// req: 请求接口（当前未使用）
// esn: 机器人序列号，用于匹配机器人信息
// transcribedText: 转录的文本内容，用于LLM处理
// isKG: 是否为知识图谱模式（当前未使用）
// 返回: 处理结果字符串和可能的错误
func StreamingKGSim(req interface{}, esn string, transcribedText string, isKG bool) (string, error) {
	logger.Println("StreamingKGSim: ", transcribedText)

	sdk_wrapper.InitSDKForWirepod(esn)

	// 找机器人
	matched := false
	var robot *vector.Vector
	var guid string
	var target string

	for _, bot := range vars.BotInfo.Robots {
		if esn == bot.Esn {
			guid = bot.GUID
			target = bot.IPAddress + ":443"
			matched = true
			break
		}
	}

	if matched {
		var err error
		robot, err = vector.New(vector.WithSerialNo(esn), vector.WithToken(guid), vector.WithTarget(target))
		if err != nil {
			return err.Error(), err
		}
	} else {
		return "", fmt.Errorf("未找到匹配的机器人 ESN: %s", esn)
	}

	// 确认连接
	if _, err := robot.Conn.BatteryState(context.Background(), &vectorpb.BatteryStateRequest{}); err != nil {
		return "", err
	}

	// 拍照（你的实现保留）
	if _, err := takePhoto(); err != nil {
		return "", err
	}

	// LLM
	resp, err := chat.OpenAIchat(transcribedText)
	if err != nil {
		return "", err
	}

	var result structured_outputs.Result
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return "", err
	}

	// 清理旧音频
	if err := clearAudioFiles(); err != nil {
		logger.Println("Error:", err)
		return "", err
	}

	if len(result.Sentences) == 0 {
		return "", fmt.Errorf("LLM 未返回 sentences")
	}

	// =========================
	// ✅ 核心优化：并发生成 + 串行按序播放
	// =========================

	taskCh := make(chan AudioTask, len(result.Sentences))
	var wg sync.WaitGroup

	// 生产者：并发生成 wav + pcm
	for i, sentence := range result.Sentences {
		i := i
		msg := sentence.Message

		wg.Add(1)
		go func() {
			defer wg.Done()

			// 1) TTS 输出建议用 .wav（LocalTTS 多数返回 wav）
			wavName := fmt.Sprintf("%s_speech%d.wav", vars.APIConfig.Knowledge.OpenAIVoice, i)
			if err := tts4.LocalTTS(wavName, msg); err != nil {
				taskCh <- AudioTask{Index: i, Message: msg, Err: fmt.Errorf("TTS error: %w", err)}
				return
			}
			if _, err := os.Stat(wavName); err != nil {
				taskCh <- AudioTask{Index: i, Message: msg, Err: fmt.Errorf("TTS output missing: %s", wavName)}
				return
			}

			// 2) ffmpeg：WAV -> 16kHz mono s16le PCM（给 Vector 播放最常见）
			pcmName := fmt.Sprintf("%s_speech%d.pcm", vars.APIConfig.Knowledge.OpenAIVoice, i)

			// 你的 volume=3 保留
			// 注意：这里输出是 raw pcm，所以用 -f s16le
			cmd := exec.Command(
				"ffmpeg",
				"-y",
				"-i", wavName,
				"-af", "volume=3",
				"-ar", "16000",
				"-ac", "1",
				"-f", "s16le",
				pcmName,
			)

			if out, err := cmd.CombinedOutput(); err != nil {
				taskCh <- AudioTask{
					Index:   i,
					Message: msg,
					WavFile: wavName,
					Err:     fmt.Errorf("ffmpeg error: %w, output=%s", err, string(out)),
				}
				return
			}

			taskCh <- AudioTask{
				Index:   i,
				Message: msg,
				WavFile: wavName,
				PcmFile: pcmName,
				Err:     nil,
			}
		}()
	}

	// 关闭通道：所有生产者结束后关闭
	go func() {
		wg.Wait()
		close(taskCh)
	}()

	// 消费者：严格按 Index 顺序播放
	next := 0
	buffer := make(map[int]AudioTask, len(result.Sentences))

	// 用于统计：避免某一句失败后死等
	failed := make(map[int]bool, len(result.Sentences))

	for task := range taskCh {
		buffer[task.Index] = task

		// 尝试连续播放 next、next+1...
		for next < len(result.Sentences) {
			t, ok := buffer[next]
			if !ok {
				break
			}
			delete(buffer, next)

			if t.Err != nil {
				logger.Printf("❌ 句子 %d 生成失败：%v\n", t.Index, t.Err)
				failed[next] = true
				// 清理可能残留的 wav
				if t.WavFile != "" {
					_ = os.Remove(t.WavFile)
				}
				next++
				continue
			}

			logger.Printf("▶️ 当前正在播放 %d “%s”\n", t.Index, t.Message)

			if _, err := playSound(t.PcmFile); err != nil {
				logger.Printf("Error playing sound: %s\n", err)
				// 播放失败也继续推进，避免整个链路卡死
			}

			time.Sleep(1 * time.Second)

			// 清理文件
			_ = os.Remove(t.WavFile)
			_ = os.Remove(t.PcmFile)

			next++
		}
	}

	// 如果有未播放的（极少见：比如某些任务从未投递），给个日志
	if next < len(result.Sentences) {
		logger.Printf("⚠️ 播放未完成：期望 %d 句，实际推进到 %d\n", len(result.Sentences), next)
	}

	return "", nil
}

// takePhoto 让机器人拍照并保存为 camera.jpg
// 返回: 拍照结果字符串和可能的错误
func takePhoto() (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := make(chan bool)
	stop := make(chan bool)

	go func() {
		defer close(start)
		defer close(stop)
		err := sdk_wrapper.Robot.BehaviorControl(ctx, start, stop)
		if err != nil {
			logger.Println("BehaviorControl error:", err)
		}
	}()

	select {
	case <-start:
		sdk_wrapper.SaveHiResCameraPicture("camera.jpg")
		stop <- true
		fmt.Println("拍照成功")
		return "", nil
	case <-ctx.Done():
		return "", fmt.Errorf("context canceled")
	}
}
