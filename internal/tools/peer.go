package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

func (te *Executor) peerToolDefs() []llm.Tool {
	if te.peers == nil {
		return nil
	}
	names := te.peers.PeerNames()
	peerList := "none configured"
	if len(names) > 0 {
		peerList = strings.Join(names, ", ")
	}
	return []llm.Tool{
		{
			Type: llm.ToolTypeFunction,
			Function: &llm.FunctionDef{
				Name: "peer_send",
				Description: "Send a message to another Faultline agent. The message lands in their pull-only inbox — they will not see it until they call peer_inbox/peer_read. " +
					"Known peers: " + peerList + ".",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"to": map[string]interface{}{
							"type":        "string",
							"description": "Peer agent name from the configured list.",
						},
						"text": map[string]interface{}{
							"type":        "string",
							"description": "Message body.",
						},
					},
					"required": []string{"to", "text"},
				},
			},
		},
		{
			Type: llm.ToolTypeFunction,
			Function: &llm.FunctionDef{
				Name:        "peer_inbox",
				Description: "List unread messages from other Faultline agents. Does not mark them read. Pull-only: nothing arrives in conversation until you call this (or peer_read).",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: llm.ToolTypeFunction,
			Function: &llm.FunctionDef{
				Name:        "peer_read",
				Description: "Read and acknowledge one peer inbox message by id (removes it from the inbox).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Message id from peer_inbox.",
						},
					},
					"required": []string{"id"},
				},
			},
		},
	}
}

func (te *Executor) peerSend(ctx context.Context, argsJSON string) string {
	if te.peers == nil {
		return "Error: peer messaging is not configured."
	}
	var args struct {
		To   string `json:"to"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "Error: invalid arguments: " + err.Error()
	}
	id, err := te.peers.Send(ctx, args.To, args.Text)
	if err != nil {
		return "Error: " + err.Error()
	}
	if id != "" {
		return fmt.Sprintf("Sent to %s (id %s).", strings.ToLower(strings.TrimSpace(args.To)), id)
	}
	return fmt.Sprintf("Sent to %s.", strings.ToLower(strings.TrimSpace(args.To)))
}

func (te *Executor) peerInbox() string {
	if te.peers == nil {
		return "Error: peer messaging is not configured."
	}
	msgs, err := te.peers.List()
	if err != nil {
		return "Error: " + err.Error()
	}
	type row struct {
		ID         string `json:"id"`
		From       string `json:"from"`
		ReceivedAt string `json:"received_at"`
		Preview    string `json:"preview"`
	}
	out := make([]row, 0, len(msgs))
	for _, m := range msgs {
		preview := m.Text
		if len(preview) > 120 {
			preview = preview[:120] + "…"
		}
		out = append(out, row{
			ID:         m.ID,
			From:       m.From,
			ReceivedAt: m.ReceivedAt.Format("2006-01-02T15:04:05Z"),
			Preview:    preview,
		})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "Error: " + err.Error()
	}
	if len(out) == 0 {
		return "[]"
	}
	return string(data)
}

func (te *Executor) peerRead(argsJSON string) string {
	if te.peers == nil {
		return "Error: peer messaging is not configured."
	}
	var args struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "Error: invalid arguments: " + err.Error()
	}
	msg, err := te.peers.Read(args.ID)
	if err != nil {
		return "Error: " + err.Error()
	}
	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return "Error: " + err.Error()
	}
	return string(data)
}
