package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/CamiloValderruten/faultline/internal/llm"
	"github.com/CamiloValderruten/faultline/internal/search/bm25"
	"github.com/CamiloValderruten/faultline/internal/search/vector"
)

// tier1Tools are always advertised in ToolDefs. Everything else is Tier 2:
// discoverable via search_available_tools, then unlocked into ToolDefs.
//
// wiki_fetch and email_fetch are intentionally Tier 2 (email off by default;
// wiki is infrequent relative to web_fetch).
var tier1Tools = map[string]struct{}{
	"search_available_tools": {},

	"send_message":       {},
	"send_rich_message":  {},
	"send_voice_message": {},
	"send_file":          {},

	"memory_read":   {},
	"memory_write":  {},
	"memory_list":   {},
	"memory_search": {},
	"memory_edit":   {},

	"get_time":       {},
	"sleep":          {},
	"context_status": {},

	"mcp_discover_tools": {},
	"mcp_list_servers":   {},

	"web_fetch": {},

	"sandbox_execute": {},
	"sandbox_write":   {},
	"sandbox_read":    {},
	"sandbox_list":    {},

	"daemon_spawn": {},
	"daemon_list":  {},
	"daemon_fetch": {},
	"daemon_stop":  {},

	"schedule_task":         {},
	"list_scheduled_tasks":  {},
	"cancel_scheduled_task": {},

	"peer_send":  {},
	"peer_inbox": {},
	"peer_read":  {},
}

const (
	toolSearchRRFK           = 60
	toolSearchSemanticWeight = 0.7
	toolSearchKeywordWeight  = 0.3
	toolSearchExactNameBoost = 1.5
)

type toolCatalog struct {
	mu        sync.Mutex
	byName    map[string]llm.Tool
	docs      map[string]string // tier-2 search documents
	search    *bm25.Index
	vectors   *vector.Index
	unlocked  map[string]struct{}
	signature string // joined tier-2 names; skip BM25 rebuild when unchanged
	vectorSig string // signature last successfully embedded
}

func newToolCatalog() *toolCatalog {
	return &toolCatalog{
		byName:   make(map[string]llm.Tool),
		docs:     make(map[string]string),
		search:   bm25.New(),
		unlocked: make(map[string]struct{}),
	}
}

func (te *Executor) searchAvailableToolsDef() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: &llm.FunctionDef{
			Name: "search_available_tools",
			Description: "Search the long-tail specialty tools by semantic intent or keyword " +
				"(hybrid BM25 + embeddings when configured). " +
				"Returns matching tools with full schemas and unlocks them for subsequent calls. " +
				"Use when you suspect a capability exists but it is not in your current tool list " +
				"(e.g. trash restore, sandbox shell, subagents, wiki, email, skills, updates).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Free-text query. Examples: 'restore deleted memory', 'run shell in sandbox', 'wikipedia', 'spawn subagent', 'install skill'.",
					},
					"max_results": map[string]interface{}{
						"type":        "integer",
						"description": "Max tools to return. Defaults to 5.",
					},
					"include_disallowed": map[string]interface{}{
						"type":        "boolean",
						"description": "Reserved; Faultline has no separate disallowed built-in set today. Defaults to false.",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

// applyToolTiers keeps Tier 1 + previously unlocked Tier 2 in the
// advertised set. Refreshes the searchable Tier 2 catalog from `all`.
func (te *Executor) applyToolTiers(all []llm.Tool) []llm.Tool {
	if te.catalog == nil {
		te.catalog = newToolCatalog()
	}
	te.catalog.refresh(all)

	out := make([]llm.Tool, 0, 32)
	seen := make(map[string]struct{}, 32)
	add := func(t llm.Tool) {
		if t.Function == nil || t.Function.Name == "" {
			return
		}
		name := t.Function.Name
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, t)
	}

	add(te.searchAvailableToolsDef())
	for _, t := range all {
		if t.Function == nil {
			continue
		}
		name := t.Function.Name
		if name == "search_available_tools" {
			continue
		}
		if _, ok := tier1Tools[name]; ok {
			add(t)
			continue
		}
		if te.catalog.isUnlocked(name) {
			add(t)
		}
	}
	return out
}

func (c *toolCatalog) refresh(all []llm.Tool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	byName := make(map[string]llm.Tool, len(all))
	docs := make(map[string]string)
	var names []string
	for _, t := range all {
		if t.Function == nil || t.Function.Name == "" {
			continue
		}
		name := t.Function.Name
		byName[name] = t
		if _, tier1 := tier1Tools[name]; tier1 || name == "search_available_tools" {
			continue
		}
		names = append(names, name)
		docs[name] = toolSearchDocument(t)
	}
	sort.Strings(names)
	sig := strings.Join(names, "\n")
	c.byName = byName
	c.docs = docs
	if sig == c.signature {
		return
	}
	c.signature = sig
	c.search.Build(docs)
	// Vector index rebuilt lazily on next search (needs embedder + ctx).
	c.vectors = nil
	c.vectorSig = ""
}

