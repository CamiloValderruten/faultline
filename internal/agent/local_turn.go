package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

// ErrTurnBusy is returned when a headset turn is already in flight or queued.
var ErrTurnBusy = errors.New("local turn already in progress")

// localTurnPreamble is injected as the user message for USB-headset turns.
// Assistant text is the spoken reply; Discord send_* tools are stripped.
const localTurnPreamble = `[Local headset turn - %s]

You are speaking aloud over a USB headset. Discord is silent for this turn.
Do not call send_message, send_rich_message, send_voice_message, send_file, or any subagent tool.
Your final assistant text (no tool calls) is spoken to the headset. Keep it concise spoken language, under 2000 characters.

They said: %s`

type localTurnResult struct {
	text string
	err  error
}

type localTurnRequest struct {
	text string
	done chan localTurnResult
}

type localTurns struct {
	mu       sync.Mutex
	pending  *localTurnRequest
	inflight bool
}

func newLocalTurns() *localTurns {
	return &localTurns{}
}

func (q *localTurns) HasPending() bool {
	if q == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pending != nil
}

func (q *localTurns) submit(ctx context.Context, text string) (string, error) {
	text = trimLocalTurnText(text)
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	req := &localTurnRequest{text: text, done: make(chan localTurnResult, 1)}
	q.mu.Lock()
	if q.pending != nil || q.inflight {
		q.mu.Unlock()
		return "", ErrTurnBusy
	}
	q.pending = req
	q.mu.Unlock()

	select {
	case <-ctx.Done():
		q.abandon(req)
		return "", ctx.Err()
	case res := <-req.done:
		return res.text, res.err
	}
}

func (q *localTurns) takePending() *localTurnRequest {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	req := q.pending
	if req == nil {
		return nil
	}
	q.pending = nil
	q.inflight = true
	return req
}

func (q *localTurns) abandon(req *localTurnRequest) {
	if q == nil || req == nil {
		return
	}
	q.mu.Lock()
	if q.pending == req {
		q.pending = nil
	}
	q.mu.Unlock()
}

func (q *localTurns) clearInflight() {
	if q == nil {
		return
	}
	q.mu.Lock()
	q.inflight = false
	q.mu.Unlock()
}

func completeLocalTurn(req *localTurnRequest, text string, err error) {
	if req == nil {
		return
	}
	select {
	case req.done <- localTurnResult{text: text, err: err}:
	default:
	}
}

func trimLocalTurnText(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	const maxRunes = 4000
	if len(runes) > maxRunes {
		text = string(runes[:maxRunes])
	}
	return text
}

func localTurnUserMessage(text string) llm.Message {
	return llm.Message{
		Role:    llm.RoleUser,
		Content: fmt.Sprintf(localTurnPreamble, time.Now().Format(time.RFC1123), text),
	}
}

type localTurnGate interface {
	SetLocalTurn(active bool)
}

func (a *Agent) finishLocalTurn(text string) {
	req := a.localInFlight
	a.localInFlight = nil
	a.setLocalTurnTools(false)
	a.localTurns.clearInflight()
	completeLocalTurn(req, strings.TrimSpace(text), nil)
}

func (a *Agent) failLocalTurn(err error) {
	if a == nil || a.localInFlight == nil {
		return
	}
	req := a.localInFlight
	a.localInFlight = nil
	a.setLocalTurnTools(false)
	a.localTurns.clearInflight()
	completeLocalTurn(req, "", err)
}

func (a *Agent) setLocalTurnTools(active bool) {
	if g, ok := a.tools.(localTurnGate); ok {
		g.SetLocalTurn(active)
	}
}

// SubmitTurn queues a headset transcript into the running primary loop and
// blocks until that loop produces a final assistant text (no tool calls).
func (a *Agent) SubmitTurn(ctx context.Context, text string) (string, error) {
	if a == nil || a.localTurns == nil {
		return "", fmt.Errorf("local turns are not available")
	}
	return a.localTurns.submit(ctx, text)
}

// LocalTurnPending reports whether a headset turn is waiting to be injected.
// Used by sleep to wake without draining the queue.
func (a *Agent) LocalTurnPending() bool {
	return a != nil && a.localTurns.HasPending()
}
