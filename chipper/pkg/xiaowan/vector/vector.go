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

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	sdk_wrapper "github.com/wangergou2023/xiao_wan/chipper/pkg/sdk-wrapper"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/chat"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/structured_outputs"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/tts4"
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

// ---------- 工具函数 ----------
func wavToPCM(wavName, pcmName string) error {
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
		return fmt.Errorf("ffmpeg error: %w, output=%s", err, string(out))
	}
	return nil
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
	// Pipeline: 串行 TTS → 并行 ffmpeg → 顺序播放
	// =========================

	// 任务结构
	type TTSTask struct {
		Index   int
		Message string
		WavFile string
		PcmFile string
		Err     error
	}

	// ---------- Stage 1: TTS 串行 ----------
	ttsCh := make(chan TTSTask)
	wavCh := make(chan TTSTask)

	// ⚠️ 只有这个 goroutine 会调用 tts4.LocalTTS
	go func() {
		for task := range ttsCh {
			wav := fmt.Sprintf("%s_speech%d.wav",
				vars.APIConfig.Knowledge.OpenAIVoice, task.Index)

			if err := tts4.LocalTTS(wav, task.Message); err != nil {
				task.Err = fmt.Errorf("TTS error: %w", err)
			} else {
				task.WavFile = wav
			}
			wavCh <- task
		}
		close(wavCh)
	}()

	// ---------- Stage 2: ffmpeg 并行 ----------
	pcmCh := make(chan TTSTask)
	var wg sync.WaitGroup

	workerNum := 3 // ffmpeg 并发数，可调
	wg.Add(workerNum)

	for i := 0; i < workerNum; i++ {
		go func() {
			defer wg.Done()
			for task := range wavCh {
				if task.Err != nil {
					pcmCh <- task
					continue
				}

				pcm := fmt.Sprintf("%s_speech%d.pcm",
					vars.APIConfig.Knowledge.OpenAIVoice, task.Index)

				if err := wavToPCM(task.WavFile, pcm); err != nil {
					task.Err = err
				} else {
					task.PcmFile = pcm
				}
				pcmCh <- task
			}
		}()
	}

	go func() {
		wg.Wait()
		close(pcmCh)
	}()

	// ---------- Stage 3: 顺序播放 ----------
	next := 0
	buffer := make(map[int]TTSTask)

	go func() {
		for i, s := range result.Sentences {
			ttsCh <- TTSTask{
				Index:   i,
				Message: s.Message,
			}
		}
		close(ttsCh)
	}()

	for task := range pcmCh {
		buffer[task.Index] = task

		for {
			t, ok := buffer[next]
			if !ok {
				break
			}
			delete(buffer, next)

			if t.Err != nil {
				logger.Printf("❌ 句子 %d 失败: %v\n", t.Index, t.Err)
				if t.WavFile != "" {
					_ = os.Remove(t.WavFile)
				}
				next++
				continue
			}

			logger.Printf("▶️ 播放 %d: %s\n", t.Index, t.Message)

			if _, err := playSound(t.PcmFile); err != nil {
				logger.Printf("播放失败: %v\n", err)
			}

			time.Sleep(1 * time.Second)

			_ = os.Remove(t.WavFile)
			_ = os.Remove(t.PcmFile)

			next++
		}
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
