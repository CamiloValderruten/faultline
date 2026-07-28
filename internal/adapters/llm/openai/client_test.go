package openai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestThinkingWireOmittedWhenEmpty(t *testing.T) {
	b, err := json.Marshal(chatRequestWire{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "thinking") {
		t.Fatalf("empty thinking should be omitted: %s", b)
	}
}

func TestThinkingWireDisabled(t *testing.T) {
	b, err := json.Marshal(chatRequestWire{
		Model:    "m",
		Thinking: &thinkingWire{Type: "disabled"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `"thinking":{"type":"disabled"}`
	if !strings.Contains(string(b), want) {
		t.Fatalf("got %s, want substring %s", b, want)
	}
}
