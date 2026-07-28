package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/CamiloValderruten/faultline/internal/messaging"
)

// voiceSender is optionally implemented by the Discord messenger when Deepgram
// TTS is configured.
type voiceSender interface {
	SendVoice(text string) error
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

	// If a collaborator message is already queued at entry, do not sleep
	// through it. The agent should respond before doing anything else.
	if te.messenger != nil && te.messenger.HasPending() {
		te.logger.Info("sleep skipped: collaborator message already pending",
			"requested_s", args.Seconds)
		return clampNote + "Did not sleep: a collaborator message is already pending. Handle it before sleeping."
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
			// Belt-and-braces: if the timer fires between selects somehow,
			// still exit at the deadline rather than oversleeping.
			if !time.Now().Before(deadline) {
				elapsed := time.Since(start).Round(time.Second)
				return clampNote + fmt.Sprintf("Slept for %s.", elapsed)
			}
		}
	}
}
