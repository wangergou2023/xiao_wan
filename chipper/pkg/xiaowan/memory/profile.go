package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	workspacepkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/workspace"
)

type UserProfile struct {
	ESN        string `json:"esn"`
	MemoryText string `json:"memory_text,omitempty"`
}

type EditableProfile struct {
	ESN        string `json:"esn"`
	MemoryText string `json:"memory_text"`
}

var profileMu sync.Mutex

// LoadProfile 读取指定机器人的自由长期记忆。
func LoadProfile(esn string) UserProfile {
	profileMu.Lock()
	defer profileMu.Unlock()

	return loadProfileUnlocked(esn)
}

func loadProfileUnlocked(esn string) UserProfile {
	esn = strings.TrimSpace(esn)
	if esn == "" {
		return UserProfile{}
	}

	path := profilePath(esn)
	data, err := os.ReadFile(path)
	if err != nil {
		return UserProfile{ESN: esn}
	}

	var profile UserProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		logger.Println("Failed to unmarshal long-term memory: " + err.Error())
		return UserProfile{ESN: esn}
	}
	if strings.TrimSpace(profile.ESN) == "" {
		profile.ESN = esn
	}
	profile.MemoryText = strings.TrimSpace(profile.MemoryText)
	return profile
}

func SaveProfile(profile UserProfile) {
	saveProfile(profile)
}

// BuildPromptContext 现在由 workspace/memory/MEMORY.md 提供长期记忆提示，避免重复注入。
func BuildPromptContext(esn string) string {
	_ = esn
	return ""
}

func profilesDir() string {
	return filepath.Join(filepath.Dir(vars.ApiConfigPath), "memory_profiles")
}

func profilePath(esn string) string {
	safe := strings.ToLower(strings.TrimSpace(esn))
	safe = strings.ReplaceAll(safe, "/", "_")
	return filepath.Join(profilesDir(), safe+".json")
}

func LoadEditableProfile(esn string) EditableProfile {
	profile := LoadProfile(esn)
	return EditableProfile{
		ESN:        profile.ESN,
		MemoryText: profile.MemoryText,
	}
}

func SaveEditableProfile(editable EditableProfile) {
	saveProfile(UserProfile{
		ESN:        strings.TrimSpace(editable.ESN),
		MemoryText: strings.TrimSpace(editable.MemoryText),
	})
}

func saveProfile(profile UserProfile) {
	profileMu.Lock()
	defer profileMu.Unlock()

	profile.ESN = strings.TrimSpace(profile.ESN)
	profile.MemoryText = strings.TrimSpace(profile.MemoryText)
	if profile.ESN == "" {
		return
	}

	if err := os.MkdirAll(profilesDir(), 0o755); err != nil {
		logger.Println("Failed to create profile dir: " + err.Error())
		return
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		logger.Println("Failed to marshal long-term memory: " + err.Error())
		return
	}
	if err := os.WriteFile(profilePath(profile.ESN), data, 0o644); err != nil {
		logger.Println("Failed to write long-term memory: " + err.Error())
		return
	}

	syncProfileToMemoryDoc(profile)
}

func syncProfileToMemoryDoc(profile UserProfile) {
	path := workspacepkg.ResolveWritableDocPath(filepath.Join("memory", "MEMORY.md"))
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logger.Println("Failed to create workspace memory dir: " + err.Error())
		return
	}
	if err := os.WriteFile(path, []byte(buildMemoryDoc(profile)), 0o644); err != nil {
		logger.Println("Failed to sync MEMORY.md: " + err.Error())
	}
}

func buildMemoryDoc(profile UserProfile) string {
	var b strings.Builder
	b.WriteString("# Long-term Memory\n\n")
	b.WriteString("This file stores freeform durable memory for the robot.\n\n")
	b.WriteString("## Remembered Notes\n\n")
	if strings.TrimSpace(profile.MemoryText) == "" {
		b.WriteString("- No durable memory has been confirmed yet.\n")
	} else {
		b.WriteString(profile.MemoryText)
		if !strings.HasSuffix(profile.MemoryText, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Sync Info\n\n")
	b.WriteString(fmt.Sprintf("- Robot ESN: %s\n", profile.ESN))
	b.WriteString("- Source of truth: freeform long-term memory saved by web settings or confirmed remembered facts\n")
	return b.String()
}
