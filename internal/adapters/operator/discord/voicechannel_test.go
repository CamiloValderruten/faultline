package discord

import (
	"testing"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	"github.com/pion/webrtc/v3/pkg/media/oggreader"
	"bytes"
	"io"
)

func TestMuxOpusPacketsToOgg_RoundTripPages(t *testing.T) {
	// Minimal fake opus payloads (not real audio; container only).
	packets := [][]byte{
		{0x01, 0x02, 0x03},
		{0x04, 0x05},
	}
	ogg, err := muxOpusPacketsToOgg(packets)
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := oggreader.NewWith(bytes.NewReader(ogg))
	if err != nil {
		t.Fatal(err)
	}
	var pages int
	for {
		page, _, err := r.ParseNextPage()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(page) > 0 {
			pages++
		}
	}
	if pages < 1 {
		t.Fatalf("expected audio pages, got %d", pages)
	}
}

func TestAckOgg_Readable(t *testing.T) {
	if len(ackOgg) == 0 {
		t.Fatal("ack.ogg not embedded")
	}
	r, _, err := oggreader.NewWith(bytes.NewReader(ackOgg))
	if err != nil {
		t.Fatal(err)
	}
	page, _, err := r.ParseNextPage()
	if err != nil {
		t.Fatal(err)
	}
	if len(page) == 0 {
		t.Fatal("expected first audio page")
	}
}

func TestVoiceChannelPreamble(t *testing.T) {
	if !containsAll(messaging.VoiceChannelPreamble, "send_voice_message", "voice channel") {
		t.Fatalf("preamble=%q", messaging.VoiceChannelPreamble)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !bytes.Contains([]byte(s), []byte(p)) {
			return false
		}
	}
	return true
}
