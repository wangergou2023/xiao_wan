package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

type Docs struct {
	Agents    string
	Identity  string
	Bootstrap string
	Soul      string
	User      string
	Memory    string
	Todo      string
}

// LoadDocs 读取 workspace 文档层。
// 优先兼容 mimiclaw 风格的 AGENTS.md / IDENTITY.md / BOOTSTRAP.md，
// 同时兼容旧的 AGENT.md。
func LoadDocs() Docs {
	return Docs{
		Agents:    readWorkspaceFileFirst("AGENTS.md", "AGENT.md"),
		Identity:  readWorkspaceFile("IDENTITY.md"),
		Bootstrap: readWorkspaceFile("BOOTSTRAP.md"),
		Soul:      readWorkspaceFile("SOUL.md"),
		User:      readWorkspaceFile("USER.md"),
		Memory:    readWorkspaceFile(filepath.Join("memory", "MEMORY.md")),
		Todo:      readWorkspaceFile("TODO.md"),
	}
}

func BuildPromptContext() string {
	docs := LoadDocs()
	var sections []string
	if strings.TrimSpace(docs.Agents) != "" {
		sections = append(sections, "workspace/AGENTS.md:\n"+strings.TrimSpace(docs.Agents))
	}
	if strings.TrimSpace(docs.Identity) != "" {
		sections = append(sections, "workspace/IDENTITY.md:\n"+strings.TrimSpace(docs.Identity))
	}
	if strings.TrimSpace(docs.Bootstrap) != "" {
		sections = append(sections, "workspace/BOOTSTRAP.md:\n"+strings.TrimSpace(docs.Bootstrap))
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
	if strings.TrimSpace(docs.Todo) != "" {
		sections = append(sections, "workspace/TODO.md:\n"+strings.TrimSpace(docs.Todo))
	} else if todoPrompt := strings.TrimSpace(BuildTodoPromptContext()); todoPrompt != "" {
		sections = append(sections, todoPrompt)
	}
	return strings.Join(sections, "\n\n")
}

func readWorkspaceFile(rel string) string {
	return readWorkspaceFileFirst(rel)
}

func readWorkspaceFileFirst(rel ...string) string {
	for _, root := range WorkspaceRoots() {
		for _, one := range rel {
			path := filepath.Join(root, one)
			data, err := os.ReadFile(path)
			if err == nil {
				return string(data)
			}
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
		if wd, err := os.Getwd(); err == nil {
			base := filepath.Base(wd)
			switch base {
			case "workspace":
				return filepath.Join(wd, rel)
			case "chipper":
				return filepath.Join(wd, "workspace", rel)
			default:
				return filepath.Join(wd, "workspace", rel)
			}
		}
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
