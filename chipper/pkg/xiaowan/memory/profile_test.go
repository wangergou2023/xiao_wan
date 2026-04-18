package memory

import (
	"os"
	"strings"
	"testing"
)

func TestBuildMemoryDocMatchesFreeformLongTermMemoryFormat(t *testing.T) {
	doc := buildMemoryDoc(UserProfile{
		ESN:        "0dd1c497",
		MemoryText: "- The user prefers to be called Wang Er Gou.",
	})

	checks := []string{
		"# Long-term Memory",
		"## How To Read This File",
		"## Durable Remembered Context",
		"- The user prefers to be called Wang Er Gou.",
		"## Memory Writing Guidance",
		"## Sync Info",
		"- Robot ESN: 0dd1c497",
		"- Source of truth: workspace/memory/MEMORY.md",
	}
	for _, want := range checks {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected memory doc to contain %q, got:\n%s", want, doc)
		}
	}
}

func TestBuildMemoryDocUsesEmptyStateWhenNoDurableMemory(t *testing.T) {
	doc := buildMemoryDoc(UserProfile{ESN: "0dd1c497"})
	if !strings.Contains(doc, memoryDocEmptyText) {
		t.Fatalf("expected empty memory doc to contain %q, got:\n%s", memoryDocEmptyText, doc)
	}
}

func TestExtractDurableRememberedContext(t *testing.T) {
	doc := buildMemoryDoc(UserProfile{
		ESN:        "0dd1c497",
		MemoryText: "- 用户是小丸的主人\n- 用户喜欢吃苹果",
	})
	got := extractDurableRememberedContext(doc)
	want := "- 用户是小丸的主人\n- 用户喜欢吃苹果"
	if got != want {
		t.Fatalf("unexpected remembered context\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestSaveAndLoadEditableProfileUseWorkspaceMemoryDocAsSingleSource(t *testing.T) {
	tmp := t.TempDir()
	oldWirepodHome := os.Getenv("WIREPOD_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("WIREPOD_HOME", oldWirepodHome)
	})
	if err := os.Setenv("WIREPOD_HOME", tmp); err != nil {
		t.Fatal(err)
	}

	SaveEditableProfile(EditableProfile{
		ESN:        "0dd1c497",
		MemoryText: "- 用户是小丸的主人\n- 用户喜欢吃苹果",
	})

	path := memoryDocPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "用户喜欢吃苹果") {
		t.Fatalf("expected memory doc to contain saved memory, got:\n%s", string(data))
	}

	editable := LoadEditableProfile("0dd1c497")
	if editable.MemoryText != "- 用户是小丸的主人\n- 用户喜欢吃苹果" {
		t.Fatalf("unexpected loaded memory text: %q", editable.MemoryText)
	}
}
