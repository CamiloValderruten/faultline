package peer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Agent is a known remote Faultline peer.
type Agent struct {
	Name  string
	URL   string
	Token string
}

// Mailbox is the tools-facing surface: send to configured peers and
// pull/ack local inbox messages. It does not wake the agent loop.
type Mailbox struct {
	Name   string
	Inbox  *Inbox
	Agents map[string]Agent
	Client *http.Client
}

// NewMailbox builds a mailbox. agents are keyed by lowercase name.
func NewMailbox(name string, inbox *Inbox, agents []Agent) *Mailbox {
	byName := make(map[string]Agent, len(agents))
	for _, a := range agents {
		n := strings.ToLower(strings.TrimSpace(a.Name))
		if n == "" {
			continue
		}
		a.Name = n
		a.URL = strings.TrimRight(strings.TrimSpace(a.URL), "/")
		byName[n] = a
	}
	return &Mailbox{
		Name:   strings.ToLower(strings.TrimSpace(name)),
		Inbox:  inbox,
		Agents: byName,
		Client: &http.Client{Timeout: 15 * time.Second},
	}
}

// PeerNames returns configured peer names sorted for tool descriptions.
func (m *Mailbox) PeerNames() []string {
	names := make([]string, 0, len(m.Agents))
	for n := range m.Agents {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

type sendBody struct {
	From string `json:"from"`
	Text string `json:"text"`
}

type sendResponse struct {
	ID string `json:"id"`
}

// Send POSTs text to a configured peer's /inbox.
func (m *Mailbox) Send(ctx context.Context, to, text string) (string, error) {
	to = strings.ToLower(strings.TrimSpace(to))
	text = strings.TrimSpace(text)
	if to == "" {
		return "", fmt.Errorf("to is required")
	}
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	agent, ok := m.Agents[to]
	if !ok {
		return "", fmt.Errorf("unknown peer %q", to)
	}
	if agent.URL == "" || agent.Token == "" {
		return "", fmt.Errorf("peer %q is missing url or token", to)
	}

	payload, err := json.Marshal(sendBody{From: m.Name, Text: text})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, agent.URL+"/inbox", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+agent.Token)

	resp, err := m.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("peer %s returned %s: %s", to, resp.Status, strings.TrimSpace(string(body)))
	}
	var out sendResponse
	if err := json.Unmarshal(body, &out); err != nil || out.ID == "" {
		return "", nil // delivered; id optional
	}
	return out.ID, nil
}

// List returns unread local inbox messages.
func (m *Mailbox) List() ([]Message, error) {
	return m.Inbox.List()
}

// Read acks (removes) and returns one local inbox message.
func (m *Mailbox) Read(id string) (Message, error) {
	return m.Inbox.Read(id)
}
