package memory

import "testing"

func TestExtractExplicitMemoryFact(t *testing.T) {
	t.Run("remember home region", func(t *testing.T) {
		fact := extractExplicitMemoryFact("记住我家住东北")
		if fact != "用户家住东北" {
			t.Fatalf("expected remembered home fact, got %q", fact)
		}
	})

	t.Run("remember generic preference", func(t *testing.T) {
		fact := extractExplicitMemoryFact("你要记住我喜欢做饭")
		if fact != "用户喜欢做饭" {
			t.Fatalf("expected remembered generic fact, got %q", fact)
		}
	})
}

func TestExtractStableFacts(t *testing.T) {
	facts := extractStableFacts("我家住东北")
	if len(facts) != 1 || facts[0] != "用户家住东北" {
		t.Fatalf("expected stable home fact, got %#v", facts)
	}
}
