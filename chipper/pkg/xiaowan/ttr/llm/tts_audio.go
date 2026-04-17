package llm

import (
	"context"
	"errors"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
	robotpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/robot"
)

func pcmChunkBytes(chunks [][]byte) int {
	total := 0
	for _, chunk := range chunks {
		total += len(chunk)
	}
	return total
}

func pcmLengthFromBytes(totalBytes int) time.Duration {
	bytesPerSample := 2
	sampleRate := 16000
	numSamples := totalBytes / bytesPerSample
	return time.Duration(numSamples*1000/sampleRate) * time.Millisecond
}

// playPCM24kOnRobot 把 24k PCM 音频降采样后推给机器人外放。
// 行为控制与说话动画由上层流式播报流程统一管理，这里只负责可靠推流。
func playPCM24kOnRobot(robot *vector.Vector, speechBytes []byte) error {
	if robot == nil {
		return errors.New("robot is nil")
	}
	vclient, err := robot.Conn.ExternalAudioStreamPlayback(context.Background())
	if err != nil {
		return err
	}
	defer vclient.CloseSend()

	if err := vclient.Send(&vectorpb.ExternalAudioStreamRequest{
		AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamPrepare{
			AudioStreamPrepare: &vectorpb.ExternalAudioStreamPrepare{
				AudioFrameRate: 16000,
				AudioVolume:    100,
			},
		},
	}); err != nil {
		return err
	}

	audioChunks := robotpkg.Downsample24kTo16k(speechBytes)
	if len(audioChunks) == 0 {
		return errors.New("empty pcm after downsample")
	}

	for _, chunk := range audioChunks {
		if err := vclient.Send(&vectorpb.ExternalAudioStreamRequest{
			AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamChunk{
				AudioStreamChunk: &vectorpb.ExternalAudioStreamChunk{
					AudioChunkSizeBytes: uint32(len(chunk)),
					AudioChunkSamples:   chunk,
				},
			},
		}); err != nil {
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}

	if err := vclient.Send(&vectorpb.ExternalAudioStreamRequest{
		AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamComplete{
			AudioStreamComplete: &vectorpb.ExternalAudioStreamComplete{},
		},
	}); err != nil {
		return err
	}

	time.Sleep(pcmLengthFromBytes(pcmChunkBytes(audioChunks)) + 50*time.Millisecond)
	return nil
}
