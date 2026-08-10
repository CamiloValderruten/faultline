package docker

import (
	"strings"
	"testing"
)

func TestDaemonRunArgsDetachedPersistent(t *testing.T) {
	s := &Sandbox{
		dir:         "/tmp/faultline/sandbox",
		image:       "faultline-sandbox",
		memoryLimit: "128m",
		uid:         1000,
		gid:         1000,
	}
	args := s.daemonRunArgs(
		"faultline-daemon-coco-abc123",
		"coco", "abc123", "price-watch", "Poll BTC prices",
		"2026-08-10T17:00:00Z",
		map[string]string{"SYMBOL": "BTC"},
		[]string{"python3", "/scripts/watch.py"},
	)
	joined := strings.Join(args, "\x00")
	for _, want := range []string{
		"run", "-d",
		"--restart\x00unless-stopped",
		"--name\x00faultline-daemon-coco-abc123",
		"--label\x00faultline.daemon=1",
		"--label\x00faultline.agent=coco",
		"--label\x00faultline.daemon.id=abc123",
		"--label\x00faultline.daemon.name=price-watch",
		"--label\x00faultline.daemon.description=Poll BTC prices",
		"-v\x00/tmp/faultline/sandbox/scripts:/scripts:ro",
		"-v\x00/tmp/faultline/sandbox/daemons/abc123:/work:rw",
		"--user\x001000:1000",
		"--security-opt\x00no-new-privileges",
		"--network=none",
		"-e\x00SYMBOL=BTC",
		"faultline-sandbox\x00python3\x00/scripts/watch.py",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("docker args missing %q in %v", want, args)
		}
	}
	if strings.Contains(joined, "--rm") {
		t.Fatalf("daemon containers must not use --rm: %v", args)
	}
}

func TestDaemonRunArgsInheritsNetwork(t *testing.T) {
	s := &Sandbox{
		dir:         "/tmp/faultline/sandbox",
		image:       "faultline-sandbox",
		memoryLimit: "128m",
		uid:         1000,
		gid:         1000,
		network:     true,
	}
	args := s.daemonRunArgs("n", "coco", "id", "n", "d", "t", nil, []string{"python3", "/scripts/x.py"})
	if strings.Contains(strings.Join(args, "\x00"), "--network=none") {
		t.Fatalf("expected sandbox network inherited: %v", args)
	}
}

func TestDaemonScriptFilename(t *testing.T) {
	got, err := daemonScriptFilename([]string{"python3", "/scripts/watch_prices.py"})
	if err != nil || got != "watch_prices.py" {
		t.Fatalf("got %q err %v", got, err)
	}
	for _, bad := range [][]string{
		nil,
		{"python3"},
		{"python3", "/scripts/../etc/passwd"},
		{"python3", "/scripts/sub/x.py"},
		{"python3", "/output/x.py"},
	} {
		if _, err := daemonScriptFilename(bad); err == nil {
			t.Fatalf("expected reject for %v", bad)
		}
	}
}
