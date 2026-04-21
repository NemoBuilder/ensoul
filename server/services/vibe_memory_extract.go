package services

import (
	"regexp"
	"strings"

	"github.com/ensoul-labs/ensoul-server/models"
)

// MemorySuggestion is a parsed `:::memory-suggest` block produced by the LLM.
type MemorySuggestion struct {
	Category string `json:"category"`
	Content  string `json:"content"`
	Reason   string `json:"reason"`
}

// memorySuggestBlockRe matches a fenced :::memory-suggest ... ::: block.
// Multi-line, ungreedy.
var memorySuggestBlockRe = regexp.MustCompile(`(?s):::memory-suggest\s*\n?(.*?):::`)

// memorySuggestFieldRe extracts "key: value" lines (value may be on a single
// line; trailing newlines/blank lines are ignored).
var memorySuggestFieldRe = regexp.MustCompile(`(?m)^\s*(category|content|reason)\s*:\s*(.+?)\s*$`)

// ExtractMemorySuggestions parses all `:::memory-suggest` blocks from an
// assistant message and returns the cleaned message (with blocks removed)
// plus the parsed suggestions. Invalid blocks (missing category/content) are
// dropped silently.
func ExtractMemorySuggestions(content string) (cleaned string, suggestions []MemorySuggestion) {
	matches := memorySuggestBlockRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	for _, m := range matches {
		body := content[m[2]:m[3]]
		fields := map[string]string{}
		for _, fm := range memorySuggestFieldRe.FindAllStringSubmatch(body, -1) {
			fields[strings.ToLower(fm[1])] = strings.TrimSpace(fm[2])
		}
		cat := fields["category"]
		text := fields["content"]
		if cat == "" || text == "" {
			continue
		}
		if !validMemoryCategory(cat) {
			continue
		}
		suggestions = append(suggestions, MemorySuggestion{
			Category: cat,
			Content:  text,
			Reason:   fields["reason"],
		})
	}

	cleaned = memorySuggestBlockRe.ReplaceAllString(content, "")
	cleaned = strings.TrimRight(cleaned, " \t\n")
	return cleaned, suggestions
}

func validMemoryCategory(c string) bool {
	switch c {
	case models.MemoryCategoryProfile,
		models.MemoryCategoryKnowledge,
		models.MemoryCategoryNetwork,
		models.MemoryCategoryArchive,
		models.MemoryCategoryRules:
		return true
	}
	return false
}
