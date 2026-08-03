package docker

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestFolderDirHTMLAlias(t *testing.T) {
	dir := t.TempDir()
	s := &Sandbox{dir: dir}

	got := s.folderDir("html")
	want := filepath.Join(dir, "output", "html")
	if got != want {
		t.Fatalf("folderDir(html) = %q, want %q", got, want)
	}
	if s.folderDir("output") != filepath.Join(dir, "output") {
		t.Fatalf("folderDir(output) should stay under sandbox root")
	}
}

func TestWriteFileHTMLCreatesPublishRoot(t *testing.T) {
	dir := t.TempDir()
	s := &Sandbox{dir: dir, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	if err := s.WriteFile("html", "page.html", "<html>ok</html>"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	path := filepath.Join(dir, "output", "html", "page.html")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read published file: %v", err)
	}
	if string(b) != "<html>ok</html>" {
		t.Fatalf("content = %q", b)
	}

	files, err := s.ListFiles("html")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 1 || files[0].Name != "page.html" {
		t.Fatalf("ListFiles(html) = %+v", files)
	}
}
