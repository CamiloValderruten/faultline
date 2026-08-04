package agent

import (
	"strings"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

func TestLastCollaboratorMessage(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "sys"},
		{Role: llm.RoleUser, Content: "[Collaborator message - Mon]\n\nYour collaborator says: first\n\nReply"},
		{Role: llm.RoleAssistant, Content: "ok"},
		{Role: llm.RoleUser, Content: "[Collaborator message - Tue]\n\nYour collaborator says: second\n\nReply"},
		{Role: llm.RoleTool, Content: "noise"},
	}
	got := lastCollaboratorMessage(messages)
	if !strings.Contains(got, "second") || strings.Contains(got, "first") {
		t.Fatalf("got %q", got)
	}
	if lastCollaboratorMessage(nil) != "" {
		t.Fatal("expected empty on nil")
	}
}

func TestCompactionRecoverySummary(t *testing.T) {
	got := compactionRecoverySummary("[Collaborator message]\n\nDo the expenses")
	if !strings.Contains(got, "overflowed") {
		t.Fatalf("missing overflow note: %q", got)
	}
	if !strings.Contains(got, "Do the expenses") {
		t.Fatalf("missing collaborator text: %q", got)
	}
	if !strings.Contains(compactionRecoverySummary(""), "overflowed") {
		t.Fatal("empty collab should still produce recovery text")
	}
}

func TestShrinkOversizedToolMessages(t *testing.T) {
	fat := strings.Repeat("x", 100_000)
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "sys"},
		{Role: llm.RoleUser, Content: "hi"},
		{Role: llm.RoleTool, ToolCallID: "1", Content: fat},
		{Role: llm.RoleTool, ToolCallID: "2", Content: strings.Repeat("y", 50_000)},
	}
	// Heuristic: chars/4. Target well under the fat payload.
	target := 5000
	n := shrinkOversizedToolMessages(messages, target, llm.EstimateMessagesTokens)
	if n == 0 {
		t.Fatal("expected at least one truncation")
	}
	if est := llm.EstimateMessagesTokens(messages); est > target {
		t.Fatalf("tokens_est=%d still above target %d after %d shrinks", est, target, n)
	}
	for _, m := range messages {
		if m.Role == llm.RoleTool && !strings.Contains(m.Content, "[truncated during compaction") {
			// Small messages may be untouched; only require the large ones changed.
			if len(m.Content) > 20_000 {
				t.Fatalf("large tool message not truncated: len=%d", len(m.Content))
			}
		}
	}
}
