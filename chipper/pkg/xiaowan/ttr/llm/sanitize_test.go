package llm

import "testing"

func TestStripLegacyCommandMarkupRemovesCompleteLegacyCommand(t *testing.T) {
	input := "好的，来放烟花庆祝！ " + "{{" + "celebrateFireworks||now}}"
	got := stripLegacyCommandMarkup(input)
	want := "好的，来放烟花庆祝！ "
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestStripLegacyCommandMarkupRemovesOrphanTail(t *testing.T) {
	input := "好的，来放烟花庆祝！ celebrateFireworks||now}}"
	got := stripLegacyCommandMarkup(input)
	want := "好的，来放烟花庆祝！ "
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestStripLegacyCommandMarkupRemovesBareLegacyCommand(t *testing.T) {
	input := "好的，来放烟花庆祝！ celebrateFireworks||now"
	got := stripLegacyCommandMarkup(input)
	want := "好的，来放烟花庆祝！ "
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
