package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/CamiloValderruten/faultline/internal/llm"
)

func TestSubmitTurnBusy(t *testing.T) {
	q := newLocalTurns()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	started := make(chan struct{})
	go func() {
		close(started)
		_, _ = q.submit(ctx, "first")
	}()
	<-started
	// Give the first submit time to park on pending.
	deadline := time.Now().Add(time.Second)
	for !q.HasPending() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !q.HasPending() {
		t.Fatal("expected pending turn")
	}
	if _, err := q.submit(context.Background(), "second"); err != ErrTurnBusy {
		t.Fatalf("err=%v want ErrTurnBusy", err)
	}
	req := q.takePending()
	completeLocalTurn(req, "ok", nil)
}

func TestRunLocalTurnReturnsAssistantText(t *testing.T) {
	chat := &scriptedChat{responses: []*llm.ChatResponse{
		textOnlyResponse("kitchen lights are on"),
	}}
	tools := &recordingTools{results: map[string]string{
		"send_message": "Message sent to collaborator.",
	}}
	a := newDeliveryDebtAgent(chat, tools, &scriptedOperator{}, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type result struct {
		reply string
		err   error
	}
	got := make(chan result, 1)
	go func() {
		reply, err := a.SubmitTurn(ctx, "are the lights on?")
		got <- result{reply, err}
	}()
	deadline := time.Now().Add(time.Second)
	for !a.LocalTurnPending() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !a.LocalTurnPending() {
		t.Fatal("expected queued local turn")
	}
	if err := a.Run(ctx, make(chan struct{})); err != nil {
		t.Fatalf("Run: %v", err)
	}
	res := <-got
	if res.err != nil {
		t.Fatalf("SubmitTurn: %v", res.err)
	}
	if res.reply != "kitchen lights are on" {
		t.Fatalf("reply=%q", res.reply)
	}
	for _, name := range tools.executed {
		if name == "send_message" {
			t.Fatal("local turn must not call send_message")
		}
	}
	var sawLocal bool
	for _, m := range chat.seen {
		if m.Role == llm.RoleUser && strings.Contains(m.Content, "USB headset") {
			sawLocal = true
		}
	}
	if !sawLocal {
		t.Fatal("expected local headset user message in chat")
	}
}

func TestRunLocalTurnDoesNotOpenDeliveryDebt(t *testing.T) {
	chat := &scriptedChat{responses: []*llm.ChatResponse{
		textOnlyResponse("done"),
		textOnlyResponse("still idle"),
	}}
	tools := &recordingTools{}
	a := newDeliveryDebtAgent(chat, tools, &scriptedOperator{}, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := a.SubmitTurn(ctx, "ping")
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for !a.LocalTurnPending() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := a.Run(ctx, make(chan struct{})); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("SubmitTurn: %v", err)
	}
	for _, m := range chat.seen {
		if m.Role == llm.RoleUser && strings.Contains(m.Content, "NOT delivered") {
			t.Fatalf("delivery-debt nudge leaked into local turn:\n%s", m.Content)
		}
	}
}
