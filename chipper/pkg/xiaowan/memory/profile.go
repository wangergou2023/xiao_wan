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

// EnsureWorkspaceMemoryDoc 从长期记忆 json 源数据恢复并同步 workspace/memory/MEMORY.md。
// 这样即使进程重启，只要 json 还在，下一次请求组 prompt 时也能重新读到长期记忆。
func EnsureWorkspaceMemoryDoc(esn string) {
	profileMu.Lock()
	defer profileMu.Unlock()

	profile := loadProfileUnlocked(esn)
	if strings.TrimSpace(profile.ESN) == "" {
		return
	}
	syncProfileToMemoryDoc(profile)
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
	b.WriteString("This file stores freeform durable memory that may matter across conversations.\n\n")
	b.WriteString("## How To Read This File\n\n")
	b.WriteString("- Treat this as living memory, not a rigid database.\n")
	b.WriteString("- Prefer facts that are stable, user-confirmed, and likely to matter later.\n")
	b.WriteString("- Prefer concise summaries over chat transcripts.\n")
	b.WriteString("- If something is uncertain or old, keep the uncertainty visible instead of pretending it is fresh.\n\n")
	b.WriteString("## Durable Remembered Context\n\n")
	if strings.TrimSpace(profile.MemoryText) == "" {
		b.WriteString("No durable user facts have been confirmed yet.\n")
	} else {
		b.WriteString(profile.MemoryText)
		if !strings.HasSuffix(profile.MemoryText, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Memory Writing Guidance\n\n")
	b.WriteString("Good long-term memory includes:\n")
	b.WriteString("- how the user wants to be addressed\n")
	b.WriteString("- stable relationship facts\n")
	b.WriteString("- lasting preferences about greetings, language, and style\n")
	b.WriteString("- important life context the user explicitly wants remembered\n")
	b.WriteString("- repeated interests or dislikes that clearly matter over time\n\n")
	b.WriteString("Do not store as long-term memory:\n")
	b.WriteString("- one-off requests\n")
	b.WriteString("- temporary moods\n")
	b.WriteString("- raw multi-turn chat logs\n")
	b.WriteString("- sensitive details unless the user clearly wants them remembered\n")
	b.WriteString("- guesses inferred without confirmation\n\n")
	b.WriteString("## Sync Info\n\n")
	b.WriteString(fmt.Sprintf("- Robot ESN: %s\n", profile.ESN))
	b.WriteString("- Source of truth: confirmed freeform memory saved by the system\n")
	return b.String()
}
