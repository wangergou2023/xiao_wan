package wirepod_bigmodel

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	sr "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/speechrequest"
	"github.com/orcaman/writerseeker"
)

var Name string = "bigmodel"

type bigmodelResp struct {
	Text string `json:"text"`
}

func Init() error {
	if os.Getenv("BIGMODEL_API_TOKEN") == "" {
		logger.Println("BIGMODEL_API_TOKEN not found. You must set it to use BigModel GLM-ASR.")
	}
	return nil
}

func pcm2wav(in io.Reader) []byte {
	out := &writerseeker.WriterSeeker{}

	// 16 kHz, 16 bit, 1 channel, WAV.
	e := wav.NewEncoder(out, 16000, 16, 1, 1)

	audioBuf, err := newAudioIntBuffer(in)
	if err != nil {
		logger.Println(err)
	}
	if err := e.Write(audioBuf); err != nil {
		logger.Println(err)
	}
	if err := e.Close(); err != nil {
		logger.Println(err)
	}
	outBuf := new(bytes.Buffer)
	io.Copy(outBuf, out.BytesReader())
	return outBuf.Bytes()
}

func newAudioIntBuffer(r io.Reader) (*audio.IntBuffer, error) {
	buf := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  16000,
		},
	}
	for {
		var sample int16
		err := binary.Read(r, binary.LittleEndian, &sample)
		switch {
		case err == io.EOF:
			return &buf, nil
		case err != nil:
			return nil, err
		}
		buf.Data = append(buf.Data, int(sample))
	}
}

func makeBigModelReq(in []byte) string {
	url := "https://open.bigmodel.cn/api/paas/v4/audio/transcriptions"
	model := os.Getenv("BIGMODEL_ASR_MODEL")
	if model == "" {
		model = "glm-asr-2512"
	}

	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)
	w.WriteField("model", model)
	w.WriteField("stream", "false")
	sendFile, _ := w.CreateFormFile("file", "audio.wav")
	sendFile.Write(in)
	w.Close()

	httpReq, _ := http.NewRequest("POST", url, buf)
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("BIGMODEL_API_TOKEN"))

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		logger.Println(err)
		return "There was an error."
	}
	defer resp.Body.Close()

	response, _ := io.ReadAll(resp.Body)

	var bmResponse bigmodelResp
	json.Unmarshal(response, &bmResponse)

	return bmResponse.Text
}

func STT(req sr.SpeechRequest) (string, error) {
	logger.Println("(Bot " + req.Device + ", BigModel) Processing...")
	speechIsDone := false
	var err error
	for {
		_, err = req.GetNextStreamChunk()
		if err != nil {
			return "", err
		}
		speechIsDone, _ = req.DetectEndOfSpeech()
		if speechIsDone {
			break
		}
	}

	pcmBufTo := &writerseeker.WriterSeeker{}
	pcmBufTo.Write(req.DecodedMicData)
	pcmBuf := pcm2wav(pcmBufTo.BytesReader())

	transcribedText := strings.ToLower(makeBigModelReq(pcmBuf))
	logger.Println("Bot " + req.Device + " Transcribed text: " + transcribedText)
	return transcribedText, nil
}
