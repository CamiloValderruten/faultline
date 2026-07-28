package discord

import (
	"encoding/base64"
	"testing"
)

func TestWaveformFromBytes(t *testing.T) {
	got := waveformFromBytes([]byte{1, 2, 255, 10})
	raw, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 256 {
		t.Fatalf("len=%d", len(raw))
	}
}

func TestFallbackOggDuration(t *testing.T) {
	if d := fallbackOggDuration(0); d < 0.5 {
		t.Fatalf("d=%v", d)
	}
	if d := fallbackOggDuration(24000); d < 7 || d > 9 {
		t.Fatalf("d=%v", d)
	}
}
