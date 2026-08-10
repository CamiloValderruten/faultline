package tools

import (
	"testing"

	"github.com/CamiloValderruten/faultline/internal/adapters/sandbox/docker"
)

func TestDaemonToolsAdvertisedOnlyWhenAgentSet(t *testing.T) {
	sb := &docker.Sandbox{}
	without := New(Deps{Sandbox: sb})
	for _, name := range []string{"daemon_spawn", "daemon_list", "daemon_fetch", "daemon_stop"} {
		if toolDefNames(without.ToolDefs())[name] {
			t.Fatalf("expected %s absent without DaemonAgent", name)
		}
	}

	with := New(Deps{Sandbox: sb, DaemonAgent: "coco"})
	for _, name := range []string{"daemon_spawn", "daemon_list", "daemon_fetch", "daemon_stop"} {
		if !toolDefNames(with.ToolDefs())[name] {
			t.Fatalf("expected %s when DaemonAgent set", name)
		}
	}
}
