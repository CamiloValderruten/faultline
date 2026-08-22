package agent

import (
	"context"
	"strings"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

// collaboratorDeliveryDebtPrompt replaces the generic continue prompt when
// a drained collaborator batch still needs an outbound reply. Assistant
// text is not delivered to Discord/Telegram — only the send_* tools are.
const collaboratorDeliveryDebtPrompt = `[Time: %s]

Your collaborator is still waiting. Your previous assistant text was NOT delivered to them — only send_message / send_rich_message / send_voice_message reach Discord/Telegram.

Call one of those tools NOW with your reply. You may use other tools first if you still need them; do not sleep and do not reply with text-only content.`

// sleepBlockedByDeliveryDebt is returned when the model calls sleep while a
// collaborator delivery debt is outstanding. Research tools stay allowed;
// sleep is gated so the agent cannot nap before delivering a reply.
// Preserves tool_call_id pairing.
const sleepBlockedByDeliveryDebt = `Error: collaborator reply still owed. Call send_message (or send_rich_message / send_voice_message) with your reply before sleeping.`

func isCollaboratorSendTool(name string) bool {
	switch name {
	case "send_message", "send_rich_message", "send_voice_message":
		return true
	default:
		return false
	}
}

// collaboratorSendSucceeded reports whether a collaborator outbound tool
// result is a successful delivery. Matches the tools package convention of
// Error:/Failed: prefixes on failure strings.
func collaboratorSendSucceeded(name, result string) bool {
	if !isCollaboratorSendTool(name) {
		return false
	}
	if strings.HasPrefix(result, "Error") || strings.HasPrefix(result, "Failed") {
		return false
	}
	return true
}

// isToolErrorResult reports whether a tool execution result represents a failure.
// Checks standard error prefixes produced by the tools and adapters layers.
func isToolErrorResult(result string) bool {
	trimmed := strings.TrimSpace(result)
	switch {
	case strings.HasPrefix(trimmed, "Error:"),
		strings.HasPrefix(trimmed, "ERROR:"),
		strings.HasPrefix(trimmed, "error:"),
		strings.HasPrefix(trimmed, "Failed:"),
		strings.HasPrefix(trimmed, "failed:"),
		strings.HasPrefix(trimmed, "Error parsing arguments:"),
		strings.HasPrefix(trimmed, "Tool"):
		return true
	default:
		return false
	}
}

// executeToolCalls runs each tool call. While collaboratorReplyOwed is set,
// sleep is rejected so the model cannot nap before delivering a reply;
// other tools (research, skills, MCP) remain allowed. An early
// acknowledgment send is optional — the first successful send_* may be the
// full answer. Debt clears after a successful send. debt may be nil when
// the caller does not track it (e.g. compaction). scrubbed lists tool_call
// IDs whose Arguments were rewritten from invalid JSON; those get a
// synthetic error result instead of a real dispatch.
//
// Returns the updated message log, whether all executed tool calls failed,
// and the error text of the last failure (if any).
func (a *Agent) executeToolCalls(ctx context.Context, messages []llm.Message, toolCalls []llm.ToolCall, debt *bool, scrubbed map[string]bool) ([]llm.Message, bool, string) {
	a.tools.SetContextInfo(a.countMessageTokens(messages))
	failedCount := 0
	lastErr := ""
	for _, tc := range toolCalls {
		name := ""
		if tc.Function.Name != "" {
			name = tc.Function.Name
		}
		var result string
		switch {
		case scrubbed[tc.ID]:
			result = malformedToolArgsResult
			a.logger.Warn("skipped tool call with invalid arguments JSON",
				"tool", name, "tool_call_id", tc.ID)
		case debt != nil && *debt && name == "sleep":
			result = sleepBlockedByDeliveryDebt
			a.logger.Info("sleep blocked: collaborator delivery debt outstanding")
		default:
			result = a.tools.Execute(ctx, tc)
		}
		if debt != nil && *debt && collaboratorSendSucceeded(name, result) {
			*debt = false
			a.logger.Info("collaborator delivery debt cleared", "tool", name)
		}
		if isToolErrorResult(result) {
			failedCount++
			lastErr = result
		}
		messages = append(messages, toolMessage(tc.ID, result))
		a.recordToolCall()
	}
	allFailed := len(toolCalls) > 0 && failedCount == len(toolCalls)
	return messages, allFailed, lastErr
}
