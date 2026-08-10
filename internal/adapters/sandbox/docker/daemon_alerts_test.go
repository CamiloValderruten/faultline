package docker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CamiloValderruten/faultline/internal/daemon"
)

func TestParseAlertLine(t *testing.T) {
	if got := parseAlertLine(`{"message":"hi"}`); got != "hi" {
		t.Fatalf("got %q", got)
	}
	if got := parseAlertLine(`{"text":"x"}`); got != "x" {
		t.Fatalf("got %q", got)
	}
	if parseAlertLine(`not json`) != "" || parseAlertLine(`{"level":"info"}`) != "" {
		t.Fatal("expected empty for non-message lines")
	}
}

func TestPollDaemonAlertsEnqueues(t *testing.T) {
	dir := t.TempDir()
	s := &Sandbox{
		dir:        dir,
		daemonOwner: "owner",
		alertInbox: daemon.NewInbox(10),
		logger:     nil,
	}
	id := "abc123"
	work := filepath.Join(dir, "daemons", id)
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	alerts := filepath.Join(work, alertsFileName)
	if err := os.WriteFile(alerts, []byte(`{"message":"cry detected"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recent := map[string][]time.Time{}
	s.pollDaemonAlerts(recent)
	got := s.alertInbox.Pending()
	if len(got) != 1 || got[0].Text != "cry detected" || got[0].DaemonID != id {
		t.Fatalf("got %+v", got)
	}
	// Second poll should not re-enqueue (offset advanced).
	s.pollDaemonAlerts(recent)
	if s.alertInbox.HasPending() {
		t.Fatal("expected no replay")
	}
}
