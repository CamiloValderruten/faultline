package agent

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/config"
	"github.com/CamiloValderruten/faultline/internal/llm"
)

func TestCollaboratorSendSucceeded(t *testing.T) {
	cases := []struct {
		name, result string
		want         bool
	}{
		{"send_message", "Message sent to collaborator.", true},
		{"send_rich_message", "Rich message sent to collaborator.", true},
		{"send_voice_message", "Voice message sent to collaborator.", true},
		{"send_message", "Error: discord send failed", false},
		{"send_message", "Failed: boom", false},
		{"sleep", "Slept for 1s", false},
		{"memory_write", "ok", false},
	}
	for _, tc := range cases {
		got := collaboratorSendSucceeded(tc.name, tc.result)
		if got != tc.want {
			t.Fatalf("%s / %q: got %v want %v", tc.name, tc.result, got, tc.want)
		}
	}
}

func TestInjectPendingMessagesMarksCollaborator(t *testing.T) {
	a := newTestAgent()
	a.operator = &scriptedOperator{batches: [][]string{{"ping"}}}

	messages, injected, collab := a.injectPendingMessages(nil)
	if !injected || !collab {
		t.Fatalf("injected=%v collab=%v, want both true", injected, collab)
	}
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	if !strings.Contains(messages[0].Content, "assistant text is not delivered") {
		t.Fatalf("missing delivery guidance:\n%s", messages[0].Content)
	}
	if !strings.Contains(messages[0].Content, "not required") {
		t.Fatalf("missing optional-ack guidance:\n%s", messages[0].Content)
	}
}

func TestRunNudgeOnTextOnlyWhileDeliveryDebt(t *testing.T) {
	chat := &scriptedChat{responses: []*llm.ChatResponse{
		textOnlyResponse("I looked it up; balance is $1."),
		toolCallResponse("send_message", `{"text":"balance is $1"}`),
	}}
	tools := &recordingTools{results: map[string]string{
		"send_message": "Message sent to collaborator.",
	}}
	op := &scriptedOperator{batches: [][]string{{"what is left?"}}}
	agent := newDeliveryDebtAgent(chat, tools, op, 2)

	if err := agent.Run(context.Background(), make(chan struct{})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var sawNudge, sawSend bool
	for _, m := range chat.seen {
		if m.Role == llm.RoleUser && strings.Contains(m.Content, "NOT delivered") {
			sawNudge = true
		}
	}
	for _, name := range tools.executed {
		if name == "send_message" {
			sawSend = true
		}
	}
	if !sawNudge {
		t.Fatal("expected delivery-debt nudge after text-only reply")
	}
	if !sawSend {
		t.Fatal("expected send_message after nudge")
	}
}

func TestRunAllowsResearchToolsWhileDeliveryDebt(t *testing.T) {
	chat := &scriptedChat{responses: []*llm.ChatResponse{
		toolCallResponse("skill_activate", `{"name":"finances"}`),
		toolCallResponse("send_message", `{"text":"July groceries were $412."}`),
	}}
	tools := &recordingTools{results: map[string]string{
		"skill_activate": "skill loaded",
		"send_message":   "Message sent to collaborator.",
	}}
	op := &scriptedOperator{batches: [][]string{{"quick question"}}}
	agent := newDeliveryDebtAgent(chat, tools, op, 2)

	if err := agent.Run(context.Background(), make(chan struct{})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if tools.execCount["skill_activate"] != 1 {
		t.Fatalf("skill_activate executions = %d, want 1 (research allowed before send)", tools.execCount["skill_activate"])
	}
	if tools.execCount["send_message"] != 1 {
		t.Fatalf("send_message executions = %d, want 1", tools.execCount["send_message"])
	}
}

func TestRunBlocksSleepWhileDeliveryDebt(t *testing.T) {
	chat := &scriptedChat{responses: []*llm.ChatResponse{
		toolCallResponse("sleep", `{"seconds":60}`),
		toolCallResponse("send_message", `{"text":"Looking up the mortgage balance now."}`),
	}}
	tools := &recordingTools{results: map[string]string{
		"sleep":        "Slept for 60s",
		"send_message": "Message sent to collaborator.",
	}}
	op := &scriptedOperator{batches: [][]string{{"quick question"}}}
	agent := newDeliveryDebtAgent(chat, tools, op, 2)

	if err := agent.Run(context.Background(), make(chan struct{})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if tools.execCount["sleep"] != 0 {
		t.Fatalf("sleep should not reach Tools.Execute while debt owed, got %d", tools.execCount["sleep"])
	}
	if tools.execCount["send_message"] != 1 {
		t.Fatalf("send_message executions = %d, want 1", tools.execCount["send_message"])
	}
	var sawBlocked bool
	for _, m := range chat.seen {
		if m.Role == llm.RoleTool && strings.Contains(m.Content, "before sleeping") {
			sawBlocked = true
		}
	}
	if !sawBlocked {
		t.Fatal("expected synthetic sleep rejection in conversation")
	}
}

func TestRunClearsDebtOnSuccessfulSend(t *testing.T) {
	chat := &scriptedChat{responses: []*llm.ChatResponse{
		toolCallResponse("send_message", `{"text":"Pulling your July spending from Tiller."}`),
		toolCallResponse("sleep", `{"seconds":1}`),
	}}
	tools := &recordingTools{results: map[string]string{
		"send_message": "Message sent to collaborator.",
		"sleep":        "Slept for 1s",
	}}
	op := &scriptedOperator{batches: [][]string{{"hey"}}}
	agent := newDeliveryDebtAgent(chat, tools, op, 2)

	if err := agent.Run(context.Background(), make(chan struct{})); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if tools.execCount["sleep"] != 1 {
		t.Fatalf("sleep executions = %d, want 1 after debt cleared", tools.execCount["sleep"])
	}
	if tools.execCount["send_message"] != 1 {
		t.Fatalf("send_message executions = %d, want 1", tools.execCount["send_message"])
	}
}

func newDeliveryDebtAgent(chat ChatModel, tools Tools, op Operator, maxTurns int) *Agent {
	memory := newAgentTestMemory()
	memory.files["prompts/migrations.md"] = `# Prompt migrations applied

## Applied

- 000 add-untrusted-content-convention 2026-05-01T00:00:00Z
- 001 autonomy-prompts-v1 2026-05-01T00:00:00Z
`
	cfg := config.Default()
	cfg.Limits.RecentMemoryChars = 1024
	return New(cfg, Deps{
		Chat:     chat,
		Memory:   memory,
		Search:   noopSearcher{},
		Operator: op,
		Tools:    tools,
		State:    emptyStateStore{},
		MaxTurns: maxTurns,
	}, newTestLogger())
}

type scriptedOperator struct {
	mu      sync.Mutex
	batches [][]string
}

func (o *scriptedOperator) Pending() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.batches) == 0 {
		return nil
	}
	batch := o.batches[0]
	o.batches = o.batches[1:]
	return batch
}

type scriptedChat struct {
	mu               sync.Mutex
	responses        []*llm.ChatResponse
	seen             []llm.Message
	headsetThinking  string
	sawHeadsetPrompt bool
}

func (c *scriptedChat) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seen = append([]llm.Message(nil), req.Messages...)
	for _, m := range req.Messages {
		if m.Role == llm.RoleUser && strings.Contains(m.Content, "USB headset") {
			c.sawHeadsetPrompt = true
			c.headsetThinking = req.Thinking
			break
		}
	}
	if len(c.responses) == 0 {
		return textOnlyResponse("done"), nil
	}
	resp := c.responses[0]
	c.responses = c.responses[1:]
	return resp, nil
}

