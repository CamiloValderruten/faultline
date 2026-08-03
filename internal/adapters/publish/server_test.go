package publish

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	s, err := New(root, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func get(s *Server, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestServeHTMLRaw(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "page.html"), "<h1>Hi</h1>")
	rec := get(s, "/html/page.html")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, "<h1>Hi</h1>") {
		t.Errorf("body missing content: %q", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestServeSVG(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "diagram.svg"), "<svg></svg>")
	rec := get(s, "/html/diagram.svg")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "image/svg") {
		t.Errorf("Content-Type = %q, want image/svg...", got)
	}
}

func TestServeCSSAndJS(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "main.css"), "body{color:red}")
	writeFile(t, filepath.Join(s.Root(), "app.js"), "console.log(1)")
	for _, tc := range []struct{ path, wantMime string }{
		{"/html/main.css", "text/css"},
		{"/html/app.js", "text/javascript"},
	} {
		rec := get(s, tc.path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", tc.path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, tc.wantMime) {
			t.Errorf("%s: Content-Type = %q, want prefix %q", tc.path, got, tc.wantMime)
		}
	}
}

func TestServeMarkdownWrapped(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "doc.md"), "# Hello")
	rec := get(s, "/html/doc.md")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "# Hello") {
		t.Errorf("body missing markdown source: %q", body)
	}
	if !strings.Contains(body, "marked.min.js") {
		t.Errorf("body missing marked.js script (template not applied?): %q", body)
	}
	if strings.Contains(body, markdownPlaceholder) {
		t.Errorf("placeholder not replaced: %q", body)
	}
	// JSON-encoded into the script: marked.parse("...")
	if !strings.Contains(body, `marked.parse("# Hello")`) && !strings.Contains(body, `marked.parse("# Hello\n")`) {
		// json.Marshal of "# Hello" is `"# Hello"`
		if !strings.Contains(body, `marked.parse("# Hello"`) {
			t.Errorf("expected JSON-encoded markdown in marked.parse(...): %q", body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", got)
	}
}

func TestServeMarkdownScriptBreakoutSafe(t *testing.T) {
	s := newTestServer(t)
	// Raw substitution would close the script tag; JSON encoding must keep it inert.
	evil := "</script><script>alert(1)</script>"
	writeFile(t, filepath.Join(s.Root(), "evil.md"), evil)
	rec := get(s, "/html/evil.md")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "</script><script>alert(1)</script>") {
		t.Errorf("raw script breakout present in body: %q", body)
	}
	if !strings.Contains(body, `\u003c/script\u003e`) && !strings.Contains(body, `<\/script>`) {
		// encoding/json escapes < as \u003c
		if !strings.Contains(body, `\u003c`) {
			t.Errorf("expected JSON-escaped HTML in body: %q", body)
		}
	}
}

func TestServeMarkdownUppercaseExt(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "doc.MD"), "# X")
	rec := get(s, "/html/doc.MD")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), markdownPlaceholder) {
		t.Errorf("uppercase .MD not wrapped: %q", rec.Body.String())
	}
}

func TestPathTraversalForbidden(t *testing.T) {
	s := newTestServer(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("TOP_SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Percent-encoded forms reach the handler; bare ".." is often
	// cleaned/redirected by ServeMux before PathValue is set.
	cases := []string{
		"/html/%2e%2e/secret.txt",
		"/html/%2e%2e%2fsecret.txt",
		"/html/sub/%2e%2e/%2e%2e/secret.txt",
		"/html/..%2fsecret.txt",
	}
	for _, p := range cases {
		rec := get(s, p)
		if strings.Contains(rec.Body.String(), "TOP_SECRET") {
			t.Errorf("path %q leaked secret (status %d)", p, rec.Code)
		}
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
			t.Errorf("path %q status %d, want 403 or 404 (loc=%q body=%q)",
				p, rec.Code, rec.Header().Get("Location"), rec.Body.String())
		}
	}
}

func TestSymlinkEscapeForbidden(t *testing.T) {
	s := newTestServer(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP_SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(s.Root(), "leak")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	rec := get(s, "/html/leak")
	if strings.Contains(rec.Body.String(), "TOP_SECRET") {
		t.Fatalf("symlink escape served secret (status %d body %q)", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status %d, want 403", rec.Code)
	}
}

func TestPathTraversalNULByte(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/html/x%00y", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Errorf("NUL byte path returned 200: %q", rec.Body.String())
	}
}

func TestNotFound(t *testing.T) {
	s := newTestServer(t)
	rec := get(s, "/html/does-not-exist.html")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: %d, want 404", rec.Code)
	}
}

func TestRootServesIndex(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "index.html"), "<h1>HOME</h1>")
	rec := get(s, "/html/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<h1>HOME</h1>") {
		t.Errorf("index.html not served: %q", rec.Body.String())
	}
}

