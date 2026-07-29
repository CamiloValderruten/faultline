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

func TestFuseToolSearchRanksWeightsAndExactBoost(t *testing.T) {
	// Vector ranks "wiki_fetch" first; BM25 ranks "memory_delete" first.
	got := fuseToolSearchRanks(
		[]string{"memory_delete", "wiki_fetch"},
		[]string{"wiki_fetch", "memory_delete"},
		5,
		"",
	)
	if len(got) < 2 || got[0] != "wiki_fetch" {
		t.Fatalf("expected semantic weight to prefer wiki_fetch, got %v", got)
	}

	boosted := fuseToolSearchRanks(
		[]string{"memory_delete", "wiki_fetch"},
		[]string{"wiki_fetch", "memory_delete"},
		5,
		"memory_delete",
	)
	if len(boosted) < 1 || boosted[0] != "memory_delete" {
		t.Fatalf("exact-name boost should lift memory_delete, got %v", boosted)
	}
}

func TestSearchAvailableToolsUsesEmbedder(t *testing.T) {
	// Make wiki_fetch's embedding uniquely match a paraphrased query by
	// giving it a distinctive document length fingerprint in fakeEmbedder
	// (first dim = len(text)+1). Use a query string whose length matches
	// wiki_fetch's search document more closely than others... that's
	// fragile. Instead: verify ensureToolVectors populates the index and
	// that a search with embedder still returns wiki_fetch for "wikipedia".
	emb := &fakeEmbedder{dim: 8, model: "test-emb"}
	te := New(Deps{Logger: silentTestLogger(), Embedder: emb})

	got := te.Execute(context.Background(), llm.ToolCall{
		Function: llm.FunctionCall{
			Name:      "search_available_tools",
			Arguments: `{"query":"wikipedia article","max_results":5}`,
		},
	})
	if strings.HasPrefix(got, "Error:") {
		t.Fatalf("search failed: %s", got)
	}
	if !strings.Contains(got, `"wiki_fetch"`) {
		t.Fatalf("expected wiki_fetch with hybrid search, got %s", got)
	}
	if emb.Dim() != 8 {
		t.Fatal("embedder should be wired")
	}
	te.catalog.mu.Lock()
	defer te.catalog.mu.Unlock()
	if te.catalog.vectors == nil || te.catalog.vectors.Len() == 0 {
		t.Fatal("expected tool vector index to be built")
	}
	if len(emb.calls) == 0 {
		t.Fatal("expected embedder to be called for catalog and/or query")
	}
}
