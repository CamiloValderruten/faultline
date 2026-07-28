package discord

import (
	"strings"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	"github.com/bwmarrin/discordgo"
)

func TestPickVoiceAttachment_VoiceFlag(t *testing.T) {
	msg := &discordgo.Message{
		Flags: discordgo.MessageFlagsIsVoiceMessage,
		Attachments: []*discordgo.MessageAttachment{
			{Filename: "voice-message.ogg", ContentType: "audio/ogg", URL: "https://example.com/v.ogg"},
		},
	}
	att := pickVoiceAttachment(msg)
	if att == nil || att.Filename != "voice-message.ogg" {
		t.Fatalf("att=%+v", att)
	}
}

func TestPickVoiceAttachment_AudioByExt(t *testing.T) {
	msg := &discordgo.Message{
		Attachments: []*discordgo.MessageAttachment{
			{Filename: "note.mp3", URL: "https://example.com/n.mp3"},
		},
	}
	if pickVoiceAttachment(msg) == nil {
		t.Fatal("expected audio attachment")
	}
}

func TestStripForSpeech(t *testing.T) {
	got := stripForSpeech("Hello **world** see https://example.com now")
	if strings.Contains(got, "**") || strings.Contains(got, "https://") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Fatalf("got %q", got)
	}
}

func TestVoiceNotePreambleUsed(t *testing.T) {
	if !strings.Contains(messaging.VoiceNotePreamble, "send_voice_message") {
		t.Fatal("preamble should mention send_voice_message")
	}
}
