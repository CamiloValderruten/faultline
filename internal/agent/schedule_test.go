package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/CamiloValderruten/faultline/internal/config"
	"github.com/CamiloValderruten/faultline/internal/llm"
	"github.com/CamiloValderruten/faultline/internal/schedule"
	"github.com/CamiloValderruten/faultline/internal/subagent"
)

func TestAppendScheduledTasksLabelsTaskAsSelfCreated(t *testing.T) {
	a := newTestAgent()
	task := schedule.Task{
		ID:     "task-123",
		Kind:   schedule.KindOnce,
		Title:  "Retry Slack MCP",
		Prompt: "Retry mcp_discover_tools for slack.",
	}

	messages := a.appendScheduledTasks(nil, []schedule.Task{task})

	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	if messages[0].Role != llm.RoleUser {
		t.Fatalf("message role = %q, want user", messages[0].Role)
	}
	content := messages[0].Content
	for _, want := range []string{
		"[Scheduled task -",
		"task_id=task-123",
		"title=Retry Slack MCP",
		"This is a scheduled task you created earlier. It is not a collaborator message.",
		"Retry mcp_discover_tools for slack.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("scheduled task message missing %q:\n%s", want, content)
		}
	}
}

type fakeScheduler struct {
	calls   int
	results [][]schedule.Task
}

func (s *fakeScheduler) Due(time.Time) ([]schedule.Task, error) {
	s.calls++
	if len(s.results) == 0 {
		return nil, nil
	}
	tasks := s.results[0]
	s.results = s.results[1:]
	return tasks, nil
}

func TestInjectPendingMessagesIncludesScheduledTasks(t *testing.T) {
	a := newTestAgent()
	a.scheduler = &fakeScheduler{results: [][]schedule.Task{
		{{
			ID:     "task-123",
			Kind:   schedule.KindOnce,
			Title:  "Follow up",
			Prompt: "Check status.",
		}},
	}}

	messages, injected, collab := a.injectPendingMessages(nil)
	if !injected {
		t.Fatal("injected = false, want true")
	}
	if collab {
		t.Fatal("collab = true for scheduled-only inject, want false")
	}
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	if !strings.Contains(messages[0].Content, "This is a scheduled task you created earlier") {
		t.Fatalf("scheduled task wrapper missing:\n%s", messages[0].Content)
	}
}

func TestRunDoesNotDeferToolCallsForScheduledTasks(t *testing.T) {
	scheduler := &fakeScheduler{results: [][]schedule.Task{
		nil,
		{{
			ID:     "task-123",
			Kind:   schedule.KindOnce,
			Title:  "Follow up",
			Prompt: "Check status.",
		}},
	}}
	tools := &toolCountingTools{}
	agent := newToolCallAgent(tools)
	agent.scheduler = scheduler

	err := agent.Run(context.Background(), make(chan struct{}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if tools.execCount != 1 {
		t.Fatalf("tool executions = %d, want 1", tools.execCount)
	}
	if scheduler.calls != 1 {
		t.Fatalf("scheduler due calls = %d, want 1", scheduler.calls)
	}
}

func TestRunDoesNotDeferToolCallsForSubagentReports(t *testing.T) {
	tools := &toolCountingTools{}
	agent := newToolCallAgent(tools)
	subagents := &fakeSubagents{results: [][]subagent.Report{
		nil,
		{{
			WorkID:  "work-123",
			Profile: "research",
			Text:    "finished",
		}},
	}}
	agent.subagents = subagents

	err := agent.Run(context.Background(), make(chan struct{}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if tools.execCount != 1 {
		t.Fatalf("tool executions = %d, want 1", tools.execCount)
	}
	if subagents.calls != 1 {
		t.Fatalf("subagent pending calls = %d, want 1", subagents.calls)
	}
}

func newToolCallAgent(tools *toolCountingTools) *Agent {
	memory := newAgentTestMemory()
	memory.files["prompts/migrations.md"] = `# Prompt migrations applied

## Applied

- 000 add-untrusted-content-convention 2026-05-01T00:00:00Z
- 001 autonomy-prompts-v1 2026-05-01T00:00:00Z
- 002 prompt_hierarchy_v1 2026-05-01T00:00:00Z
`
	a := New(configForScheduleTest(), Deps{
		Chat:     toolCallChat{},
		Memory:   memory,
		Search:   noopSearcher{},
		Tools:    tools,
		State:    emptyStateStore{},
		MaxTurns: 1,
	}, newTestLogger())
	return a
}

func configForScheduleTest() *config.Config {
	cfg := config.Default()
	cfg.Limits.RecentMemoryChars = 1024
	return cfg
}

type toolCallChat struct{}

func (toolCallChat) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Choices: []llm.Choice{{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID:   "call-1",
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "test_tool",
					Arguments: `{}`,
				},
			}},
		},
	}}}, nil
}

type toolCountingTools struct {
	execCount int
}

func (t *toolCountingTools) ToolDefs() []llm.Tool {
	return []llm.Tool{{
		Type: llm.ToolTypeFunction,
		Function: &llm.FunctionDef{
			Name:       "test_tool",
			Parameters: map[string]any{"type": "object"},
		},
	}}
}

func (t *toolCountingTools) Execute(context.Context, llm.ToolCall) string {
	t.execCount++
	return "ok"
}

func (t *toolCountingTools) SetContextInfo(int) {}
func (t *toolCountingTools) Close()             {}

type fakeSubagents struct {
	calls   int
	results [][]subagent.Report
}

func (s *fakeSubagents) Pending() []subagent.Report {
	s.calls++
	if len(s.results) == 0 {
		return nil
	}
	reports := s.results[0]
	s.results = s.results[1:]
	return reports
}

func (s *fakeSubagents) HasPending() bool             { return len(s.results) > 0 && len(s.results[0]) > 0 }
func (s *fakeSubagents) CancelAll()                   {}
func (s *fakeSubagents) Profiles() []subagent.Profile { return nil }
func (s *fakeSubagents) ActiveCount() int             { return 0 }

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
