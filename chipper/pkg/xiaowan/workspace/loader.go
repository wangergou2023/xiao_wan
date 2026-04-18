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
// AGENT.md / SOUL.md / USER.md / memory/MEMORY.md。
// 这里先保持为简单的“多候选路径 + 首个命中”策略，方便在开发与打包环境下共用。
func LoadDocs() Docs {
	return Docs{
		Agent:  readFirstExistingFile("AGENT.md"),
		Soul:   readFirstExistingFile("SOUL.md"),
		User:   readFirstExistingFile("USER.md"),
		Memory: readFirstExistingFile(filepath.Join("memory", "MEMORY.md")),
	}
}

func BuildPromptContext() string {
	docs := LoadDocs()
	var sections []string
	if strings.TrimSpace(docs.Agent) != "" {
		sections = append(sections, "AGENT.md:\n"+strings.TrimSpace(docs.Agent))
	}
	if strings.TrimSpace(docs.Soul) != "" {
		sections = append(sections, "SOUL.md:\n"+strings.TrimSpace(docs.Soul))
	}
	if strings.TrimSpace(docs.User) != "" {
		sections = append(sections, "USER.md:\n"+strings.TrimSpace(docs.User))
	}
	if strings.TrimSpace(docs.Memory) != "" {
		sections = append(sections, "MEMORY.md:\n"+strings.TrimSpace(docs.Memory))
	}
	return strings.Join(sections, "\n\n")
}

func readFirstExistingFile(rel string) string {
	for _, root := range workspaceRoots() {
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
	for _, root := range workspaceRoots() {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	roots := workspaceRoots()
	if len(roots) == 0 {
		return rel
	}
	return filepath.Join(roots[0], rel)
}

func workspaceRoots() []string {
	var roots []string
	if wirepodHome := strings.TrimSpace(os.Getenv("WIREPOD_HOME")); wirepodHome != "" {
		roots = append(roots,
			filepath.Join(wirepodHome, "workspace"),
			filepath.Join(wirepodHome, "chipper", "workspace"),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots,
			filepath.Join(wd, "workspace"),
			filepath.Join(wd, "chipper", "workspace"),
			filepath.Join(filepath.Dir(wd), "workspace"),
			filepath.Join(filepath.Dir(wd), "chipper", "workspace"),
		)
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
