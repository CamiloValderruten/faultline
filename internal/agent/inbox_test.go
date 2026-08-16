package agent

import (
	"strings"
	"testing"
)

func TestDrainHighestTakesOnlyTopBucketFIFO(t *testing.T) {
	q := newInbox()
	q.Push(Item{Bucket: BucketWebhook, Source: SourceWebhook, Text: "wh-1"})
	q.Push(Item{Bucket: BucketHuman, Source: SourceCollaborator, Text: "d-1"})
	q.Push(Item{Bucket: BucketDaemon, Source: SourceDaemon, Text: "cry"})
	q.Push(Item{Bucket: BucketHuman, Source: SourceCollaborator, Text: "d-2"})

	got := q.DrainHighest()
	if len(got) != 1 || got[0].Text != "cry" {
		t.Fatalf("highest=%v", texts(got))
	}
	got = q.DrainHighest()
	if len(got) != 2 || got[0].Text != "d-1" || got[1].Text != "d-2" {
		t.Fatalf("human=%v", texts(got))
	}
	got = q.DrainHighest()
	if len(got) != 1 || got[0].Text != "wh-1" {
		t.Fatalf("webhook=%v", texts(got))
	}
	if q.HasPending() {
		t.Fatal("expected empty")
	}
}

func TestDrainHighestCapsCount(t *testing.T) {
	q := newInbox()
	for i := 0; i < drainMaxItems+3; i++ {
		q.Push(Item{Bucket: BucketHuman, Source: SourceCollaborator, Text: strings.Repeat("x", 8) + string(rune('a'+i))})
	}
	got := q.DrainHighest()
	if len(got) != drainMaxItems {
		t.Fatalf("len=%d want %d", len(got), drainMaxItems)
	}
	if !q.HasPending() {
		t.Fatal("overflow should remain queued")
	}
}

func TestDrainHighestCapsRunes(t *testing.T) {
	q := newInbox()
	for i := 0; i < drainMaxRunes/itemMaxRunes+1; i++ {
		q.Push(Item{Bucket: BucketHuman, Source: SourceCollaborator, Text: strings.Repeat("a", itemMaxRunes)})
	}
	got := q.DrainHighest()
	if len(got) != drainMaxRunes/itemMaxRunes {
		t.Fatalf("len=%d want %d", len(got), drainMaxRunes/itemMaxRunes)
	}
	if !q.HasPending() {
		t.Fatal("oversize remainder should stay queued")
	}
}

func TestDrainInterruptP0EvenIfWaitForTools(t *testing.T) {
	q := newInbox()
	q.Push(Item{Bucket: BucketHuman, Source: SourceCollaborator, Text: "hi"})
	q.Push(Item{Bucket: BucketDaemon, Source: SourceDaemon, Text: "cry"})
	got := q.DrainInterrupt(true)
	if len(got) != 1 || got[0].Text != "cry" {
		t.Fatalf("got=%v", texts(got))
	}
}

func TestDrainInterruptSkipsP1WhenWaitForTools(t *testing.T) {
	q := newInbox()
	q.Push(Item{Bucket: BucketHuman, Source: SourceCollaborator, Text: "hi"})
	if got := q.DrainInterrupt(true); len(got) != 0 {
		t.Fatalf("got=%v", texts(got))
	}
	if !q.HasPending() {
		t.Fatal("P1 should stay queued")
	}
	got := q.DrainInterrupt(false)
	if len(got) != 1 || got[0].Text != "hi" {
		t.Fatalf("got=%v", texts(got))
	}
}

func TestUrgentWebhookIsHumanBucket(t *testing.T) {
	q := newInbox()
	q.Push(Item{Bucket: BucketWebhook, Source: SourceWebhook, Text: "low"})
	q.Push(Item{Bucket: BucketHuman, Source: SourceWebhook, Text: "page", Urgent: true})
	got := q.DrainHighest()
	if len(got) != 1 || got[0].Text != "page" || !got[0].Urgent {
		t.Fatalf("got=%v", got)
	}
	if batchHasCollaborator(got) {
		t.Fatal("urgent webhook must not open delivery debt")
	}
}

func TestPushIgnoresEmpty(t *testing.T) {
	q := newInbox()
	q.Push(Item{Bucket: BucketHuman, Source: SourceCollaborator, Text: "  "})
	if q.HasPending() {
		t.Fatal("empty")
	}
}

func TestPushDropsLowestBucketOnOverflow(t *testing.T) {
	q := newInbox()
	for i := 0; i < inboxMaxItems; i++ {
		q.Push(Item{Bucket: BucketWebhook, Source: SourceWebhook, Text: "wh"})
	}
	q.Push(Item{Bucket: BucketDaemon, Source: SourceDaemon, Text: "cry"})
	got := q.DrainHighest()
	if len(got) != 1 || got[0].Text != "cry" {
		t.Fatalf("overflow must drop P4, not the cry; got=%v", texts(got))
	}
}

func texts(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Text
	}
	return out
}
