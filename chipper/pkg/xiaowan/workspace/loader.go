package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

type Docs struct {
	Agent  string
	Soul   string
	User   string
	Memory string
}

// LoadDocs 读取 PicoClaw 风格的 workspace 文档层：
// workspace/AGENT.md / workspace/SOUL.md / workspace/USER.md /
// workspace/memory/MEMORY.md。
func LoadDocs() Docs {
	return Docs{
		Agent:  readWorkspaceFile("AGENT.md"),
		Soul:   readWorkspaceFile("SOUL.md"),
		User:   readWorkspaceFile("USER.md"),
		Memory: readWorkspaceFile(filepath.Join("memory", "MEMORY.md")),
	}
}

func BuildPromptContext() string {
	docs := LoadDocs()
	var sections []string
	if strings.TrimSpace(docs.Agent) != "" {
		sections = append(sections, "workspace/AGENT.md:\n"+strings.TrimSpace(docs.Agent))
	}
	if strings.TrimSpace(docs.Soul) != "" {
		sections = append(sections, "workspace/SOUL.md:\n"+strings.TrimSpace(docs.Soul))
	}
	if strings.TrimSpace(docs.User) != "" {
		sections = append(sections, "workspace/USER.md:\n"+strings.TrimSpace(docs.User))
	}
	if strings.TrimSpace(docs.Memory) != "" {
		sections = append(sections, "workspace/memory/MEMORY.md:\n"+strings.TrimSpace(docs.Memory))
	}
	return strings.Join(sections, "\n\n")
}

func readWorkspaceFile(rel string) string {
	for _, root := range WorkspaceRoots() {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

// ResolveWritableDocPath 返回某个 workspace 文档推荐写入的位置。
// 优先复用已存在文件；如果都不存在，就落到第一个候选根目录。
func ResolveWritableDocPath(rel string) string {
	for _, root := range WorkspaceRoots() {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	roots := WorkspaceRoots()
	if len(roots) == 0 {
		return rel
	}
	return filepath.Join(roots[0], rel)
}

// WorkspaceRoots returns candidate workspace roots in priority order.
func WorkspaceRoots() []string {
	var roots []string
	addIfWorkspace := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			roots = append(roots, path)
		}
	}
	if wirepodHome := strings.TrimSpace(os.Getenv("WIREPOD_HOME")); wirepodHome != "" {
		addIfWorkspace(filepath.Join(wirepodHome, "workspace"))
		addIfWorkspace(filepath.Join(wirepodHome, "chipper", "workspace"))
	}
	if wd, err := os.Getwd(); err == nil {
		base := filepath.Base(wd)
		switch base {
		case "workspace":
			addIfWorkspace(wd)
		case "chipper":
			addIfWorkspace(filepath.Join(wd, "workspace"))
		default:
			addIfWorkspace(filepath.Join(wd, "workspace"))
			addIfWorkspace(filepath.Join(wd, "chipper", "workspace"))
			addIfWorkspace(filepath.Join(filepath.Dir(wd), "workspace"))
			addIfWorkspace(filepath.Join(filepath.Dir(wd), "chipper", "workspace"))
		}
	}
	return dedupeStrings(roots)
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
