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

Call one of those tools NOW with your reply (or a short acknowledgment like "on it" if you still need tools). Do not sleep, and do not reply with text-only content.`

// sleepBlockedByDeliveryDebt is returned as the sleep tool result when a
// collaborator delivery debt is outstanding. Preserves tool_call_id pairing.
const sleepBlockedByDeliveryDebt = `Error: collaborator reply still owed. Call send_message (or send_rich_message / send_voice_message) to deliver a reply — even a short "on it" — before sleeping.`

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

// executeToolCalls runs each tool call, rejecting sleep while
// collaboratorReplyOwed is set and clearing that debt after a successful
// collaborator send. debt may be nil when the caller does not track it
// (should not happen on the primary loop).
func (a *Agent) executeToolCalls(ctx context.Context, messages []llm.Message, toolCalls []llm.ToolCall, debt *bool) []llm.Message {
	a.tools.SetContextInfo(a.countMessageTokens(messages))
	for _, tc := range toolCalls {
		name := ""
		if tc.Function.Name != "" {
			name = tc.Function.Name
		}
		var result string
		if debt != nil && *debt && name == "sleep" {
			result = sleepBlockedByDeliveryDebt
			a.logger.Info("sleep blocked: collaborator delivery debt outstanding")
		} else {
			result = a.tools.Execute(ctx, tc)
		}
		if debt != nil && *debt && collaboratorSendSucceeded(name, result) {
			*debt = false
			a.logger.Info("collaborator delivery debt cleared", "tool", name)
		}
		messages = append(messages, toolMessage(tc.ID, result))
		a.recordToolCall()
	}
	return messages
}
