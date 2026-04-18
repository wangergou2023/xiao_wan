package memory

import (
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
		"- Source of truth: confirmed freeform memory saved by the system",
	}
	for _, want := range checks {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected memory doc to contain %q, got:\n%s", want, doc)
		}
	}
}

func TestBuildMemoryDocUsesEmptyStateWhenNoDurableMemory(t *testing.T) {
	doc := buildMemoryDoc(UserProfile{ESN: "0dd1c497"})
	want := "No durable user facts have been confirmed yet."
	if !strings.Contains(doc, want) {
		t.Fatalf("expected empty memory doc to contain %q, got:\n%s", want, doc)
	}
}
