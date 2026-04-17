package llm

import (
	"context"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/vector"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
	robotpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/robot"
)

func pcmLength(data []byte) time.Duration {
	bytesPerSample := 2
	sampleRate := 16000
	numSamples := len(data) / bytesPerSample
	return time.Duration(numSamples*1000/sampleRate) * time.Millisecond
}

// playPCM24kOnRobot 把 24k PCM 音频降采样后推给机器人外放。
func playPCM24kOnRobot(robot *vector.Vector, speechBytes []byte) error {
	vclient, err := robot.Conn.ExternalAudioStreamPlayback(context.Background())
	if err != nil {
		return err
	}
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
	var playedBytes []byte
	for _, chunk := range audioChunks {
		playedBytes = append(playedBytes, chunk...)
	}

	go func() {
		for _, chunk := range audioChunks {
			_ = vclient.Send(&vectorpb.ExternalAudioStreamRequest{
				AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamChunk{
					AudioStreamChunk: &vectorpb.ExternalAudioStreamChunk{
						AudioChunkSizeBytes: 1024,
						AudioChunkSamples:   chunk,
					},
				},
			})
			time.Sleep(25 * time.Millisecond)
		}
		_ = vclient.Send(&vectorpb.ExternalAudioStreamRequest{
			AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamComplete{
				AudioStreamComplete: &vectorpb.ExternalAudioStreamComplete{},
			},
		})
	}()

	time.Sleep(pcmLength(playedBytes) + 50*time.Millisecond)
	return nil
}
