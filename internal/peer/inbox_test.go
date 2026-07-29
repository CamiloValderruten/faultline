package peer

import (
	"path/filepath"
	"testing"
)

func TestInboxEnqueueListRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.json")
	in := NewInbox(path, 10)

	msg, err := in.Enqueue("bob", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID == "" || msg.From != "bob" || msg.Text != "hello" {
		t.Fatalf("unexpected message: %+v", msg)
	}

	list, err := in.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != msg.ID {
		t.Fatalf("list = %+v", list)
	}

	got, err := in.Read(msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "hello" {
		t.Fatalf("read = %+v", got)
	}
	list, err = in.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty after read, got %+v", list)
	}
}

func TestInboxDropsOldestWhenFull(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.json")
	in := NewInbox(path, 2)
	if _, err := in.Enqueue("a", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Enqueue("a", "two"); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Enqueue("a", "three"); err != nil {
		t.Fatal(err)
	}
	list, err := in.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d", len(list))
	}
	if list[0].Text != "two" || list[1].Text != "three" {
		t.Fatalf("expected two,three got %+v", list)
	}
}
