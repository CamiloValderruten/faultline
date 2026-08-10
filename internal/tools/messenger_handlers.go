package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CamiloValderruten/faultline/internal/messaging"
)

// voiceSender is optionally implemented by the Discord messenger when Deepgram
// TTS is configured.
type voiceSender interface {
	SendVoice(text string) error
}

// fileSender is optionally implemented by the Discord messenger for outbound
// file uploads (sandbox paths resolved in the tools layer).
type fileSender interface {
	SendFile(path, filename, caption string) error
}

func (te *Executor) sendMessage(argsJSON string) string {
	var args struct {
		Text    string                 `json:"text"`
		Buttons [][]messaging.Button   `json:"buttons"`
		Selects []messaging.SelectMenu `json:"selects"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %s", err)
	}

	if args.Text == "" {
		return "Error: text is required"
	}

	if te.messenger == nil {
		return "Error: messaging is not configured. No collaborator channel available."
	}

	if len(args.Buttons) == 0 && len(args.Selects) == 0 {
		if err := te.messenger.Send(args.Text); err != nil {
			return fmt.Sprintf("Error sending message: %s", err)
		}
		te.logger.Info("message sent to collaborator", "length", len(args.Text))
		return "Message sent to collaborator."
	}

	if len(args.Selects) > 0 {
		if err := te.messenger.SendRich(messaging.RichMessage{
			Content: args.Text,
			Buttons: args.Buttons,
			Selects: args.Selects,
		}); err != nil {
			return fmt.Sprintf("Error sending message: %s", err)
		}
		te.logger.Info("message with components sent to collaborator",
			"length", len(args.Text),
			"button_rows", len(args.Buttons),
			"selects", len(args.Selects),
		)
		return "Message sent to collaborator."
	}

	if err := te.messenger.SendWithButtons(args.Text, args.Buttons); err != nil {
		return fmt.Sprintf("Error sending message: %s", err)
	}
	te.logger.Info("message with buttons sent to collaborator",
		"length", len(args.Text),
		"button_rows", len(args.Buttons),
	)
	return "Message sent to collaborator."
}

func (te *Executor) sendRichMessage(argsJSON string) string {
	var args messaging.RichMessage
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %s", err)
	}
	if strings.TrimSpace(args.Content) == "" && strings.TrimSpace(args.Title) == "" && len(args.Fields) == 0 {
		return "Error: content is required"
	}
	if te.messenger == nil {
		return "Error: messaging is not configured. No collaborator channel available."
	}
	if err := te.messenger.SendRich(args); err != nil {
		return fmt.Sprintf("Error sending rich message: %s", err)
	}
	te.logger.Info("rich message sent to collaborator", "length", len(args.Content))
	return "Rich message sent to collaborator."
}

func (te *Executor) sendVoiceMessage(argsJSON string) string {
	var args struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %s", err)
	}
	if strings.TrimSpace(args.Text) == "" {
		return "Error: text is required"
	}
	if te.voice == nil {
		return "Error: voice replies are not configured. Enable [deepgram] with Discord."
	}
	if err := te.voice.SendVoice(args.Text); err != nil {
		return fmt.Sprintf("Error sending voice message: %s", err)
	}
	te.logger.Info("voice message sent to collaborator", "length", len(args.Text))
	return "Voice message sent to collaborator."
}

func (te *Executor) sendFile(argsJSON string) string {
	var args struct {
		Path     string `json:"path"`
		Filename string `json:"filename"`
		Text     string `json:"text"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %s", err)
	}
	if te.files == nil {
		return "Error: send_file requires Discord with sandbox enabled."
	}
	if te.sandbox == nil {
		return "Error: sandbox is not configured"
	}
	hostPath, err := resolveSandboxFilePath(te.sandbox.Dir(), args.Path)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	if err := te.files.SendFile(hostPath, args.Filename, args.Text); err != nil {
		return fmt.Sprintf("Error sending file: %s", err)
	}
	te.logger.Info("file sent to collaborator", "path", args.Path, "host", hostPath)
	return fmt.Sprintf("File sent to collaborator (%s).", filepath.Base(hostPath))
}

