package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

func (te *Executor) daemonsEnabled() bool {
	return te.sandbox != nil && te.daemonAgent != ""
}

func (te *Executor) daemonToolDefs() []llm.Tool {
	return []llm.Tool{
		{
			Type: llm.ToolTypeFunction,
			Function: &llm.FunctionDef{
				Name: "daemon_spawn",
				Description: "Start a long-lived Docker daemon that survives Faultline restarts and host reboots " +
					"(--restart unless-stopped). Command must run a flat script under /scripts/ " +
					"(write it with sandbox_write first). A description is required so daemon_list " +
					"remains meaningful after context compaction. Inherits sandbox network/memory/user settings. " +
					"Per-daemon /work is mounted read-write for alerts/state.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Short lowercase name (e.g. price-watch).",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "What this daemon does (required, single line, max 512 chars). Stored as a Docker label.",
						},
						"command": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "Argv inside the sandbox image. Must include /scripts/<file>, e.g. [\"python3\", \"/scripts/watch.py\"].",
						},
						"env": map[string]interface{}{
							"type":                 "object",
							"additionalProperties": map[string]interface{}{"type": "string"},
							"description":          "Optional extra environment variables for this daemon.",
						},
					},
					"required": []string{"name", "description", "command"},
				},
			},
		},
		{
			Type: llm.ToolTypeFunction,
			Function: &llm.FunctionDef{
				Name: "daemon_list",
				Description: "List this agent's daemon containers via Docker labels (faultline.agent). " +
					"Works after Faultline restarts and host reboots. Includes description and status.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: llm.ToolTypeFunction,
			Function: &llm.FunctionDef{
				Name:        "daemon_fetch",
				Description: "Fetch recent stdout/stderr from a daemon container (docker logs --tail).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"daemon_id": map[string]interface{}{
							"type":        "string",
							"description": "Daemon id from daemon_spawn / daemon_list.",
						},
						"tail": map[string]interface{}{
							"type":        "integer",
							"description": "Lines to return (default 50, max 2000).",
						},
					},
					"required": []string{"daemon_id"},
				},
			},
		},
		{
			Type: llm.ToolTypeFunction,
			Function: &llm.FunctionDef{
				Name: "daemon_stop",
				Description: "Intentionally stop a daemon: clears restart policy then docker stop " +
					"so it will not return on host reboot.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"daemon_id": map[string]interface{}{
							"type":        "string",
							"description": "Daemon id from daemon_spawn / daemon_list.",
						},
					},
					"required": []string{"daemon_id"},
				},
			},
		},
	}
}

func (te *Executor) daemonSpawn(ctx context.Context, args string) string {
	if !te.daemonsEnabled() {
		return "Error: daemons not enabled"
	}
	var p struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Command     []string          `json:"command"`
		Env         map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return fmt.Sprintf("Error: invalid arguments: %v", err)
	}
	if strings.TrimSpace(p.Description) == "" {
		return "Error: description is required"
	}
	if utf8.RuneCountInString(p.Description) > 512 {
		return "Error: description exceeds 512 characters"
	}
	info, err := te.sandbox.StartDaemon(ctx, te.daemonAgent, p.Name, p.Description, p.Command, p.Env)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	b, err := json.Marshal(info)
	if err != nil {
		return fmt.Sprintf("Error: encode: %v", err)
	}
	return string(b)
}

func (te *Executor) daemonList(ctx context.Context) string {
	if !te.daemonsEnabled() {
		return "Error: daemons not enabled"
	}
	infos, err := te.sandbox.ListDaemons(ctx, te.daemonAgent)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if infos == nil {
		return "[]"
	}
	b, err := json.Marshal(infos)
	if err != nil {
		return fmt.Sprintf("Error: encode: %v", err)
	}
	return string(b)
}

func (te *Executor) daemonFetch(ctx context.Context, args string) string {
	if !te.daemonsEnabled() {
		return "Error: daemons not enabled"
	}
	var p struct {
		DaemonID string `json:"daemon_id"`
		Tail     int    `json:"tail"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return fmt.Sprintf("Error: invalid arguments: %v", err)
	}
	out, err := te.sandbox.FetchDaemonLogs(ctx, te.daemonAgent, p.DaemonID, p.Tail)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if out == "" {
		return "(no log output)"
	}
	return out
}

func (te *Executor) daemonStop(ctx context.Context, args string) string {
	if !te.daemonsEnabled() {
		return "Error: daemons not enabled"
	}
	var p struct {
		DaemonID string `json:"daemon_id"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return fmt.Sprintf("Error: invalid arguments: %v", err)
	}
	info, err := te.sandbox.StopDaemon(ctx, te.daemonAgent, p.DaemonID)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	b, err := json.Marshal(info)
	if err != nil {
		return fmt.Sprintf("Error: encode: %v", err)
	}
	return string(b)
}
