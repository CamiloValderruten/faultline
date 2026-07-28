package deepgram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranscribe_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/listen" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-key" {
			t.Fatalf("auth=%q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("model") != "nova-3" {
			t.Fatalf("model=%q", r.URL.Query().Get("model"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "AUDIO" {
			t.Fatalf("body=%q", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":{"channels":[{"alternatives":[{"transcript":"hello world"}]}]}}`))
	}))
	defer srv.Close()

	c, err := New("test-key", "", "")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL
	c.http = srv.Client()

	got, err := c.Transcribe(context.Background(), []byte("AUDIO"), "audio/ogg")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestSpeak_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/speak" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("encoding") != "mp3" {
			t.Fatalf("encoding=%q", r.URL.Query().Get("encoding"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hi there") {
			t.Fatalf("body=%s", body)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("MP3DATA"))
	}))
	defer srv.Close()

	c, err := New("test-key", "", "aura-2-thalia-en")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL
	c.http = srv.Client()

	audio, ct, err := c.Speak(context.Background(), "hi there")
	if err != nil {
		t.Fatal(err)
	}
	if ct != "audio/mpeg" || string(audio) != "MP3DATA" {
		t.Fatalf("audio=%q ct=%q", audio, ct)
	}
}

func TestNew_RequiresKey(t *testing.T) {
	if _, err := New("", "", ""); err == nil {
		t.Fatal("expected error")
	}
}
