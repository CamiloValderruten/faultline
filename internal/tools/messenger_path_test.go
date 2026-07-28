package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSandboxFilePath(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("output/chart.png", "png")

	got, err := resolveSandboxFilePath(root, "/output/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "output", "chart.png")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	if _, err := resolveSandboxFilePath(root, "/etc/passwd"); err == nil {
		t.Fatal("expected reject for /etc/passwd")
	}
	if _, err := resolveSandboxFilePath(root, "/output/../input/../etc/passwd"); err == nil {
		t.Fatal("expected traversal reject")
	}
	if _, err := resolveSandboxFilePath(root, "/output/missing.png"); err == nil {
		t.Fatal("expected missing file")
	}
}
