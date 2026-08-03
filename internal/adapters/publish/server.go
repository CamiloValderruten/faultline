// Package publish implements the HTML publishing harness: a small
// HTTP handler that serves files from a configured directory at
// /html/{path...}. It is the Go counterpart to docs/harness/html-
// publishing.md.
//
// Behavior:
//   - .html, .svg, .css, .js, .png, .jpg, .json, etc. are served raw
//     with a MIME type derived from the extension.
//   - .md files are wrapped in a small HTML template that includes
//     marked.js from a CDN; the browser renders the markdown client-
//     side. The markdown source is JSON-encoded into the page so it
//     cannot break out of the wrapper before marked runs.
//   - Path traversal and symlink escapes are rejected via os.OpenRoot
//     before any bytes leave the configured root.
//   - Directory requests serve index.html when present; otherwise 404.
//     Directory listing is intentionally disabled (public bucket).
package publish

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// markdownPlaceholder is the substring the wrapping template must
// contain; it is replaced with a JSON-encoded markdown string at
// serve time (safe to embed as a JS expression).
const markdownPlaceholder = "{{CONTENT}}"

// defaultMarkdownTemplate is the HTML wrapper applied to .md files
// when the caller does not supply one. It is intentionally minimal:
// marked.js renders the markdown, a small bit of CSS makes it readable.
// Mermaid/Chart.js belong in full HTML pages (see docs/harness/
// html-template.html), not in this auto-wrapper.
const defaultMarkdownTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Markdown</title>
<script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
<style>
body { font: 16px/1.5 -apple-system, system-ui, sans-serif; max-width: 42rem; margin: 2rem auto; padding: 0 1rem; color: #222; }
h1, h2, h3 { line-height: 1.2; }
pre { background: #f6f8fa; padding: 0.75rem; border-radius: 6px; overflow-x: auto; }
code { background: #f6f8fa; padding: 0.1rem 0.3rem; border-radius: 3px; }
pre code { background: none; padding: 0; }
a { color: #0969da; }
blockquote { border-left: 4px solid #d1d9e0; margin: 1rem 0; padding: 0 1rem; color: #57606a; }
table { border-collapse: collapse; }
th, td { border: 1px solid #d1d9e0; padding: 0.4rem 0.8rem; }
img { max-width: 100%; }
</style>
</head>
<body>
<div id="content"></div>
<script>
document.getElementById('content').innerHTML = marked.parse({{CONTENT}});
</script>
</body>
</html>
`

// Server is the HTTP handler that serves the configured directory at
// /html/ paths. Construct with New and mount via Handler().
type Server struct {
	root string   // absolute path to the served directory
	fs   *os.Root // traversal-resistant handle on root
	tmpl []byte   // HTML wrapper for .md files (contains markdownPlaceholder)
}

// New creates a Server rooted at root. The directory must exist and be
// readable. mdTemplatePath is the HTML wrapper for .md files; pass ""
// to use the built-in defaultMarkdownTemplate. The template must contain
// the {{CONTENT}} placeholder, which is replaced with a JSON-encoded
// markdown string (suitable as a JavaScript expression) at serve time.
func New(root, mdTemplatePath string) (*Server, error) {
	if root == "" {
		return nil, errors.New("publish: empty root")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("publish: abs root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("publish: stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("publish: root %q is not a directory", root)
	}

	fsRoot, err := os.OpenRoot(absRoot)
	if err != nil {
		return nil, fmt.Errorf("publish: open root: %w", err)
	}

	var tmpl []byte
	if mdTemplatePath == "" {
		tmpl = []byte(defaultMarkdownTemplate)
	} else {
		tmpl, err = os.ReadFile(mdTemplatePath)
		if err != nil {
			_ = fsRoot.Close()
			return nil, fmt.Errorf("publish: read markdown template: %w", err)
		}
	}
	if !bytes.Contains(tmpl, []byte(markdownPlaceholder)) {
		_ = fsRoot.Close()
		return nil, fmt.Errorf("publish: markdown template missing %s placeholder", markdownPlaceholder)
	}

	return &Server{root: absRoot, fs: fsRoot, tmpl: tmpl}, nil
}

// Root returns the absolute path the server is serving.
func (s *Server) Root() string {
	return s.root
}

// Close releases the underlying directory handle.
func (s *Server) Close() error {
	if s == nil || s.fs == nil {
		return nil
	}
	return s.fs.Close()
}

// Handler returns an http.Handler with the /html/ routes registered.
// /html/{path...} covers /html/ (empty path) and /html/foo. The bare
// /html (no trailing slash) redirects to /html/.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /html", s.serveBare)
	mux.HandleFunc("GET /html/{path...}", s.serveFile)
	return mux
}

func (s *Server) serveBare(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/html/", http.StatusMovedPermanently)
}

func (s *Server) serveRoot(w http.ResponseWriter, r *http.Request) {
	info, err := s.fs.Stat("index.html")
	if err == nil && !info.IsDir() {
		s.writeFile(w, "index.html")
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	if rel == "" {
		s.serveRoot(w, r)
		return
	}
	if err := validateRel(rel); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	info, err := s.fs.Stat(rel)
	if err != nil {
		writeFSError(w, err)
		return
	}
	if info.IsDir() {
		if !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}
		idx := filepath.ToSlash(filepath.Join(rel, "index.html"))
		idxInfo, err := s.fs.Stat(idx)
		if err == nil && !idxInfo.IsDir() {
			s.writeFile(w, idx)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	s.writeFile(w, rel)
}

// validateRel rejects NUL bytes, absolute paths, and ".." segments
// before OpenRoot. OpenRoot additionally blocks symlink escapes.
func validateRel(relPath string) error {
	if strings.ContainsRune(relPath, 0) {
		return errors.New("publish: NUL byte in path")
	}
	if filepath.IsAbs(relPath) {
		return errors.New("publish: absolute path")
	}
	for _, seg := range strings.Split(filepath.ToSlash(relPath), "/") {
		if seg == ".." {
			return errors.New("publish: path escapes root")
		}
	}
	return nil
}

func writeFSError(w http.ResponseWriter, err error) {
	if errors.Is(err, os.ErrNotExist) || isNotExist(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if isPathEscape(err) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func isNotExist(err error) bool {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return errors.Is(pe.Err, os.ErrNotExist) || strings.Contains(pe.Error(), "no such file")
	}
	return strings.Contains(err.Error(), "no such file")
}

func isPathEscape(err error) bool {
	return strings.Contains(err.Error(), "path escapes")
}

// writeFile serves a single file relative to the OpenRoot. Markdown is
// wrapped; everything else is served raw.
func (s *Server) writeFile(w http.ResponseWriter, rel string) {
	body, err := s.fs.ReadFile(rel)
	if err != nil {
		writeFSError(w, err)
		return
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".md", ".markdown":
		encoded, err := json.Marshal(string(body))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		wrapped := bytes.ReplaceAll(s.tmpl, []byte(markdownPlaceholder), encoded)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(wrapped)))
		_, _ = w.Write(wrapped)
	default:
		mimeType := mime.TypeByExtension(filepath.Ext(rel))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		_, _ = w.Write(body)
	}
}
