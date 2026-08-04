package agent

import (
	"errors"
	"fmt"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

func TestIsInvalidToolArgs(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unrelated", err: errors.New("connection reset"), want: false},
		{name: "rate limit", err: errors.New("chat completion: HTTP 429: rate limit"), want: false},
		{
			name: "minimax 2013",
			err:  fmt.Errorf("llm chat: %w", fmt.Errorf("chat completion: HTTP 400: invalid params, invalid function arguments json string, tool_call_id: call_x (2013)")),
			want: true,
		},
		{
			name: "wording only",
			err:  errors.New("HTTP 400: Invalid Function Arguments"),
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isInvalidToolArgs(tc.err); got != tc.want {
				t.Fatalf("isInvalidToolArgs(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestScrubInvalidToolCallArgs(t *testing.T) {
	msg := &llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{
			{
				ID:   "good",
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "sandbox_write",
					Arguments: `{"folder":"html","filename":"x.html","content":"<p>hi</p>"}`,
				},
			},
			{
				ID:   "bad",
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "sandbox_write",
					Arguments: `{"folder":"html","filename":"x.html","content":"<p>truncat`,
				},
			},
			{
				ID:   "empty",
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "get_time",
					Arguments: "",
				},
			},
		},
	}

	scrubbed := scrubInvalidToolCallArgs(msg)
	if !scrubbed["bad"] || !scrubbed["empty"] {
		t.Fatalf("scrubbed = %#v, want bad and empty", scrubbed)
	}
	if scrubbed["good"] {
		t.Fatalf("good call should not be scrubbed")
	}
	if msg.ToolCalls[0].Function.Arguments != `{"folder":"html","filename":"x.html","content":"<p>hi</p>"}` {
		t.Fatalf("good args mutated: %q", msg.ToolCalls[0].Function.Arguments)
	}
	if msg.ToolCalls[1].Function.Arguments != "{}" {
		t.Fatalf("bad args = %q, want {}", msg.ToolCalls[1].Function.Arguments)
	}
	if msg.ToolCalls[2].Function.Arguments != "{}" {
		t.Fatalf("empty args = %q, want {}", msg.ToolCalls[2].Function.Arguments)
	}
}

func TestScrubInvalidToolCallArgsInHistory(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "sys"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID: "poison",
				Function: llm.FunctionCall{
					Name:      "sandbox_write",
					Arguments: `{"content":"oops`,
				},
			}},
		},
		{Role: llm.RoleTool, ToolCallID: "poison", Content: "Error parsing arguments"},
	}
	n := scrubInvalidToolCallArgsInHistory(messages)
	if n != 1 {
		t.Fatalf("scrubbed count = %d, want 1", n)
	}
	if messages[1].ToolCalls[0].Function.Arguments != "{}" {
		t.Fatalf("history args = %q, want {}", messages[1].ToolCalls[0].Function.Arguments)
	}
}
