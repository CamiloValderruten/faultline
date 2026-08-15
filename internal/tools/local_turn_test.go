package tools

import (
	"context"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/llm"
	"github.com/CamiloValderruten/faultline/internal/messaging"
)

type stubMessenger struct{}

func (stubMessenger) Send(string) error { return nil }
func (stubMessenger) SendWithButtons(string, [][]messaging.Button) error {
	return nil
}
func (stubMessenger) SendRich(messaging.RichMessage) error { return nil }
func (stubMessenger) HasPending() bool                     { return false }
func (stubMessenger) ChannelGuide() string                 { return "guide" }

func TestLocalTurnStripsAndRejectsDiscordTools(t *testing.T) {
	te := New(Deps{
		Mode:      ModePrimary,
		Logger:    silentTestLogger(),
		Messenger: stubMessenger{},
	})
	var advertised bool
	for _, d := range te.ToolDefs() {
		if d.Function != nil && d.Function.Name == "send_message" {
			advertised = true
		}
	}
	if !advertised {
		t.Fatal("expected send_message advertised before local turn")
	}

	te.SetLocalTurn(true)
	for _, d := range te.ToolDefs() {
		if d.Function == nil {
			continue
		}
		if _, banned := localTurnForbidden[d.Function.Name]; banned {
			t.Fatalf("local turn advertised forbidden tool %q", d.Function.Name)
		}
	}
	got := te.Execute(context.Background(), llm.ToolCall{
		ID: "x",
		Function: llm.FunctionCall{
			Name:      "send_message",
			Arguments: `{"text":"hi"}`,
		},
	})
	if got == "" || got[:5] != "Tool " {
		t.Fatalf("reject=%q", got)
	}
	te.SetLocalTurn(false)
	var restored bool
	for _, d := range te.ToolDefs() {
		if d.Function != nil && d.Function.Name == "send_message" {
			restored = true
		}
	}
	if !restored {
		t.Fatal("expected send_message restored after local turn")
	}
}
