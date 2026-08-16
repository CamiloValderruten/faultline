package agent

import (
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	// ponytail: in-memory only; persist if webhooks must survive restart.
	inboxMaxItems = 200
	drainMaxItems = 8
	drainMaxRunes = 16000
	itemMaxRunes  = 4000
)

// Bucket is inject priority. Lower wins. FIFO within a bucket.
type Bucket int

const (
	BucketDaemon  Bucket = iota // P0 cry / health
	BucketHuman                 // P1 Discord + urgent webhook
	BucketWork                  // P2 subagent / cron
	BucketPeer                  // P3
	BucketWebhook               // P4
	bucketCount
)

// Source is who produced an inbox item. Delivery debt follows SourceCollaborator only.
type Source string

const (
	SourceCollaborator Source = "collaborator"
	SourceDaemon       Source = "daemon"
	SourceSubagent     Source = "subagent"
	SourceScheduled    Source = "scheduled"
	SourcePeer         Source = "peer"
	SourceWebhook      Source = "webhook"
)

// Item is one inbound event waiting to become a user turn.
type Item struct {
	Bucket    Bucket
	Source    Source
	Text      string
	Label     string // daemon name, peer from, task title
	ID        string
	Urgent    bool
	Kind      string // schedule kind
	Truncated bool
	Canceled  bool
	Err       string
}

// Inbox is the unified agent queue. Adapters flush into it at drain time;
// the webhook handler Pushes directly.
//
// ponytail: no aging; P4 waits behind a chatty P1 until that bucket empties.
type Inbox struct {
	mu    sync.Mutex
	items []Item
}

func newInbox() *Inbox {
	return &Inbox{}
}

// Push appends an item. Empty text is ignored.
func (q *Inbox) Push(it Item) {
	if q == nil {
		return
	}
	it.Text = trimInboxText(it.Text)
	if it.Text == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, it)
	for len(q.items) > inboxMaxItems {
		q.dropLowestLocked()
	}
}

func (q *Inbox) HasPending() bool {
	if q == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items) > 0
}

// DrainHighest takes a bounded FIFO snapshot of the highest-priority
// nonempty bucket. Other buckets stay queued.
func (q *Inbox) DrainHighest() []Item {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	b, ok := q.highestLocked()
	if !ok {
		return nil
	}
	return q.takeBucketLocked(b)
}

// DrainInterrupt is the post-Chat inject: P0 always; P1 only when
// waitForTools is false. P2–P4 never interrupt mid-turn.
func (q *Inbox) DrainInterrupt(waitForTools bool) []Item {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.bucketHasLocked(BucketDaemon) {
		return q.takeBucketLocked(BucketDaemon)
	}
	if !waitForTools && q.bucketHasLocked(BucketHuman) {
		return q.takeBucketLocked(BucketHuman)
	}
	return nil
}

func (q *Inbox) highestLocked() (Bucket, bool) {
	for b := Bucket(0); b < bucketCount; b++ {
		if q.bucketHasLocked(b) {
			return b, true
		}
	}
	return 0, false
}

func (q *Inbox) bucketHasLocked(b Bucket) bool {
	for _, it := range q.items {
		if it.Bucket == b {
			return true
		}
	}
	return false
}

func (q *Inbox) takeBucketLocked(b Bucket) []Item {
	var out []Item
	var rest []Item
	runes := 0
	for _, it := range q.items {
		if it.Bucket != b {
			rest = append(rest, it)
			continue
		}
		if len(out) >= drainMaxItems || runes+utf8.RuneCountInString(it.Text) > drainMaxRunes {
			rest = append(rest, it)
			continue
		}
		out = append(out, it)
		runes += utf8.RuneCountInString(it.Text)
	}
	q.items = rest
	return out
}

func (q *Inbox) dropLowestLocked() {
	worst := Bucket(-1)
	idx := -1
	for i, it := range q.items {
		if it.Bucket >= worst {
			worst = it.Bucket
			idx = i
		}
	}
	if idx >= 0 {
		q.items = append(q.items[:idx], q.items[idx+1:]...)
	}
}

func trimInboxText(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) > itemMaxRunes {
		text = string(runes[:itemMaxRunes])
	}
	return text
}

func batchHasCollaborator(batch []Item) bool {
	for _, it := range batch {
		if it.Source == SourceCollaborator {
			return true
		}
	}
	return false
}