// resolveSandboxFilePath maps container-style sandbox paths to host paths.
// Allowed roots: /output, /input, /scripts (and bare relative under those).
func resolveSandboxFilePath(root, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("sandbox is not configured")
	}
	p := filepath.Clean(raw)
	p = strings.ReplaceAll(p, `\`, `/`)

	var rel string
	switch {
	case strings.HasPrefix(p, "/output/") || p == "/output":
		rel = strings.TrimPrefix(p, "/")
	case strings.HasPrefix(p, "/input/") || p == "/input":
		rel = strings.TrimPrefix(p, "/")
	case strings.HasPrefix(p, "/scripts/") || p == "/scripts":
		rel = strings.TrimPrefix(p, "/")
	case strings.HasPrefix(p, "output/") || strings.HasPrefix(p, "input/") || strings.HasPrefix(p, "scripts/"):
		rel = p
	default:
		return "", fmt.Errorf("path must be under /output, /input, or /scripts")
	}

	host := filepath.Join(root, filepath.FromSlash(rel))
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("sandbox root: %w", err)
	}
	absHost, err := filepath.Abs(host)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if absHost != absRoot && !strings.HasPrefix(absHost, absRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes sandbox")
	}
	st, err := os.Stat(absHost)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", raw)
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("path is not a regular file")
	}
	return absHost, nil
}

// sleep suspends the agent for the requested number of seconds, returning
// early if the operator sends a message or the process is shutting down.
//
// The handler does not drain the collaborator queue; it only peeks. The agent
// loop's existing between-turn drain (Agent.injectPendingMessages) is the
// single owner of the message queue. Returning to the agent loop with a
// pending message in place causes it to be surfaced on the next turn just
// like any message that arrived while no tool was running.
//
// Polling at 500ms is intentional: minute-scale sleeps don't care about
// sub-second wake latency, and avoiding a notify channel keeps the
// messenger surface area small.
func (te *Executor) sleep(ctx context.Context, argsJSON string) string {
	var args struct {
		Seconds int `json:"seconds"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %s", err)
	}

	if args.Seconds <= 0 {
		return "Error: seconds must be a positive integer."
	}

	requested := time.Duration(args.Seconds) * time.Second
	target := requested
	var clampNote string
	if te.maxSleep > 0 && target > te.maxSleep {
		clampNote = fmt.Sprintf("Requested %s exceeds the configured maximum %s; clamped. ", requested, te.maxSleep)
		target = te.maxSleep
	}

	// If a collaborator or peer message is already queued at entry, do
	// not sleep through it. The agent should respond before doing
	// anything else.
	if te.messenger != nil && te.messenger.HasPending() {
		te.logger.Info("sleep skipped: collaborator message already pending",
			"requested_s", args.Seconds)
		return clampNote + "Did not sleep: a collaborator message is already pending. Handle it before sleeping."
	}
	if te.peers != nil && te.peers.HasPending() {
		te.logger.Info("sleep skipped: peer message already pending",
			"requested_s", args.Seconds)
		return clampNote + "Did not sleep: a peer message is already pending. Handle it before sleeping."
	}
	if inbox := te.daemonAlertInbox(); inbox != nil && inbox.HasPending() {
		te.logger.Info("sleep skipped: daemon alert already pending",
			"requested_s", args.Seconds)
		return clampNote + "Did not sleep: a daemon alert is already pending. Handle it before sleeping."
	}

	te.logger.Info("sleep started", "requested_s", args.Seconds, "actual_s", int(target.Seconds()))

	const pollInterval = 500 * time.Millisecond
	start := time.Now()
	deadline := start.Add(target)

	timer := time.NewTimer(target)
	defer timer.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			elapsed := time.Since(start).Round(time.Second)
			te.logger.Info("sleep interrupted by shutdown", "elapsed_s", int(elapsed.Seconds()))
			return clampNote + fmt.Sprintf("Slept for %s then interrupted: shutdown.", elapsed)

		case <-timer.C:
			elapsed := time.Since(start).Round(time.Second)
			te.logger.Info("sleep completed", "elapsed_s", int(elapsed.Seconds()))
			return clampNote + fmt.Sprintf("Slept for %s.", elapsed)

		case <-ticker.C:
			if te.messenger != nil && te.messenger.HasPending() {
				elapsed := time.Since(start).Round(time.Second)
				te.logger.Info("sleep interrupted by collaborator message", "elapsed_s", int(elapsed.Seconds()))
				return clampNote + fmt.Sprintf("Slept for %s then interrupted: collaborator message pending.", elapsed)
			}
			if te.peers != nil && te.peers.HasPending() {
				elapsed := time.Since(start).Round(time.Second)
				te.logger.Info("sleep interrupted by peer message", "elapsed_s", int(elapsed.Seconds()))
				return clampNote + fmt.Sprintf("Slept for %s then interrupted: peer message pending.", elapsed)
			}
			if inbox := te.daemonAlertInbox(); inbox != nil && inbox.HasPending() {
				elapsed := time.Since(start).Round(time.Second)
				te.logger.Info("sleep interrupted by daemon alert", "elapsed_s", int(elapsed.Seconds()))
				return clampNote + fmt.Sprintf("Slept for %s then interrupted: daemon alert pending.", elapsed)
			}
			// Belt-and-braces: if the timer fires between selects somehow,
			// still exit at the deadline rather than oversleeping.
			if !time.Now().Before(deadline) {
				elapsed := time.Since(start).Round(time.Second)
				return clampNote + fmt.Sprintf("Slept for %s.", elapsed)
			}
		}
	}
}
