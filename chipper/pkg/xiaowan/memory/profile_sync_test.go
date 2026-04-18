package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

func TestEnsureWorkspaceMemoryDocRestoresMemoryDocFromJSON(t *testing.T) {
	tmp := t.TempDir()
	oldConfigPath := vars.ApiConfigPath
	oldWirepodHome := os.Getenv("WIREPOD_HOME")
	t.Cleanup(func() {
		vars.ApiConfigPath = oldConfigPath
		_ = os.Setenv("WIREPOD_HOME", oldWirepodHome)
	})
	vars.ApiConfigPath = filepath.Join(tmp, "config.json")
	if err := os.Setenv("WIREPOD_HOME", tmp); err != nil {
		t.Fatal(err)
	}

	workspaceDir := filepath.Join(tmp, "workspace", "memory")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "MEMORY.md"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	SaveEditableProfile(EditableProfile{
		ESN:        "0dd1c497",
		MemoryText: "- The user lives in northeast China.",
	})

	if err := os.WriteFile(filepath.Join(workspaceDir, "MEMORY.md"), []byte("stale again"), 0o644); err != nil {
		t.Fatal(err)
	}

	EnsureWorkspaceMemoryDoc("0dd1c497")

	data, err := os.ReadFile(filepath.Join(workspaceDir, "MEMORY.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "The user lives in northeast China.") {
		t.Fatalf("expected synced memory doc to contain saved memory, got:\n%s", got)
	}
	if !strings.Contains(got, "## Durable Remembered Context") {
		t.Fatalf("expected synced memory doc structure, got:\n%s", got)
	}
}
