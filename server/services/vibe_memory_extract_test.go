package services

import (
	"strings"
	"testing"
)

func TestExtractMemorySuggestions_Basic(t *testing.T) {
	in := `Here is your tweet draft.

:::memory-suggest
category: rules
content: User prefers no emojis in tweets
reason: User explicitly asked to avoid emojis
:::

Hope this helps!`
	cleaned, sugs := ExtractMemorySuggestions(in)
	if len(sugs) != 1 {
		t.Fatalf("want 1 suggestion, got %d", len(sugs))
	}
	if sugs[0].Category != "rules" {
		t.Errorf("category=%s", sugs[0].Category)
	}
	if !strings.Contains(sugs[0].Content, "no emojis") {
		t.Errorf("content=%s", sugs[0].Content)
	}
	if strings.Contains(cleaned, ":::memory-suggest") {
		t.Errorf("cleaned still contains block: %s", cleaned)
	}
}

func TestExtractMemorySuggestions_Multiple(t *testing.T) {
	in := `text
:::memory-suggest
category: profile
content: User is a Web3 founder
:::
mid
:::memory-suggest
category: knowledge
content: User cares about restaking
reason: mentioned EigenLayer twice
:::
end`
	_, sugs := ExtractMemorySuggestions(in)
	if len(sugs) != 2 {
		t.Fatalf("want 2, got %d", len(sugs))
	}
}

func TestExtractMemorySuggestions_InvalidCategory(t *testing.T) {
	in := `:::memory-suggest
category: bogus
content: x
:::`
	_, sugs := ExtractMemorySuggestions(in)
	if len(sugs) != 0 {
		t.Fatalf("invalid category should be dropped, got %d", len(sugs))
	}
}

func TestExtractMemorySuggestions_None(t *testing.T) {
	cleaned, sugs := ExtractMemorySuggestions("plain message")
	if len(sugs) != 0 || cleaned != "plain message" {
		t.Fatalf("unexpected: %q %v", cleaned, sugs)
	}
}
