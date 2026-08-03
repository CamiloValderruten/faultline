// Package publish implements the HTML publishing harness: a small
// HTTP handler that serves files from a configured directory at
// /html/{path...}. It is the Go counterpart to docs/harness/html-
// publishing.md and complements the HTML-to-markdown extractor in
// internal/tools/html.go (that direction is HTML→markdown, used by
// webfetch; this direction is the reverse — serving markdown as
// rendered HTML).
//
// Behaviour:
//   - .html, .svg, .css, .js, .png, .jpg, .json, etc. are served raw
//     with a MIME type derived from the extension.
//   - .md files are wrapped in a small HTML template that includes
//     marked.js from a CDN; the browser renders the markdown client-
//     side. This matches the multi-format harness convention without
//     pulling in a server-side markdown renderer dependency.
//   - Path traversal ("../", encoded variants, NUL bytes) is rejected
//     before any filesystem access.
//   - Directory requests (with or without trailing slash) render a
//     simple index, falling back to index.html when present.
package publish

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// markdownPlaceholder is the substring the wrapping template must
// contain; it is replaced with the .md source bytes at serve time.
const markdownPlaceholder = "{{CONTENT}}"

// defaultMarkdownTemplate is the HTML wrapper applied to .md files
// when the caller does not supply one. It is intentionally minimal:
// marked.js renders the markdown, a small bit of CSS makes it readable.
const defaultMarkdownTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{CONTENT}}</title>
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
<div id="content">{{CONTENT}}</div>
<script>
document.getElementById('content').innerHTML = marked.parse(document.getElementById('content').textContent);
</script>
</body>
</html>
`

// Server is the HTTP handler that serves the configured directory at
// /html/ paths. Construct with New and mount via Handler().
type Server struct {
	root string // absolute path to the served directory
	tmpl []byte // HTML wrapper for .md files (contains markdownPlaceholder)
}

// New creates a Server rooted at root. The directory must exist and be
// readable. mdTemplatePath is the HTML wrapper for .md files; pass ""
// to use the built-in defaultMarkdownTemplate. The template must contain
// the {{CONTENT}} placeholder, which is replaced with the raw markdown
// source at serve time.
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

	var tmpl []byte
	if mdTemplatePath == "" {
		tmpl = []byte(defaultMarkdownTemplate)
	} else {
		tmpl, err = os.ReadFile(mdTemplatePath)
		if err != nil {
			return nil, fmt.Errorf("publish: read markdown template: %w", err)
		}
	}
	if !bytes.Contains(tmpl, []byte(markdownPlaceholder)) {
		return nil, fmt.Errorf("publish: markdown template missing %s placeholder", markdownPlaceholder)
	}

	return &Server{root: absRoot, tmpl: tmpl}, nil
}

// Root returns the absolute path the server is serving. Useful for
// configuration introspection.
func (s *Server) Root() string {
	return s.root
}

// Handler returns an http.Handler with the /html/ routes registered.
// Two routes are needed because Go 1.22+ ServeMux treats /html/ and
// /html/{path...} as overlapping patterns when both are registered,
// so /html/{path...} alone covers /html/ (empty path = serveRoot) and
// /html/foo (serveFile). The bare /html (no trailing slash) is
// redirected to /html/ via a sibling route.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /html", s.serveBare)
	mux.HandleFunc("GET /html/{path...}", s.serveFile)
	return mux
}

// serveBare redirects /html to /html/ so relative links in the served
// pages resolve correctly.
func (s *Server) serveBare(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/html/", http.StatusMovedPermanently)
}

func (s *Server) serveRoot(w http.ResponseWriter, r *http.Request) {
	// Prefer index.html when present; otherwise emit a directory listing.
	if idx := filepath.Join(s.root, "index.html"); fileExists(idx) {
		s.writeFile(w, idx)
		return
	}
	s.writeListing(w, r, s.root, "")
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	if rel == "" {
		// Empty path = request for /html/ itself.
		s.serveRoot(w, r)
		return
	}
	full, err := s.resolvePath(rel)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		if !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}
		if idx := filepath.Join(full, "index.html"); fileExists(idx) {
			s.writeFile(w, idx)
			return
		}
		s.writeListing(w, r, full, rel)
		return
	}
	s.writeFile(w, full)
}

// resolvePath maps a URL path (relative to /html/) to an absolute
// filesystem path under s.root, rejecting any attempt to escape via
// "../", NUL bytes, or other tricks. The returned path is always
// either equal to s.root or starts with s.root + separator.
func (s *Server) resolvePath(relPath string) (string, error) {
	if strings.ContainsRune(relPath, 0) {
		return "", errors.New("publish: NUL byte in path")
	}
	// filepath.Clean with a leading "/" collapses any "../" segments
	// before they hit the filesystem.
	cleaned := filepath.Clean("/" + relPath)
	full := filepath.Join(s.root, cleaned)
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if abs != s.root && !strings.HasPrefix(abs, s.root+string(filepath.Separator)) {
		return "", errors.New("publish: path escapes root")
	}
	return abs, nil
}

// writeFile serves a single file. Markdown files are wrapped in the
// configured template; everything else is served raw with a MIME type
// derived from the extension.
func (s *Server) writeFile(w http.ResponseWriter, full string) {
	body, err := os.ReadFile(full)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	switch strings.ToLower(filepath.Ext(full)) {
	case ".md", ".markdown":
		wrapped := bytes.ReplaceAll(s.tmpl, []byte(markdownPlaceholder), body)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(wrapped)))
		_, _ = w.Write(wrapped)
	default:
		mimeType := mime.TypeByExtension(filepath.Ext(full))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		_, _ = w.Write(body)
	}
}

// writeListing emits a tiny HTML directory index for dir. relPath is
// the URL-relative path used to label the listing (empty for the root).
func (s *Server) writeListing(w http.ResponseWriter, r *http.Request, dir, relPath string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	heading := "/html/"
	if relPath != "" {
		heading = "/html/" + relPath
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<!doctype html><html><head><title>")
	fmt.Fprint(w, heading)
	fmt.Fprint(w, "</title></head><body><h1>")
	fmt.Fprint(w, heading)
	fmt.Fprint(w, "</h1><ul>")
	for _, e := range entries {
		name := e.Name()
		display := name
		if e.IsDir() {
			name += "/"
		}
		fmt.Fprintf(w, `<li><a href="%s">%s</a></li>`+"\n", name, display)
	}
	fmt.Fprint(w, "</ul></body></html>")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
