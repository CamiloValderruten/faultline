package messaging

import (
	"strings"
	"testing"
)

func TestChannelGuides(t *testing.T) {
	for name, guide := range map[string]string{
		"telegram": TelegramChannelGuide,
		"discord":  DiscordChannelGuide,
	} {
		if !strings.Contains(guide, "## Collaborator Channel") {
			t.Fatalf("%s: missing header", name)
		}
		if !strings.Contains(guide, "send_message") {
			t.Fatalf("%s: missing send_message", name)
		}
		if !strings.Contains(guide, "send_rich_message") {
			t.Fatalf("%s: missing send_rich_message", name)
		}
	}
	if !strings.Contains(TelegramChannelGuide, "Telegram") {
		t.Fatal("telegram guide should name Telegram")
	}
	if !strings.Contains(DiscordChannelGuide, "Discord") {
		t.Fatal("discord guide should name Discord")
	}
	if !strings.Contains(DiscordChannelGuide, "sparingly") {
		t.Fatal("discord guide should discourage button spam")
	}
}
