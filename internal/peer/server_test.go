package peer

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestMailboxSendRoundTrip(t *testing.T) {
	dir := t.TempDir()
	inbox := NewInbox(filepath.Join(dir, "inbox.json"), 10)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv, err := NewServer(addr, "secret-b", inbox, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.Start(ctx)
	defer srv.Shutdown()

	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}

	mb := NewMailbox("alice", NewInbox(filepath.Join(dir, "unused.json"), 10), []Agent{
		{Name: "bob", URL: "http://" + addr, Token: "secret-b"},
	})
	id, err := mb.Send(context.Background(), "bob", "ping")
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected message id")
	}

	list, err := inbox.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].From != "alice" || list[0].Text != "ping" {
		t.Fatalf("inbox=%+v", list)
	}
}

func TestServerRejectsBadToken(t *testing.T) {
	dir := t.TempDir()
	inbox := NewInbox(filepath.Join(dir, "inbox.json"), 10)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv, err := NewServer(addr, "secret-b", inbox, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.Start(ctx)
	defer srv.Shutdown()

	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}

	mb := NewMailbox("alice", NewInbox(filepath.Join(dir, "unused.json"), 10), []Agent{
		{Name: "bob", URL: "http://" + addr, Token: "wrong"},
	})
	if _, err := mb.Send(context.Background(), "bob", "ping"); err == nil {
		t.Fatal("expected auth error")
	}
}
