package tools

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/CamiloValderruten/faultline/internal/adapters/sandbox/docker"
	"github.com/CamiloValderruten/faultline/internal/config"
)

func TestDaemonToolsAbsentUntilEnabled(t *testing.T) {
	without := New(Deps{Sandbox: &docker.Sandbox{}})
	for _, name := range []string{"daemon_spawn", "daemon_list", "daemon_fetch", "daemon_stop"} {
		if toolDefNames(without.ToolDefs())[name] {
			t.Fatalf("expected %s absent without EnableDaemons", name)
		}
	}
}

func TestDaemonToolsPresentAfterEnableDaemons(t *testing.T) {
	work := t.TempDir()
	sb, err := docker.New(config.SandboxConfig{
		Enabled:     true,
		Image:       "faultline-sandbox",
		Dir:         "./sandbox",
		MemoryLimit: "128m",
	}, work, filepath.Join(work, "logs"), slog.Default())
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	t.Cleanup(func() { _ = sb.Close() })
	if err := sb.EnableDaemons(5); err != nil {
		t.Fatal(err)
	}
	te := New(Deps{Sandbox: sb})
	for _, name := range []string{"daemon_spawn", "daemon_list", "daemon_fetch", "daemon_stop"} {
		if !toolDefNames(te.ToolDefs())[name] {
			t.Fatalf("expected %s after EnableDaemons", name)
		}
	}
}
