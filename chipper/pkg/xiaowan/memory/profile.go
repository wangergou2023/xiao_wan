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

const (
	memoryManualBegin = "<!-- MANUAL_NOTES_BEGIN -->"
	memoryManualEnd   = "<!-- MANUAL_NOTES_END -->"
)

type UserProfile struct {
	ESN               string   `json:"esn"`
	UserName          string   `json:"user_name,omitempty"`
	OwnerName         string   `json:"owner_name,omitempty"`
	Nickname          string   `json:"nickname,omitempty"`
	PreferredLanguage string   `json:"preferred_language,omitempty"`
	PreferredGreeting string   `json:"preferred_greeting,omitempty"`
	ForbiddenTopics   []string `json:"forbidden_topics,omitempty"`
	FavoriteTopics    []string `json:"favorite_topics,omitempty"`
	Facts             []string `json:"facts,omitempty"`
}

type EditableProfile struct {
	Profile     UserProfile `json:"profile"`
	ManualNotes string      `json:"manual_notes"`
}

var profileMu sync.Mutex

// LoadProfile 读取指定机器人的长期用户画像。
func LoadProfile(esn string) UserProfile {
	profileMu.Lock()
	defer profileMu.Unlock()

	if strings.TrimSpace(esn) == "" {
		return UserProfile{}
	}
	path := profilePath(esn)
	data, err := os.ReadFile(path)
	if err != nil {
		return UserProfile{ESN: esn}
	}
	var profile UserProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		logger.Println("Failed to unmarshal user profile: " + err.Error())
		return UserProfile{ESN: esn}
	}
	if strings.TrimSpace(profile.ESN) == "" {
		profile.ESN = esn
	}
	return profile
}

// SaveProfile 把长期画像落盘到 apiConfig 同目录，方便 packaged / dev 环境共用路径策略。
func SaveProfile(profile UserProfile) {
	saveProfileWithManualNotes(profile, LoadManualNotes())
}

// BuildPromptContext 把长期画像整理成附加 prompt，供 LLM 在每轮会话前注入。
func BuildPromptContext(esn string) string {
	profile := LoadProfile(esn)
	var lines []string
	if profile.OwnerName != "" {
		lines = append(lines, "The owner's name is "+profile.OwnerName+".")
	}
	if profile.UserName != "" && profile.UserName != profile.OwnerName {
		lines = append(lines, "The user's name is "+profile.UserName+".")
	}
	if profile.Nickname != "" {
		lines = append(lines, "Preferred nickname or form of address: "+profile.Nickname+".")
	}
	if profile.PreferredLanguage != "" {
		lines = append(lines, "Preferred language: "+profile.PreferredLanguage+".")
	}
	if profile.PreferredGreeting != "" {
		lines = append(lines, "Preferred greeting style: "+profile.PreferredGreeting+".")
	}
	if len(profile.FavoriteTopics) > 0 {
		lines = append(lines, "Favorite topics: "+strings.Join(profile.FavoriteTopics, ", ")+".")
	}
	if len(profile.ForbiddenTopics) > 0 {
		lines = append(lines, "Topics to avoid unless necessary: "+strings.Join(profile.ForbiddenTopics, ", ")+".")
	}
	for _, fact := range profile.Facts {
		fact = strings.TrimSpace(fact)
		if fact == "" {
			continue
		}
		lines = append(lines, "Remembered fact: "+fact+".")
	}
	return strings.Join(lines, "\n")
}

func profilesDir() string {
	return filepath.Join(filepath.Dir(vars.ApiConfigPath), "memory_profiles")
}

func profilePath(esn string) string {
	safe := strings.ToLower(strings.TrimSpace(esn))
	safe = strings.ReplaceAll(safe, "/", "_")
	return filepath.Join(profilesDir(), safe+".json")
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
	manualNotes := readManualMemoryNotes(path)
	content := buildMemoryDoc(profile, manualNotes)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		logger.Println("Failed to sync MEMORY.md: " + err.Error())
	}
}

// LoadEditableProfile 返回结构化画像加上当前 MEMORY.md 手工补充区，供 Web UI 编辑。
func LoadEditableProfile(esn string) EditableProfile {
	return EditableProfile{
		Profile:     LoadProfile(esn),
		ManualNotes: LoadManualNotes(),
	}
}

