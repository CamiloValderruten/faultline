package discord

import (
	"strings"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	"github.com/bwmarrin/discordgo"
)

func TestBuildComponents_ButtonsAndSelect(t *testing.T) {
	comps, err := buildComponents(
		[][]messaging.Button{
			{{Text: "Approve", Data: "approve", Style: "success"}, {Text: "Deny", Data: "deny", Style: "danger"}},
		},
		[]messaging.SelectMenu{{
			ID:          "pick",
			Placeholder: "Choose",
			Options: []messaging.SelectOption{
				{Label: "One", Value: "1"},
				{Label: "Two", Value: "2", Description: "second"},
			},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 2 {
		t.Fatalf("rows=%d", len(comps))
	}
	row0, ok := comps[0].(discordgo.ActionsRow)
	if !ok || len(row0.Components) != 2 {
		t.Fatalf("row0=%T %+v", comps[0], comps[0])
	}
	btn, ok := row0.Components[0].(discordgo.Button)
	if !ok || btn.CustomID != "approve" || btn.Style != discordgo.SuccessButton {
		t.Fatalf("button=%+v", row0.Components[0])
	}
}

func TestBuildComponents_LinkButton(t *testing.T) {
	comps, err := buildComponents([][]messaging.Button{
		{{Text: "Docs", Style: "link", URL: "https://example.com"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	row := comps[0].(discordgo.ActionsRow)
	btn := row.Components[0].(discordgo.Button)
	if btn.Style != discordgo.LinkButton || btn.URL != "https://example.com" {
		t.Fatalf("btn=%+v", btn)
	}
}

func TestBuildComponents_TooManyRows(t *testing.T) {
	var buttons [][]messaging.Button
	for i := 0; i < 6; i++ {
		buttons = append(buttons, []messaging.Button{{Text: "B", Data: "d"}})
	}
	_, err := buildComponents(buttons, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatComponentPending_Button(t *testing.T) {
	got := formatComponentPending(discordgo.MessageComponentInteractionData{
		CustomID:      "approve",
		ComponentType: discordgo.ButtonComponent,
	})
	if !strings.Contains(got, "approve") {
		t.Fatalf("got=%q", got)
	}
}

func TestFormatComponentPending_Select(t *testing.T) {
	got := formatComponentPending(discordgo.MessageComponentInteractionData{
		CustomID:      "pick",
		ComponentType: discordgo.SelectMenuComponent,
		Values:        []string{"a", "b"},
	})
	want := `Selected menu "pick" (values=a,b)`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatPhotoNotice(t *testing.T) {
	got := formatPhotoNotice("/input/discord/x.jpg", "look")
	if !strings.Contains(got, "image_path: /input/discord/x.jpg") {
		t.Fatalf("got=%q", got)
	}
	if !strings.Contains(got, "caption: look") {
		t.Fatalf("got=%q", got)
	}
}
