package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/llm"
	"github.com/CamiloValderruten/faultline/internal/peer"
)

func TestPeerToolDefsWhenConfigured(t *testing.T) {
	mb := peer.NewMailbox("alice", peer.NewInbox(filepath.Join(t.TempDir(), "inbox.json"), 10), []peer.Agent{
		{Name: "bob", URL: "http://127.0.0.1:9102", Token: "x"},
	})
	te := New(Deps{Logger: silentTestLogger(), Peers: mb})
	names := toolDefNames(te.ToolDefs())
	for _, name := range []string{"peer_send", "peer_inbox", "peer_read"} {
		if !names[name] {
			t.Fatalf("expected %s advertised", name)
		}
	}
}

func TestPeerInboxAndRead(t *testing.T) {
	inbox := peer.NewInbox(filepath.Join(t.TempDir(), "inbox.json"), 10)
	msg, err := inbox.Enqueue("bob", "hello from bob")
	if err != nil {
		t.Fatal(err)
	}
	mb := peer.NewMailbox("alice", inbox, nil)
	te := New(Deps{Logger: silentTestLogger(), Peers: mb})

	listed := te.Execute(context.Background(), llm.ToolCall{
		Function: llm.FunctionCall{Name: "peer_inbox", Arguments: `{}`},
	})
	if !strings.Contains(listed, msg.ID) || !strings.Contains(listed, "bob") {
		t.Fatalf("peer_inbox = %s", listed)
	}

	args, _ := json.Marshal(map[string]string{"id": msg.ID})
	got := te.Execute(context.Background(), llm.ToolCall{
		Function: llm.FunctionCall{Name: "peer_read", Arguments: string(args)},
	})
	if !strings.Contains(got, "hello from bob") {
		t.Fatalf("peer_read = %s", got)
	}

	empty := te.Execute(context.Background(), llm.ToolCall{
		Function: llm.FunctionCall{Name: "peer_inbox", Arguments: `{}`},
	})
	if empty != "[]" {
		t.Fatalf("expected empty inbox, got %s", empty)
	}
}
