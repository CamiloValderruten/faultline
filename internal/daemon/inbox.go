// Package daemon holds shared types for long-lived background daemons
// (alert inbox). The Docker sandbox adapter owns process lifecycle;
// the agent drains alerts between turns like peers/subagents.
package daemon

import "sync"

// Alert is one push notification from a daemon to the agent loop.
type Alert struct {
	DaemonID string
	Name     string
	Text     string
}

// Inbox is an in-memory queue of daemon alerts. Drained by the agent
// between turns; HasPending wakes sleep.
type Inbox struct {
	mu  sync.Mutex
	q   []Alert
	max int
}

// NewInbox returns an alert inbox. max <= 0 defaults to 100.
func NewInbox(max int) *Inbox {
	if max <= 0 {
		max = 100
	}
	return &Inbox{max: max}
}

// Enqueue appends an alert, dropping oldest on overflow.
func (i *Inbox) Enqueue(a Alert) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.q = append(i.q, a)
	if len(i.q) > i.max {
		i.q = i.q[len(i.q)-i.max:]
	}
}

// Pending drains and returns all queued alerts oldest-first.
func (i *Inbox) Pending() []Alert {
	if i == nil {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.q) == 0 {
		return nil
	}
	out := i.q
	i.q = nil
	return out
}

// HasPending reports whether any alerts are waiting.
func (i *Inbox) HasPending() bool {
	if i == nil {
		return false
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	return len(i.q) > 0
}
