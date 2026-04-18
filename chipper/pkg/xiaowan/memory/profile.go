package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

const (
	memoryDocTitle         = "# Long-term Memory"
	memoryDocEmptyText     = "No durable user facts have been confirmed yet."
	memoryDocFactsHeader   = "## Durable Remembered Context"
	memoryDocGuideHeader   = "## Memory Writing Guidance"
	memoryDocSyncHeader    = "## Sync Info"
	memoryDocSourceOfTruth = "- Source of truth: workspace/memory/MEMORY.md"
)

// LoadProfile 从 workspace/memory/MEMORY.md 读取自由长期记忆。
func LoadProfile(esn string) UserProfile {
	profileMu.Lock()
	defer profileMu.Unlock()

	return loadProfileUnlocked(esn)
}

func loadProfileUnlocked(esn string) UserProfile {
	esn = strings.TrimSpace(esn)
	text := loadMemoryTextUnlocked()
	return UserProfile{
		ESN:        esn,
		MemoryText: text,
	}
}

func SaveProfile(profile UserProfile) {
	saveProfile(profile)
}

// BuildPromptContext 现在由 workspace/memory/MEMORY.md 提供长期记忆提示，避免重复注入。
func BuildPromptContext(esn string) string {
	_ = esn
	return ""
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

	path := memoryDocPath()
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(buildMemoryDoc(profile)), 0o644)
}

func memoryDocPath() string {
	return workspacepkg.ResolveWritableDocPath(filepath.Join("memory", "MEMORY.md"))
}

func loadMemoryTextUnlocked() string {
	path := memoryDocPath()
	if strings.TrimSpace(path) == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return extractDurableRememberedContext(string(data))
}

func extractDurableRememberedContext(doc string) string {
	doc = strings.ReplaceAll(doc, "\r\n", "\n")
	start := strings.Index(doc, memoryDocFactsHeader)
	if start < 0 {
		return ""
	}
	body := doc[start+len(memoryDocFactsHeader):]
	end := strings.Index(body, "\n## ")
	if end >= 0 {
		body = body[:end]
	}
	body = strings.TrimSpace(body)
	if body == "" || body == memoryDocEmptyText {
		return ""
	}
	return body
}

func buildMemoryDoc(profile UserProfile) string {
	var b strings.Builder
	b.WriteString(memoryDocTitle + "\n\n")
	b.WriteString("This file stores freeform durable memory that may matter across conversations.\n\n")
	b.WriteString("## How To Read This File\n\n")
	b.WriteString("- Treat this as living memory, not a rigid database.\n")
	b.WriteString("- Prefer facts that are stable, user-confirmed, and likely to matter later.\n")
	b.WriteString("- Prefer concise summaries over chat transcripts.\n")
	b.WriteString("- If something is uncertain or old, keep the uncertainty visible instead of pretending it is fresh.\n\n")
	b.WriteString(memoryDocFactsHeader + "\n\n")
	if strings.TrimSpace(profile.MemoryText) == "" {
		b.WriteString(memoryDocEmptyText + "\n")
	} else {
		b.WriteString(strings.TrimSpace(profile.MemoryText))
		if !strings.HasSuffix(strings.TrimSpace(profile.MemoryText), "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n" + memoryDocGuideHeader + "\n\n")
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
	b.WriteString(memoryDocSyncHeader + "\n\n")
	b.WriteString(fmt.Sprintf("- Robot ESN: %s\n", strings.TrimSpace(profile.ESN)))
	b.WriteString(memoryDocSourceOfTruth + "\n")
	return b.String()
}
