package llm

import (
	"strings"
	"sync"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
)

type ttsPrefetchEntry struct {
	ready chan struct{}
	audio []byte
	err   error
}

// ttsPrefetchSession 负责把下一句的 TTS 请求提前拉起，等上一句播放时下一句尽量已经准备好了。
// 这里用 map + mutex + ready channel 做并发保护，避免重复请求同一句文本。
type ttsPrefetchSession struct {
	enabled bool
	mu      sync.Mutex
	entries map[string]*ttsPrefetchEntry
}

func newTTSPrefetchSession() *ttsPrefetchSession {
	return &ttsPrefetchSession{
		enabled: bigModelTTSEnabled(),
		entries: make(map[string]*ttsPrefetchEntry),
	}
}

func (s *ttsPrefetchSession) PreloadFromRaw(raw string) {
	if s == nil || !s.enabled {
		return
	}
	for _, action := range GetActionsFromString(raw) {
		if action.Action != ActionSayText {
			continue
		}
		s.PreloadText(action.Parameter)
	}
}

func (s *ttsPrefetchSession) PreloadText(input string) {
	if s == nil || !s.enabled {
		return
	}
	text := normalizeSpeechText(input)
	if text == "" {
		return
	}

	s.mu.Lock()
	if _, ok := s.entries[text]; ok {
		s.mu.Unlock()
		return
	}
	entry := &ttsPrefetchEntry{ready: make(chan struct{})}
	s.entries[text] = entry
	s.mu.Unlock()

	go func() {
		entry.audio, entry.err = requestBigModelSpeech(text)
		if entry.err != nil {
			logger.Println("BigModel TTS prefetch failed, will fall back if needed: " + entry.err.Error())
		}
		close(entry.ready)
	}()
}

func (s *ttsPrefetchSession) Take(input string) ([]byte, bool, error) {
	if s == nil || !s.enabled {
		return nil, false, nil
	}
	text := normalizeSpeechText(input)
	if text == "" {
		return nil, false, nil
	}

	s.mu.Lock()
	entry, ok := s.entries[text]
	s.mu.Unlock()
	if !ok {
		return nil, false, nil
	}

	<-entry.ready

	s.mu.Lock()
	delete(s.entries, text)
	s.mu.Unlock()
	return entry.audio, true, entry.err
}

func normalizeSpeechText(input string) string {
	return strings.TrimSpace(removeSpecialCharacters(input))
}