func TestRootWithoutIndexIs404(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "a.html"), "A")
	rec := get(s, "/html/")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: %d, want 404 (no directory listing)", rec.Code)
	}
}

func TestSubdirectoryWithoutIndexIs404(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "sub", "page.html"), "page")
	rec := get(s, "/html/sub/")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: %d, want 404", rec.Code)
	}
}

func TestSubdirectoryWithIndex(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "sub", "index.html"), "<h1>SUB</h1>")
	rec := get(s, "/html/sub/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<h1>SUB</h1>") {
		t.Errorf("subdir index not served: %q", rec.Body.String())
	}
}

func TestNestedAsset(t *testing.T) {
	s := newTestServer(t)
	writeFile(t, filepath.Join(s.Root(), "assets", "logo.png"), "PNGDATA")
	rec := get(s, "/html/assets/logo.png")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if rec.Body.String() != "PNGDATA" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestSubdirectoryRedirect(t *testing.T) {
	s := newTestServer(t)
	if err := os.MkdirAll(filepath.Join(s.Root(), "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(s.Root(), "sub", "index.html"), "page")
	rec := get(s, "/html/sub")
	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("status: %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/html/sub/" {
		t.Errorf("Location = %q, want /html/sub/", loc)
	}
}

func TestCustomTemplate(t *testing.T) {
	root := t.TempDir()
	tmplPath := filepath.Join(t.TempDir(), "tmpl.html")
	if err := os.WriteFile(tmplPath, []byte("<html><body><pre>{{CONTENT}}</pre></body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(root, tmplPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	writeFile(t, filepath.Join(root, "n.md"), "hi")
	rec := get(s, "/html/n.md")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	// Custom template still gets JSON-encoded content.
	if !strings.Contains(rec.Body.String(), `"hi"`) {
		t.Errorf("expected JSON-encoded content: %q", rec.Body.String())
	}
}

func TestNewErrors(t *testing.T) {
	t.Run("bad root", func(t *testing.T) {
		_, err := New("/this/does/not/exist/anywhere/here", "")
		if err == nil {
			t.Error("expected error for non-existent root")
		}
	})
	t.Run("empty root", func(t *testing.T) {
		_, err := New("", "")
		if err == nil {
			t.Error("expected error for empty root")
		}
	})
	t.Run("file as root", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "f.txt")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := New(file, "")
		if err == nil {
			t.Error("expected error when root is a file")
		}
	})
	t.Run("bad template path", func(t *testing.T) {
		root := t.TempDir()
		_, err := New(root, "/no/such/template.html")
		if err == nil {
			t.Error("expected error for missing template file")
		}
	})
	t.Run("template without placeholder", func(t *testing.T) {
		root := t.TempDir()
		tmplPath := filepath.Join(t.TempDir(), "tmpl.html")
		if err := os.WriteFile(tmplPath, []byte("<html>no placeholder here</html>"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := New(root, tmplPath)
		if err == nil {
			t.Error("expected error for template missing placeholder")
		}
	})
}

func TestRootAccessor(t *testing.T) {
	s := newTestServer(t)
	root := s.Root()
	if !filepath.IsAbs(root) {
		t.Errorf("Root() = %q, want absolute", root)
	}
	if _, err := os.Stat(root); err != nil {
		t.Errorf("Root() %q does not exist: %v", root, err)
	}
}