// SaveEditableProfile 同时更新结构化画像和 MEMORY.md 手工补充区。
func SaveEditableProfile(editable EditableProfile) {
	saveProfileWithManualNotes(editable.Profile, editable.ManualNotes)
}

func saveProfileWithManualNotes(profile UserProfile, manualNotes string) {
	profileMu.Lock()
	defer profileMu.Unlock()

	if strings.TrimSpace(profile.ESN) == "" {
		return
	}
	dir := profilesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Println("Failed to create profile dir: " + err.Error())
		return
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		logger.Println("Failed to marshal user profile: " + err.Error())
		return
	}
	if err := os.WriteFile(profilePath(profile.ESN), data, 0o644); err != nil {
		logger.Println("Failed to write user profile: " + err.Error())
	}
	syncProfileToMemoryDocWithNotes(profile, manualNotes)
}

// LoadManualNotes 读取 MEMORY.md 中不会被自动覆盖的手工区。
func LoadManualNotes() string {
	path := workspacepkg.ResolveWritableDocPath(filepath.Join("memory", "MEMORY.md"))
	return readManualMemoryNotes(path)
}

func buildMemoryDoc(profile UserProfile, manualNotes string) string {
	var b strings.Builder
	b.WriteString("# Long-term Memory\n\n")
	b.WriteString("This file is synchronized from structured long-term profile memory.\n\n")
	b.WriteString("## User Information\n\n")
	if profile.UserName != "" {
		b.WriteString("- User name: " + profile.UserName + "\n")
	}
	if profile.OwnerName != "" {
		b.WriteString("- Owner name: " + profile.OwnerName + "\n")
	}
	if profile.Nickname != "" {
		b.WriteString("- Preferred nickname: " + profile.Nickname + "\n")
	}
	if profile.UserName == "" && profile.OwnerName == "" {
		b.WriteString("- No confirmed user identity facts yet.\n")
	}

	b.WriteString("\n## Preferences\n\n")
	if profile.PreferredLanguage != "" {
		b.WriteString("- Preferred language: " + profile.PreferredLanguage + "\n")
	}
	if profile.PreferredGreeting != "" {
		b.WriteString("- Preferred greeting: " + profile.PreferredGreeting + "\n")
	}
	if profile.PreferredLanguage == "" && profile.PreferredGreeting == "" {
		b.WriteString("- No confirmed greeting preference yet.\n")
	}
	if len(profile.FavoriteTopics) > 0 {
		b.WriteString("- Favorite topics: " + strings.Join(profile.FavoriteTopics, ", ") + "\n")
	}
	if len(profile.ForbiddenTopics) > 0 {
		b.WriteString("- Topics to avoid: " + strings.Join(profile.ForbiddenTopics, ", ") + "\n")
	}

	b.WriteString("\n## Important Notes\n\n")
	if len(profile.Facts) == 0 {
		b.WriteString("- No additional confirmed long-term notes yet.\n")
	} else {
		for _, fact := range profile.Facts {
			fact = strings.TrimSpace(fact)
			if fact == "" {
				continue
			}
			b.WriteString("- " + fact + "\n")
		}
	}

	b.WriteString("\n## Sync Info\n\n")
	b.WriteString(fmt.Sprintf("- Robot ESN: %s\n", profile.ESN))
	b.WriteString("- Source of truth: structured profile JSON + confirmed user statements\n")
	b.WriteString("\n## Manual Notes\n\n")
	b.WriteString("Anything between the markers below is preserved during automatic sync.\n\n")
	b.WriteString(memoryManualBegin + "\n")
	if strings.TrimSpace(manualNotes) == "" {
		b.WriteString("- Add hand-written long-term notes here.\n")
	} else {
		b.WriteString(strings.TrimSpace(manualNotes) + "\n")
	}
	b.WriteString(memoryManualEnd + "\n")
	return b.String()
}

func readManualMemoryNotes(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	start := strings.Index(content, memoryManualBegin)
	end := strings.Index(content, memoryManualEnd)
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	start += len(memoryManualBegin)
	return strings.TrimSpace(content[start:end])
}

func syncProfileToMemoryDocWithNotes(profile UserProfile, manualNotes string) {
	path := workspacepkg.ResolveWritableDocPath(filepath.Join("memory", "MEMORY.md"))
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logger.Println("Failed to create workspace memory dir: " + err.Error())
		return
	}
	content := buildMemoryDoc(profile, manualNotes)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		logger.Println("Failed to sync MEMORY.md: " + err.Error())
	}
}