func (c *toolCatalog) isUnlocked(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.unlocked[name]
	return ok
}

func (c *toolCatalog) unlock(names []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, n := range names {
		c.unlocked[n] = struct{}{}
	}
}

func toolParamsMap(params any) map[string]interface{} {
	if params == nil {
		return nil
	}
	if m, ok := params.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func toolSearchDocument(t llm.Tool) string {
	fn := t.Function
	var b strings.Builder
	b.WriteString(fn.Name)
	b.WriteString(" — ")
	b.WriteString(fn.Description)
	if schema := toolParamsMap(fn.Parameters); schema != nil {
		if props, ok := schema["properties"].(map[string]interface{}); ok {
			keys := make([]string, 0, len(props))
			for k := range props {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				b.WriteString(" params: ")
				b.WriteString(strings.Join(keys, ", "))
			}
		}
	}
	b.WriteString(" category: ")
	b.WriteString(toolCategory(fn.Name))
	return b.String()
}

func toolCategory(name string) string {
	switch {
	case strings.HasPrefix(name, "memory_"):
		return "memory"
	case strings.HasPrefix(name, "sandbox_"):
		return "sandbox"
	case strings.HasPrefix(name, "mcp_"):
		return "mcp"
	case strings.HasPrefix(name, "subagent_"):
		return "subagent"
	case strings.HasPrefix(name, "peer_"):
		return "peer"
	case strings.HasPrefix(name, "skill_"):
		return "skills"
	case strings.HasPrefix(name, "update_"):
		return "update"
	case strings.HasPrefix(name, "schedule_") || name == "list_scheduled_tasks" || name == "cancel_scheduled_task":
		return "schedule"
	case name == "wiki_fetch" || name == "web_fetch":
		return "web"
	case name == "email_fetch":
		return "email"
	case name == "get_version" || name == "rebuild_indexes":
		return "system"
	default:
		return "other"
	}
}

func toolExampleCall(t llm.Tool) string {
	fn := t.Function
	args := map[string]interface{}{}
	if schema := toolParamsMap(fn.Parameters); schema != nil {
		props, _ := schema["properties"].(map[string]interface{})
		var required []string
		if req, ok := schema["required"].([]string); ok {
			required = req
		} else if reqAny, ok := schema["required"].([]interface{}); ok {
			for _, r := range reqAny {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}
		}
		for _, key := range required {
			args[key] = "<" + key + ">"
		}
		if len(args) == 0 && props != nil {
			keys := make([]string, 0, len(props))
			for k := range props {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for i, k := range keys {
				if i >= 2 {
					break
				}
				args[k] = "<" + k + ">"
			}
		}
	}
	raw, err := json.Marshal(args)
	if err != nil {
		raw = []byte("{}")
	}
	return fmt.Sprintf(`{"name":%q,"arguments":%s}`, fn.Name, string(raw))
}

type toolSearchHit struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
	AllowStatus string `json:"allow_status"`
	Category    string `json:"category"`
	ExampleCall string `json:"example_call"`
}

func (te *Executor) searchAvailableTools(ctx context.Context, argsJSON string) string {
	if te.catalog == nil {
		te.catalog = newToolCatalog()
	}
	te.catalog.refresh(te.buildAllToolDefs())

	var args struct {
		Query             string `json:"query"`
		MaxResults        int    `json:"max_results"`
		IncludeDisallowed bool   `json:"include_disallowed"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "Error: invalid arguments: " + err.Error()
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "Error: query is required"
	}
	max := args.MaxResults
	if max <= 0 {
		max = 5
	}
	if max > 25 {
		max = 25
	}
	_ = args.IncludeDisallowed // ponytail: no separate disallowed built-in index yet

	// Over-fetch each leg so RRF has room to reorder.
	fetch := max * 3
	if fetch < 15 {
		fetch = 15
	}

	te.catalog.mu.Lock()
	bm25Hits := te.catalog.search.Search(query, fetch, nil)
	te.catalog.mu.Unlock()

	var vectorPaths []string
	if err := te.ensureToolVectors(ctx); err != nil {
		te.logger.Warn("search_available_tools: embed catalog failed; lexical only", "error", err)
	} else if te.embedder != nil && te.embedder.Dim() > 0 {
		vecs, err := te.embedder.Embed(ctx, []string{query})
		if err != nil {
			te.logger.Warn("search_available_tools: embed query failed; lexical only", "error", err)
		} else if len(vecs) == 1 {
			te.catalog.mu.Lock()
			vIdx := te.catalog.vectors
			te.catalog.mu.Unlock()
			if vIdx != nil && vIdx.Len() > 0 {
				q := append([]float32(nil), vecs[0]...)
				vres, err := vIdx.Search(q, fetch, 0, nil)
				if err != nil {
					te.logger.Warn("search_available_tools: vector search failed; lexical only", "error", err)
				} else {
					for _, r := range vres {
						vectorPaths = append(vectorPaths, r.Path)
					}
				}
			}
		}
	}

	bm25Paths := make([]string, 0, len(bm25Hits))
	for _, r := range bm25Hits {
		bm25Paths = append(bm25Paths, r.Path)
	}
	ranked := fuseToolSearchRanks(bm25Paths, vectorPaths, max, strings.ToLower(query))

	// Exact-name force-include if still missing (covers empty-index edge cases).
	qLower := strings.ToLower(query)
	te.catalog.mu.Lock()
	if _, ok := te.catalog.byName[qLower]; ok {
		if _, tier1 := tier1Tools[qLower]; !tier1 && qLower != "search_available_tools" {
			found := false
			for _, name := range ranked {
				if name == qLower {
					found = true
					break
				}
			}
			if !found {
				ranked = append([]string{qLower}, ranked...)
				if len(ranked) > max {
					ranked = ranked[:max]
				}
			}
		}
	}
	hits := make([]toolSearchHit, 0, len(ranked))
	unlock := make([]string, 0, len(ranked))
	for _, name := range ranked {
		t, ok := te.catalog.byName[name]
		if !ok || t.Function == nil {
			continue
		}
		unlock = append(unlock, name)
		hits = append(hits, toolSearchHit{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
			AllowStatus: "allowed",
			Category:    toolCategory(t.Function.Name),
			ExampleCall: toolExampleCall(t),
		})
	}
	te.catalog.mu.Unlock()
	te.catalog.unlock(unlock)

	if len(hits) == 0 {
		return "[]"
	}
	data, err := json.MarshalIndent(hits, "", "  ")
	if err != nil {
		return "Error: " + err.Error()
	}
	return string(data)
}

// ensureToolVectors embeds Tier 2 tool docs into an in-memory vector
// index when an embedder is configured. No-op when embeddings are off
// or Dim is still 0 (probe not finished). Rebuilds only when the
// lexical catalog signature changes.
func (te *Executor) ensureToolVectors(ctx context.Context) error {
	if te.embedder == nil || te.embedder.Dim() <= 0 {
		return nil
	}
	if te.catalog == nil {
		return nil
	}

	te.catalog.mu.Lock()
	if te.catalog.signature == te.catalog.vectorSig && te.catalog.vectors != nil {
		te.catalog.mu.Unlock()
		return nil
	}
	sig := te.catalog.signature
	docs := te.catalog.docs
	te.catalog.mu.Unlock()

	if len(docs) == 0 {
		te.catalog.mu.Lock()
		te.catalog.vectors = vector.New(te.embedder.Dim(), te.embedder.Model())
		te.catalog.vectorSig = sig
		te.catalog.mu.Unlock()
		return nil
	}

	names := make([]string, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	sort.Strings(names)
	texts := make([]string, len(names))
	for i, name := range names {
		texts[i] = docs[name]
	}

	vecs, skipped, err := embedWithAdaptiveBatching(ctx, te.embedder, texts, 0, te.logger)
	if err != nil {
		return err
	}
	if skipped > 0 {
		te.logger.Warn("search_available_tools: skipped tool embeddings", "skipped", skipped)
	}

	idx := vector.New(te.embedder.Dim(), te.embedder.Model())
	for i, name := range names {
		if i >= len(vecs) || vecs[i] == nil {
			continue
		}
		if err := idx.Upsert(name, vecs[i]); err != nil {
			return err
		}
	}

	te.catalog.mu.Lock()
	defer te.catalog.mu.Unlock()
	// Only publish if the catalog hasn't changed under us.
	if te.catalog.signature != sig {
		return nil
	}
	te.catalog.vectors = idx
	te.catalog.vectorSig = sig
	return nil
}

// fuseToolSearchRanks combines BM25 and vector rankings with weighted
// reciprocal rank fusion (0.7 semantic + 0.3 keyword). Exact tool-name
// matches get a 1.5x score boost.
func fuseToolSearchRanks(bm25Paths, vectorPaths []string, max int, exactName string) []string {
	type scored struct {
		name  string
		score float64
	}
	scores := make(map[string]float64)

	add := func(paths []string, weight float64) {
		for i, name := range paths {
			if name == "" {
				continue
			}
			scores[name] += weight / float64(toolSearchRRFK+i+1)
		}
	}
	add(vectorPaths, toolSearchSemanticWeight)
	add(bm25Paths, toolSearchKeywordWeight)

	if exactName != "" {
		if s, ok := scores[exactName]; ok {
			scores[exactName] = s * toolSearchExactNameBoost
		}
	}

	ranked := make([]scored, 0, len(scores))
	for name, score := range scores {
		ranked = append(ranked, scored{name: name, score: score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].name < ranked[j].name
		}
		return ranked[i].score > ranked[j].score
	})
	if max > 0 && len(ranked) > max {
		ranked = ranked[:max]
	}
	out := make([]string, len(ranked))
	for i, r := range ranked {
		out[i] = r.name
	}
	return out
}
