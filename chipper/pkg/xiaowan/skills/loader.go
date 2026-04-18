package skills

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	workspacepkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/workspace"
)

type Skill struct {
	Name        string
	Description string
	Prompt      string
	AutoUse     bool
	Source      string
}

var (
	cachedSkills []Skill
	loadOnce     sync.Once
)

// LoadSkills 加载内置技能以及本地技能目录里的 SKILL.md。
// 现在只加载 workspace 下的本地 SKILL.md，避免内置 skill 继续污染 prompt。
func LoadSkills() []Skill {
	loadOnce.Do(func() {
		cachedSkills = loadLocalSkills()
	})
	return append([]Skill(nil), cachedSkills...)
}

// BuildAutoSkillPrompt 把标记为 auto_use 的 skill 提示词拼成一段附加 system prompt。
func BuildAutoSkillPrompt() string {
	skills := LoadSkills()
	var sections []string
	for _, skill := range skills {
		if !skill.AutoUse || strings.TrimSpace(skill.Prompt) == "" {
			continue
		}
		sections = append(sections, "Skill: "+skill.Name+"\n"+strings.TrimSpace(skill.Prompt))
	}
	return strings.Join(sections, "\n\n")
}

// BuildSkillCatalogPrompt builds a short skill index, inspired by mimiclaw's
// "load the matching skill file when needed" pattern.
func BuildSkillCatalogPrompt() string {
	skills := LoadSkills()
	if len(skills) == 0 {
		return ""
	}
	var sections []string
	sections = append(sections,
		"Workspace skills live under `workspace/skills/<skill-name>/SKILL.md`.",
		"When a task clearly matches a skill, read that skill file before acting.",
		"Resolve any relative paths mentioned inside a skill against that skill's directory.",
		"Available skills:",
	)
	for _, skill := range skills {
		name := strings.TrimSpace(skill.Name)
		desc := strings.TrimSpace(skill.Description)
		if name == "" {
			continue
		}
		if desc == "" {
			sections = append(sections, "- "+name)
			continue
		}
		sections = append(sections, "- "+name+": "+desc)
	}
	return strings.Join(sections, "\n")
}

func loadLocalSkills() []Skill {
	var skills []Skill
	for _, root := range localSkillRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillPath := filepath.Join(root, entry.Name(), "SKILL.md")
			data, readErr := os.ReadFile(skillPath)
			if readErr != nil {
				continue
			}
			skill, ok := parseSkill(string(data), root)
			if ok {
				skills = append(skills, skill)
			}
		}
	}
	return skills
}

func localSkillRoots() []string {
	var roots []string
	for _, root := range workspacepkg.WorkspaceRoots() {
		roots = append(roots, filepath.Join(root, "skills"))
	}
	return dedupeStrings(roots)
}

func parseSkill(content, source string) (Skill, bool) {
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return Skill{}, false
	}

	var skill Skill
	skill.Source = source
	header := strings.Split(parts[1], "\n")
	for _, line := range header {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "name:"):
			skill.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		case strings.HasPrefix(line, "description:"):
			skill.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		case strings.HasPrefix(line, "auto_use:"):
			skill.AutoUse = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "auto_use:")), "true")
		}
	}
	skill.Prompt = strings.TrimSpace(parts[2])
	if skill.Name == "" || skill.Prompt == "" {
		return Skill{}, false
	}
	return skill, true
}

func dedupeStrings(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
