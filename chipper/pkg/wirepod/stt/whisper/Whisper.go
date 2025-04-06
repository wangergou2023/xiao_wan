package wirepod_whisper

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/orcaman/writerseeker"
	openai "github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/wire-pod/chipper/pkg/logger"
	"github.com/wangergou2023/wire-pod/chipper/pkg/vars"
	sr "github.com/wangergou2023/wire-pod/chipper/pkg/wirepod/speechrequest"
)

var Name string = "whisper"
var useLocalModel bool = false // 默认使用 OpenAI Whisper API，设为 true 使用本地模型

// 初始化函数
func Init() error {
	// 可通过环境变量控制是否使用本地模型
	if os.Getenv("USE_LOCAL_MODEL") == "true" {
		useLocalModel = true
	}

	if os.Getenv("OPENAI_KEY") == "" && !useLocalModel {
		logger.Println("This is an early implementation of the Whisper API. You must set the OPENAI_KEY env var or enable local model.")
	}
	return nil
}

// pcm2wav 函数将 PCM 音频数据转换为 WAV 格式
func pcm2wav(in io.Reader) []byte {

	// 创建输出文件缓冲区
	out := &writerseeker.WriterSeeker{}

	// 设置 WAV 文件参数: 16kHz 采样率, 16位深度, 单声道
	e := wav.NewEncoder(out, 16000, 16, 1, 1)

	// 创建音频缓冲区，将输入的 PCM 数据转换为音频缓冲区
	audioBuf, err := newAudioIntBuffer(in)
	if err != nil {
		logger.Println(err) // 如果出错，打印错误日志
	}
	// 将音频缓冲区写入输出文件，即生成 WAV 文件头和 PCM 数据块
	if err := e.Write(audioBuf); err != nil {
		logger.Println(err)
	}
	if err := e.Close(); err != nil {
		logger.Println(err)
	}
	// 将生成的 WAV 文件数据复制到缓冲区
	outBuf := new(bytes.Buffer)
	io.Copy(outBuf, out.BytesReader())
	// 返回 WAV 数据的字节数组
	return outBuf.Bytes()
}

// newAudioIntBuffer 函数将 PCM 数据转换为 audio.IntBuffer 格式
func newAudioIntBuffer(r io.Reader) (*audio.IntBuffer, error) {
	// 初始化音频缓冲区，设置格式为单声道、采样率 16kHz
	buf := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  16000,
		},
	}
	for {
		var sample int16
		// 从输入流中按小端序读取音频样本
		err := binary.Read(r, binary.LittleEndian, &sample)
		switch {
		case err == io.EOF:
			// 读取到文件末尾时返回缓冲区
			return &buf, nil
		case err != nil:
			// 如果出错，返回错误
			return nil, err
		}
		// 将样本数据添加到缓冲区
		buf.Data = append(buf.Data, int(sample))
	}
}

// makeOpenAIReq 函数调用 OpenAI Whisper API 进行语音转文本
func makeOpenAIReq(in []byte) string {
	// 直接创建一个 WAV 文件，使用指定的文件名
	tmpFile, err := os.Create("audio-output.wav") // 创建在当前目录
	if err != nil {
		logger.Println("Failed to create file:", err)
		return "There was an error."
	}

	// 将 WAV 数据写入文件
	_, err = tmpFile.Write(in)
	if err != nil {
		logger.Println("Failed to write to file:", err)
		return "There was an error."
	}

	// 刷新缓冲区并关闭文件
	tmpFile.Sync()
	tmpFile.Close()

	// 使用 OpenAI 客户端发起语音转文本请求
	conf := openai.DefaultConfig(vars.APIConfig.Knowledge.Key)
	conf.BaseURL = vars.APIConfig.Knowledge.Endpoint
	client := openai.NewClientWithConfig(conf)
	ctx := context.Background()

	fmt.Println(tmpFile.Name())
	// 创建音频转文本请求
	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: tmpFile.Name(),
	}
	// 发起请求并处理响应
	resp, err := client.CreateTranscription(ctx, req)
	if err != nil {
		logger.Println(err)
		return "There was an error."
	}

	// 返回转录的文本
	return resp.Text
}

// makeLocalRequest 函数调用本地 ASR 服务进行语音转文本
func makeLocalRequest(in []byte) (string, error) {
	// 创建一个临时文件用于存储音频数据
	tmpFile, err := os.Create("audio-input.mp3")
	if err != nil {
		logger.Println("Failed to create file:", err)
		return "", err
	}
	// 将 WAV 数据转为 MP3 格式并保存
	_, err = tmpFile.Write(in)
	if err != nil {
		logger.Println("Failed to write to file:", err)
		return "", err
	}
	tmpFile.Sync()
	tmpFile.Close()

	// 打印临时文件路径
	logger.Println("Temporary MP3 file created at:", tmpFile.Name())

	// 使用 curl 请求本地 ASR 服务
	url := "http://127.0.0.1:9000/asr?encode=true&task=transcribe&word_timestamps=false&output=txt"
	cmd := exec.Command("curl", "-s", "-X", "POST", url,
		"-H", "accept: application/json",
		"-H", "Content-Type: multipart/form-data",
		"-F", fmt.Sprintf("audio_file=@%s;type=audio/mpeg", tmpFile.Name()))

	// 打印执行的 curl 命令
	logger.Println("Executing curl command:", cmd.String())

	// 执行请求并获取响应
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Println("Error executing curl command:", err)
		return "", err
	}

	// 打印返回的响应
	logger.Println("ASR Response:", string(out))

	// 如果返回的是纯文本，直接返回文本
	return string(out), nil
}

// STT (Speech-to-Text) 函数处理语音请求，返回转录的文本
func STT(req sr.SpeechRequest) (string, error) {
	logger.Println("(Bot " + req.Device + ", Whisper) Processing...") // 打印处理日志
	speechIsDone := false
	var err error
	for {
		_, err = req.GetNextStreamChunk() // 获取下一个音频流块
		if err != nil {
			return "", err
		}
		// 检测语音结束（需要将音频数据分割为 320 字节块以进行 VAD 处理）
		speechIsDone, _ = req.DetectEndOfSpeech()
		if speechIsDone {
			break // 如果检测到语音结束，跳出循环
		}
	}

	// 将 PCM 数据转换为 WAV 格式
	pcmBufTo := &writerseeker.WriterSeeker{}
	pcmBufTo.Write(req.DecodedMicData)
	pcmBuf := pcm2wav(pcmBufTo.BytesReader())

	var transcribedText string
	// 根据标志位决定使用本地模型还是 OpenAI Whisper
	if useLocalModel {
		transcribedText, err = makeLocalRequest(pcmBuf) // 使用本地 ASR 服务进行转录
	} else {
		transcribedText = strings.ToLower(makeOpenAIReq(pcmBuf)) // 使用 OpenAI Whisper API 进行转录
	}
	if err != nil {
		logger.Println("Error:", err)
		return "", err
	}

	logger.Println("Bot " + req.Device + " Transcribed text: " + transcribedText) // 打印转录结果
	return transcribedText, nil                                                   // 返回转录文本
}
