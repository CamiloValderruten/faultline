package daemon

import "testing"

func TestInboxEnqueuePendingHasPending(t *testing.T) {
	in := NewInbox(2)
	if in.HasPending() {
		t.Fatal("expected empty")
	}
	in.Enqueue(Alert{DaemonID: "a", Name: "n", Text: "one"})
	in.Enqueue(Alert{DaemonID: "a", Name: "n", Text: "two"})
	in.Enqueue(Alert{DaemonID: "a", Name: "n", Text: "three"}) // drops one
	if !in.HasPending() {
		t.Fatal("expected pending")
	}
	got := in.Pending()
	if len(got) != 2 || got[0].Text != "two" || got[1].Text != "three" {
		t.Fatalf("got %+v", got)
	}
	if in.HasPending() || in.Pending() != nil {
		t.Fatal("expected drained")
	}
}