func textOnlyResponse(content string) *llm.ChatResponse {
	return &llm.ChatResponse{Choices: []llm.Choice{{
		FinishReason: "stop",
		Message:      llm.Message{Role: llm.RoleAssistant, Content: content},
	}}}
}

func toolCallResponse(name, args string) *llm.ChatResponse {
	return &llm.ChatResponse{Choices: []llm.Choice{{
		FinishReason: "tool_calls",
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID:   "call-" + name,
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      name,
					Arguments: args,
				},
			}},
		},
	}}}
}

type recordingTools struct {
	mu        sync.Mutex
	results   map[string]string
	execCount map[string]int
	executed  []string
}

func (t *recordingTools) ToolDefs() []llm.Tool {
	return []llm.Tool{
		{Type: llm.ToolTypeFunction, Function: &llm.FunctionDef{Name: "send_message", Parameters: map[string]any{"type": "object"}}},
		{Type: llm.ToolTypeFunction, Function: &llm.FunctionDef{Name: "sleep", Parameters: map[string]any{"type": "object"}}},
		{Type: llm.ToolTypeFunction, Function: &llm.FunctionDef{Name: "skill_activate", Parameters: map[string]any{"type": "object"}}},
	}
}

func (t *recordingTools) Execute(_ context.Context, call llm.ToolCall) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.execCount == nil {
		t.execCount = map[string]int{}
	}
	name := call.Function.Name
	t.execCount[name]++
	t.executed = append(t.executed, name)
	result := t.results[name]
	if result == "" {
		result = "ok"
	}
	return result
}

func (t *recordingTools) SetContextInfo(int) {}
func (t *recordingTools) Close()             {}
