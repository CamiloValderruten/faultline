package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

func TestToolDefsTier1ExcludesWikiAndEmail(t *testing.T) {
	te := New(Deps{Logger: silentTestLogger()})
	names := toolDefNames(te.ToolDefs())
	if !names["search_available_tools"] {
		t.Fatal("expected search_available_tools in Tier 1")
	}
	if !names["web_fetch"] {
		t.Fatal("expected web_fetch in Tier 1")
	}
	if names["wiki_fetch"] {
		t.Fatal("wiki_fetch should be Tier 2")
	}
	if names["memory_delete"] {
		t.Fatal("memory_delete should be Tier 2")
	}
	if !toolDefNames(te.buildAllToolDefs())["wiki_fetch"] {
		t.Fatal("wiki_fetch should still be in the full registry")
	}
}

func TestSearchAvailableToolsUnlocksTier2(t *testing.T) {
	te := New(Deps{Logger: silentTestLogger()})
	if toolDefNames(te.ToolDefs())["wiki_fetch"] {
		t.Fatal("wiki_fetch should start locked")
	}

	got := te.Execute(context.Background(), llm.ToolCall{
		Function: llm.FunctionCall{
			Name:      "search_available_tools",
			Arguments: `{"query":"wikipedia article","max_results":5}`,
		},
	})
	if strings.HasPrefix(got, "Error:") {
		t.Fatalf("search failed: %s", got)
	}
	var hits []toolSearchHit
	if err := json.Unmarshal([]byte(got), &hits); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, got)
	}
	found := false
	for _, h := range hits {
		if h.Name == "wiki_fetch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected wiki_fetch in search hits, got %s", got)
	}
	if !toolDefNames(te.ToolDefs())["wiki_fetch"] {
		t.Fatal("expected wiki_fetch unlocked into ToolDefs after search")
	}
}

func TestSearchAvailableToolsExactName(t *testing.T) {
	te := New(Deps{Logger: silentTestLogger()})
	got := te.Execute(context.Background(), llm.ToolCall{
		Function: llm.FunctionCall{
			Name:      "search_available_tools",
			Arguments: `{"query":"memory_delete"}`,
		},
	})
	if !strings.Contains(got, `"memory_delete"`) {
		t.Fatalf("exact name search missed memory_delete: %s", got)
	}
}
