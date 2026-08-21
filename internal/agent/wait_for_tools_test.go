package agent

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/config"
	"github.com/CamiloValderruten/faultline/internal/daemon"
	"github.com/CamiloValderruten/faultline/internal/llm"
)

// afterFirstChatOperator returns a collaborator message only after Chat has
// been called at least once — simulating arrival during generation.
type afterFirstChatOperator struct {
	mu      sync.Mutex
	chat    *scriptedChat
	msg     string
	drained bool
}

func (o *afterFirstChatOperator) Pending() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.chat.mu.Lock()
	seen := len(o.chat.seen)
	o.chat.mu.Unlock()
	if seen == 0 || o.drained {
		return nil
	}
	o.drained = true
	return []string{o.msg}
}

func TestWaitForToolsFalseDefersPendingTools(t *testing.T) {
	chat := &scriptedChat{responses: []*llm.ChatResponse{
		toolCallResponse("skill_activate", `{"name":"finances"}`),
		textOnlyResponse("acked"),
	}}
	tools := &recordingTools{results: map[string]string{
		"skill_activate": "skill loaded",
	}}
	op := &afterFirstChatOperator{chat: chat, msg: "stop, do this instead"}
	agent := newDeliveryDebtAgent(chat, tools, op, 2)
	agent.cfg.Agent.WaitForTools = false

	if err := agent.Run(context.Background(), make(chan struct{})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if tools.execCount["skill_activate"] != 0 {
		t.Fatalf("skill_activate executions = %d, want 0 (deferred)", tools.execCount["skill_activate"])
	}
	var sawDeferred bool
	for _, m := range chat.seen {
		if m.Role == llm.RoleTool && strings.Contains(m.Content, "[Deferred]") {
			sawDeferred = true
		}
	}
	if !sawDeferred {
		t.Fatal("expected [Deferred] tool stub when wait_for_tools=false")
	}
}

func TestWaitForToolsTrueRunsToolsBeforeInject(t *testing.T) {
	chat := &scriptedChat{responses: []*llm.ChatResponse{
		toolCallResponse("skill_activate", `{"name":"finances"}`),
		toolCallResponse("send_message", `{"text":"done with finances, saw your follow-up"}`),
	}}
	tools := &recordingTools{results: map[string]string{
		"skill_activate": "skill loaded",
		"send_message":   "Message sent to collaborator.",
	}}
	op := &afterFirstChatOperator{chat: chat, msg: "also check July"}
	cfg := config.Default()
	cfg.Limits.RecentMemoryChars = 1024
	cfg.Agent.WaitForTools = true
	memory := newAgentTestMemory()
	memory.files["prompts/migrations.md"] = `# Prompt migrations applied

## Applied

- 000 add-untrusted-content-convention 2026-05-01T00:00:00Z
- 001 autonomy-prompts-v1 2026-05-01T00:00:00Z
- 002 prompt_hierarchy_v1 2026-05-01T00:00:00Z
`
	agent := New(cfg, Deps{
		Chat:     chat,
		Memory:   memory,
		Search:   noopSearcher{},
		Operator: op,
		Tools:    tools,
		State:    emptyStateStore{},
		MaxTurns: 2,
	}, newTestLogger())

	if err := agent.Run(context.Background(), make(chan struct{})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if tools.execCount["skill_activate"] != 1 {
		t.Fatalf("skill_activate executions = %d, want 1 (not deferred)", tools.execCount["skill_activate"])
	}
	for _, m := range chat.seen {
		if m.Role == llm.RoleTool && strings.Contains(m.Content, "[Deferred]") {
			t.Fatal("did not expect [Deferred] when wait_for_tools=true")
		}
	}
	var sawFollowUp bool
	for _, m := range chat.seen {
		if m.Role == llm.RoleUser && strings.Contains(m.Content, "also check July") {
			sawFollowUp = true
		}
	}
	if !sawFollowUp {
		t.Fatal("expected follow-up injected on a later turn")
	}
}

type afterFirstChatDaemons struct {
	mu      sync.Mutex
	chat    *scriptedChat
	alert   daemon.Alert
	drained bool
}

func (d *afterFirstChatDaemons) Pending() []daemon.Alert {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.chat.mu.Lock()
	seen := len(d.chat.seen)
	d.chat.mu.Unlock()
	if seen == 0 || d.drained {
		return nil
	}
	d.drained = true
	return []daemon.Alert{d.alert}
}

func (d *afterFirstChatDaemons) HasPending() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return !d.drained
}

func TestWaitForToolsTrueDefersToolsForDaemonAlert(t *testing.T) {
	chat := &scriptedChat{responses: []*llm.ChatResponse{
		toolCallResponse("skill_activate", `{"name":"finances"}`),
		textOnlyResponse("acked cry"),
	}}
	tools := &recordingTools{results: map[string]string{
		"skill_activate": "skill loaded",
	}}
	daemons := &afterFirstChatDaemons{chat: chat, alert: daemon.Alert{
		DaemonID: "d1",
		Name:     "baby",
		Text:     "cry",
	}}
	cfg := config.Default()
	cfg.Limits.RecentMemoryChars = 1024
	cfg.Agent.WaitForTools = true
	memory := newAgentTestMemory()
	memory.files["prompts/migrations.md"] = `# Prompt migrations applied

## Applied

- 000 add-untrusted-content-convention 2026-05-01T00:00:00Z
- 001 autonomy-prompts-v1 2026-05-01T00:00:00Z
`
	agent := New(cfg, Deps{
		Chat:     chat,
		Memory:   memory,
		Search:   noopSearcher{},
		Tools:    tools,
		State:    emptyStateStore{},
		Daemons:  daemons,
		MaxTurns: 2,
	}, newTestLogger())

	if err := agent.Run(context.Background(), make(chan struct{})); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if tools.execCount["skill_activate"] != 0 {
		t.Fatalf("skill_activate executions = %d, want 0 (P0 defers tools)", tools.execCount["skill_activate"])
	}
	var sawDeferred, sawCry bool
	for _, m := range chat.seen {
		if m.Role == llm.RoleTool && strings.Contains(m.Content, "[Deferred]") {
			sawDeferred = true
		}
		if m.Role == llm.RoleUser && strings.Contains(m.Content, "cry") {
			sawCry = true
		}
	}
	if !sawDeferred {
		t.Fatal("expected [Deferred] tool stub for P0 interrupt")
	}
	if !sawCry {
		t.Fatal("expected daemon alert injected after Chat")
	}
}
