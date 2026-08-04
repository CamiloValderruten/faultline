package tools

import "fmt"

// truncateToolResult clips a tool result at limit characters and appends
// a retrieval hint. limit <= 0 disables the cap. Used as a universal
// safety net in Execute so uncapped surfaces (notably MCP) cannot blow
// out the conversation context in a single turn.
func truncateToolResult(result string, limit int) string {
	if limit <= 0 || len(result) <= limit {
		return result
	}
	return result[:limit] + fmt.Sprintf(
		"\n\n[truncated: showing first %d of %d chars. Re-call with a smaller range/query, or write large output to a file and read slices.]",
		limit, len(result),
	)
}
