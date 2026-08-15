package main

import (
	"encoding/binary"
	"testing"
)

func TestRMSInt16Silence(t *testing.T) {
	pcm := make([]byte, 640)
	if rmsInt16(pcm) != 0 {
		t.Fatalf("silence rms=%v", rmsInt16(pcm))
	}
}

func TestVADCapturesSpeechThenSilence(t *testing.T) {
	v := newVAD(100, 50)
	loud := make([]byte, frameSamples*2)
	for i := 0; i < frameSamples; i++ {
		binary.LittleEndian.PutUint16(loud[i*2:], uint16(int16(3000)))
	}
	quiet := make([]byte, frameSamples*2)

	var got []byte
	for i := 0; i < 30; i++ {
		if out := v.push(loud); out != nil {
			t.Fatal("ended during speech")
		}
	}
	for i := 0; i < silenceFrames+1; i++ {
		got = v.push(quiet)
		if got != nil {
			break
		}
	}
	if len(got) < minSpeechFrames*frameSamples*2 {
		t.Fatalf("utterance bytes=%d", len(got))
	}
}

func TestStripWakePrefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Alexa, how's Luca?", "how's Luca?"},
		{"alexa how are you", "how are you"},
		{"Alexa", ""},
		{"hey Alexa, lights", "lights"},
		{"how's Luca?", "how's Luca?"},
	}
	for _, tc := range cases {
		if got := stripWakePrefix(tc.in); got != tc.want {
			t.Errorf("stripWakePrefix(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestVADIgnoresClick(t *testing.T) {
	v := newVAD(100, 50)
	loud := make([]byte, frameSamples*2)
	for i := 0; i < frameSamples; i++ {
		binary.LittleEndian.PutUint16(loud[i*2:], uint16(int16(3000)))
	}
	quiet := make([]byte, frameSamples*2)
	v.push(loud)
	if out := v.push(quiet); out != nil {
		t.Fatal("single frame should not start an utterance")
	}
}
