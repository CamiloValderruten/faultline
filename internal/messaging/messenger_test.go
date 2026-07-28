package messaging

import (
	"strings"
	"testing"
)

func TestFlattenText(t *testing.T) {
	got := FlattenText(RichMessage{
		Title:   "Daily",
		Content: "Hello",
		Fields:  []RichField{{Name: "Status", Value: "ok"}},
		Selects: []SelectMenu{{
			Placeholder: "Pick",
			Options:     []SelectOption{{Label: "A", Value: "a", Description: "first"}},
		}},
	})
	for _, want := range []string{"**Daily**", "Hello", "**Status**", "ok", "*Pick:*", "- A", "`a`", "first"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FlattenText missing %q in %q", want, got)
		}
	}
}
