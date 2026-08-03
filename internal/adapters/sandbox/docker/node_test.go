package docker

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesNodePackageManifest(t *testing.T) {
	s := &Sandbox{
		dir:    t.TempDir(),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if err := s.init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s.dir, "node", "package.json"))
	if err != nil {
		t.Fatalf("read node/package.json: %v", err)
	}
	for _, want := range []string{`"name": "faultline-node-sandbox"`, `"private": true`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("package.json missing %q:\n%s", want, data)
		}
	}
}

func TestDockerArgsMountNodeEnvironment(t *testing.T) {
	s := &Sandbox{
		dir:         "/tmp/faultline/sandbox",
		image:       "faultline-sandbox",
		memoryLimit: "128m",
		uid:         1000,
		gid:         1000,
	}

	args := s.dockerArgs(false, "faultline-sandbox-test")
	joined := strings.Join(args, "\x00")
	for _, want := range []string{
		"-v\x00/tmp/faultline/sandbox/node:/node:rw",
		"-v\x00/tmp/faultline/sandbox/mcp:/mcp:rw",
		"-e\x00PATH=/node/node_modules/.bin:/usr/local/bin:/usr/bin:/bin",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("docker args missing %q in %v", want, args)
		}
	}
}

func TestDockerArgsInjectsSandboxEnv(t *testing.T) {
	s := &Sandbox{
		dir:         "/tmp/faultline/sandbox",
		image:       "faultline-sandbox",
		memoryLimit: "128m",
		uid:         1000,
		gid:         1000,
		env:         map[string]string{"GH_TOKEN": "ghp_secret", "Z_LAST": "1"},
	}

	args := s.dockerArgs(false, "faultline-sandbox-test")
	joined := strings.Join(args, "\x00")
	for _, want := range []string{
		"-e\x00GH_TOKEN=ghp_secret",
		"-e\x00Z_LAST=1",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("docker args missing %q in %v", want, args)
		}
	}
	// Sorted: GH_TOKEN before Z_LAST.
	if i, j := strings.Index(joined, "GH_TOKEN="), strings.Index(joined, "Z_LAST="); i < 0 || j < 0 || i > j {
		t.Fatalf("env flags not sorted: %v", args)
	}
}

func TestMCPStdioArgsInjectsSandboxEnvThenCallerEnv(t *testing.T) {
	s := &Sandbox{
		dir:         "/tmp/faultline/sandbox",
		image:       "faultline-sandbox",
		memoryLimit: "128m",
		uid:         1000,
		gid:         1000,
		env:         map[string]string{"GH_TOKEN": "from-sandbox"},
	}

	args := s.mcpStdioArgs("faultline-mcp-test", "/mcp/x", map[string]string{
		"GH_TOKEN": "from-server",
		"FOO":      "bar",
	}, "npx", nil)
	joined := strings.Join(args, "\x00")
	if !strings.Contains(joined, "-e\x00GH_TOKEN=from-sandbox") {
		t.Fatalf("missing sandbox GH_TOKEN in %v", args)
	}
	if !strings.Contains(joined, "-e\x00GH_TOKEN=from-server") {
		t.Fatalf("missing caller GH_TOKEN override in %v", args)
	}
	if !strings.Contains(joined, "-e\x00FOO=bar") {
		t.Fatalf("missing caller FOO in %v", args)
	}
	// Later -e wins in docker; caller override must appear after sandbox.
	if i, j := strings.Index(joined, "GH_TOKEN=from-sandbox"), strings.Index(joined, "GH_TOKEN=from-server"); i < 0 || j < 0 || i > j {
		t.Fatalf("caller env should follow sandbox env: %v", args)
	}
}

func TestRedactDockerArgsMasksEnvValues(t *testing.T) {
	in := []string{"run", "-e", "GH_TOKEN=ghp_secret", "-e", "HOME=/cache/home", "img"}
	out := redactDockerArgs(in)
	joined := strings.Join(out, "\x00")
	if strings.Contains(joined, "ghp_secret") {
		t.Fatalf("secret leaked in redacted args: %v", out)
	}
	if !strings.Contains(joined, "GH_TOKEN=***") || !strings.Contains(joined, "HOME=***") {
		t.Fatalf("expected masked -e values, got %v", out)
	}
	// Original unchanged.
	if in[2] != "GH_TOKEN=ghp_secret" {
		t.Fatalf("redact mutated input: %v", in)
	}
}

func TestMCPStdioArgsExposeNodeEnvironment(t *testing.T) {
	s := &Sandbox{
		dir:         "/tmp/faultline/sandbox",
		image:       "faultline-sandbox",
		memoryLimit: "128m",
		uid:         1000,
		gid:         1000,
	}

	args := s.mcpStdioArgs("faultline-mcp-test", "/mcp/playwright", nil, "playwright-mcp", nil)
	joined := strings.Join(args, "\x00")
	if !strings.Contains(joined, "-e\x00PATH=/node/node_modules/.bin:/usr/local/bin:/usr/bin:/bin") {
		t.Fatalf("mcp stdio args missing node PATH in %v", args)
	}
}
