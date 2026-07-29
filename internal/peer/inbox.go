// Package peer implements a pull-only mailbox for cross-process Faultline
// agents. Incoming messages are stored on disk and never injected into the
// agent loop — the model spends tokens only when it calls peer_* tools.
package peer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Message is one peer inbox entry.
type Message struct {
	ID         string    `json:"id"`
	From       string    `json:"from"`
	Text       string    `json:"text"`
	ReceivedAt time.Time `json:"received_at"`
}

// Inbox is a mutex-guarded JSON file of unread peer messages.
type Inbox struct {
	path    string
	maxSize int
	mu      sync.Mutex
}

// NewInbox opens (or will create) a peer inbox at path. maxSize caps
// retained messages; oldest are dropped on overflow. maxSize <= 0 defaults
// to 100.
func NewInbox(path string, maxSize int) *Inbox {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &Inbox{path: path, maxSize: maxSize}
}

// Enqueue appends a message, dropping oldest entries when over capacity.
func (in *Inbox) Enqueue(from, text string) (Message, error) {
	in.mu.Lock()
	defer in.mu.Unlock()

	from = strings.TrimSpace(from)
	text = strings.TrimSpace(text)
	if from == "" {
		return Message{}, errors.New("from is required")
	}
	if text == "" {
		return Message{}, errors.New("text is required")
	}

	msgs, err := in.loadLocked()
	if err != nil {
		return Message{}, err
	}
	msg := Message{
		ID:         newID(),
		From:       from,
		Text:       text,
		ReceivedAt: time.Now().UTC(),
	}
	msgs = append(msgs, msg)
	if len(msgs) > in.maxSize {
		// ponytail: drop oldest; per-sender quotas if spam becomes real
		msgs = msgs[len(msgs)-in.maxSize:]
	}
	if err := in.saveLocked(msgs); err != nil {
		return Message{}, err
	}
	return msg, nil
}

// List returns unread messages oldest-first.
func (in *Inbox) List() ([]Message, error) {
	in.mu.Lock()
	defer in.mu.Unlock()
	msgs, err := in.loadLocked()
	if err != nil {
		return nil, err
	}
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].ReceivedAt.Before(msgs[j].ReceivedAt)
	})
	return msgs, nil
}

// Read removes and returns the message with the given id.
func (in *Inbox) Read(id string) (Message, error) {
	in.mu.Lock()
	defer in.mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return Message{}, errors.New("id is required")
	}
	msgs, err := in.loadLocked()
	if err != nil {
		return Message{}, err
	}
	for i, msg := range msgs {
		if msg.ID == id {
			out := append([]Message{}, msgs[:i]...)
			out = append(out, msgs[i+1:]...)
			if err := in.saveLocked(out); err != nil {
				return Message{}, err
			}
			return msg, nil
		}
	}
	return Message{}, fmt.Errorf("message %q not found", id)
}

func (in *Inbox) loadLocked() ([]Message, error) {
	if in.path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(in.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var msgs []Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (in *Inbox) saveLocked(msgs []Message) error {
	if in.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(in.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return err
	}
	tmp := in.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, in.path)
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("peer-%d", time.Now().UnixNano())
	}
	return "peer-" + hex.EncodeToString(b[:])
}
