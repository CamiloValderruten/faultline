package agent

import (
	"fmt"
	"strings"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

// compactionSaveBudgetFrac is the token fraction of MaxTokens we try to
// reach by shrinking oversized tool messages before asking the model to
// summarize. Leaves headroom for the compaction Chat turn itself.
const compactionSaveBudgetFrac = 0.85

// compactionToolFloor is the smallest a tool message may be shrunk to
// while freeing context. Below this we stop — further cuts won't help
// enough to run a save turn.
const compactionToolFloor = 512

// lastCollaboratorMessage returns the Content of the most recent user
// turn that looks like an injected collaborator message, or "".
func lastCollaboratorMessage(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Role != llm.RoleUser {
			continue
		}
		if strings.Contains(m.Content, "[Collaborator message") {
			return m.Content
		}
	}
	return ""
}

// compactionRecoverySummary is used when the compaction loop cannot
// produce a model summary (typically because context was already past
// the hard ceiling). Preserves the last collaborator turn so drained
// operator messages are not silently lost on rebuild.
func compactionRecoverySummary(lastCollab string) string {
	var b strings.Builder
	b.WriteString("Context overflowed during an active turn before a normal compaction summary could be written. Large tool results were trimmed. Resume any in-progress work from memory and recent collaborator input.")
	if lastCollab != "" {
		b.WriteString("\n\nLast collaborator message still needing attention:\n")
		b.WriteString(lastCollab)
	}
	return b.String()
}

// shrinkOversizedToolMessages mutates messages in place, repeatedly
// truncating the largest role=tool payloads until countFn reports at
// most targetTokens (or nothing further can be cut). Returns how many
// messages were truncated.
func shrinkOversizedToolMessages(messages []llm.Message, targetTokens int, countFn func([]llm.Message) int) int {
	if targetTokens <= 0 || countFn == nil {
		return 0
	}
	n := 0
	for countFn(messages) > targetTokens {
		idx := -1
		maxLen := 0
		for i := range messages {
			if messages[i].Role != llm.RoleTool {
				continue
			}
			if l := len(messages[i].Content); l > maxLen {
				maxLen = l
				idx = i
			}
		}
		if idx < 0 || maxLen <= compactionToolFloor {
			break
		}
		newCap := maxLen / 2
		if maxLen > 16000 {
			// Fat MCP dumps: jump straight to a modest preview rather
			// than halving a multi-megabyte blob many times.
			newCap = 8000
		}
		if newCap < compactionToolFloor {
			newCap = compactionToolFloor
		}
		if newCap >= maxLen {
			break
		}
		messages[idx].Content = messages[idx].Content[:newCap] + fmt.Sprintf(
			"\n\n[truncated during compaction: showing first %d of %d chars to free context]",
			newCap, maxLen,
		)
		n++
	}
	return n
}
