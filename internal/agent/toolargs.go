package agent

import (
	"encoding/json"
	"strings"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

// malformedToolArgsResult is returned instead of running a tool when the
// model's function.arguments string is not valid JSON (typically truncated
// under max_response_tokens). Preserves tool_call_id pairing while keeping
// history re-sendable to providers that reject invalid args (e.g. MiniMax 2013).
const malformedToolArgsResult = `Error: tool call arguments were not valid JSON (often truncated by the response token limit). Re-issue with a smaller payload. For large files prefer sandbox_append / sandbox_edit in chunks, or a shorter sandbox_write.`

// isInvalidToolArgs reports whether err looks like a provider rejection of
// malformed tool-call function.arguments in the request history (MiniMax
// HTTP 400 code 2013 and similar wording).
func isInvalidToolArgs(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid function arguments") ||
		strings.Contains(msg, "(2013)")
}

// scrubInvalidToolCallArgs rewrites any tool call whose Arguments are not
// valid JSON to "{}". Returns the set of scrubbed tool_call IDs so the
// executor can emit a synthetic error instead of dispatching.
func scrubInvalidToolCallArgs(msg *llm.Message) map[string]bool {
	if msg == nil || len(msg.ToolCalls) == 0 {
		return nil
	}
	var scrubbed map[string]bool
	for i := range msg.ToolCalls {
		tc := &msg.ToolCalls[i]
		if json.Valid([]byte(tc.Function.Arguments)) {
			continue
		}
		tc.Function.Arguments = "{}"
		if scrubbed == nil {
			scrubbed = make(map[string]bool)
		}
		if tc.ID != "" {
			scrubbed[tc.ID] = true
		}
	}
	return scrubbed
}

// scrubInvalidToolCallArgsInHistory rewrites invalid Arguments on every
// assistant tool-call message in place. Used on state resume and when a
// Chat call fails with isInvalidToolArgs so poisoned history can heal
// without a process exit.
func scrubInvalidToolCallArgsInHistory(messages []llm.Message) int {
	n := 0
	for i := range messages {
		if messages[i].Role != llm.RoleAssistant {
			continue
		}
		before := 0
		if scrubbed := scrubInvalidToolCallArgs(&messages[i]); scrubbed != nil {
			before = len(scrubbed)
		}
		n += before
	}
	return n
}
