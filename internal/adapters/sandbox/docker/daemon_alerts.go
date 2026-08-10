package docker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/CamiloValderruten/faultline/internal/daemon"
)

const (
	alertsFileName  = "alerts.jsonl"
	alertOffsetFile = ".alert-offset"
	alertsEnvVar    = "FAULTLINE_ALERTS"
	alertsPathInCtr = "/work/alerts.jsonl"
	alertPollEvery  = time.Second
	maxAlertsPerMin = 4
)

func (s *Sandbox) startDaemonAlertWatch() {
	if s.alertInbox == nil {
		s.alertInbox = daemon.NewInbox(100)
	}
	if s.daemonWatchCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.daemonWatchCancel = cancel
	go s.watchDaemonAlerts(ctx)
}

// AlertInbox returns the daemon→agent alert queue, or nil if daemons are off.
func (s *Sandbox) AlertInbox() *daemon.Inbox {
	if s == nil {
		return nil
	}
	return s.alertInbox
}

func (s *Sandbox) watchDaemonAlerts(ctx context.Context) {
	ticker := time.NewTicker(alertPollEvery)
	defer ticker.Stop()
	// ponytail: per-daemon recent enqueue times for rate limit
	recent := map[string][]time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollDaemonAlerts(recent)
		}
	}
}

func (s *Sandbox) pollDaemonAlerts(recent map[string][]time.Time) {
	root := filepath.Join(s.dir, "daemons")
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	names := s.daemonNameIndex()
	now := time.Now()
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		id := ent.Name()
		path := filepath.Join(root, id, alertsFileName)
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		offset := readAlertOffset(filepath.Join(root, id, alertOffsetFile))
		if offset > st.Size() {
			offset = 0 // truncated
		}
		if offset == st.Size() {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		if _, err := f.Seek(offset, 0); err != nil {
			f.Close()
			continue
		}
		data, err := readAllLimited(f, 1<<20) // 1 MiB per poll
		f.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		consumed := int64(len(data))
		// If the last line is incomplete, keep it for next poll.
		if data[len(data)-1] != '\n' {
			if i := strings.LastIndexByte(string(data), '\n'); i >= 0 {
				data = data[:i+1]
				consumed = int64(len(data))
			} else {
				continue
			}
		}
		name := names[id]
		if name == "" {
			name = id
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			text := parseAlertLine(line)
			if text == "" {
				continue
			}
			if !allowAlert(recent, id, now) {
				if s.logger != nil {
					s.logger.Warn("daemon alert rate-limited", "daemon_id", id)
				}
				continue
			}
			s.alertInbox.Enqueue(daemon.Alert{
				DaemonID: id,
				Name:     name,
				Text:     text,
			})
		}
		writeAlertOffset(filepath.Join(root, id, alertOffsetFile), offset+consumed)
	}
}

func (s *Sandbox) daemonNameIndex() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	infos, err := s.ListDaemons(ctx)
	if err != nil {
		return nil
	}
	out := make(map[string]string, len(infos))
	for _, info := range infos {
		out[info.ID] = info.Name
	}
	return out
}

func parseAlertLine(line string) string {
	var ev struct {
		Message string `json:"message"`
		Text    string `json:"text"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return ""
	}
	if msg := strings.TrimSpace(ev.Message); msg != "" {
		return msg
	}
	if msg := strings.TrimSpace(ev.Text); msg != "" {
		return msg
	}
	if t := strings.TrimSpace(ev.Type); t != "" {
		return t
	}
	return ""
}

func allowAlert(recent map[string][]time.Time, id string, now time.Time) bool {
	cut := now.Add(-time.Minute)
	var filtered []time.Time
	for _, t := range recent[id] {
		if t.After(cut) {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) >= maxAlertsPerMin {
		recent[id] = filtered
		return false
	}
	recent[id] = append(filtered, now)
	return true
}

func readAlertOffset(path string) int64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func writeAlertOffset(path string, n int64) {
	_ = os.WriteFile(path, []byte(strconv.FormatInt(n, 10)+"\n"), 0o600)
}

func readAllLimited(f *os.File, limit int64) ([]byte, error) {
	buf := make([]byte, limit)
	n, err := f.Read(buf)
	if n > 0 {
		return buf[:n], nil
	}
	return nil, err
}
