# HTML Publishing Harness

Write files to the sandbox **`output/html/`** directory (host path `<sandbox.dir>/output/html`). With `[publish]` enabled, Faultline serves them at **`/html/<path>`** on a loopback bind. A Cloudflare Tunnel (or any reverse proxy) maps each agent's public hostname to that bind:

| Agent | Example public URL | Typical local bind |
|-------|--------------------|--------------------|
| Arlo  | `https://arlo.camilovalderruten.com/html/…` | `127.0.0.1:8744` |
| Coco  | `https://coco.camilovalderruten.com/html/…` | `127.0.0.1:8745` |

No build step, no bundler, no write API on the URL — writes happen only through sandbox tools.

This formalizes a pattern already used for money reports, family dashboards, and similar shareable pages.

---

## Layout

Inside the sandbox container the path is `/output/html/…` (the sandbox mounts host `output/` at `/output`):

```
output/html/          ← publish root (created at startup when [publish] is on)
├── *.md              → wrapped with marked.js (client-side render)
├── *.html            → served raw (full custom pages / interactive JS)
├── *.svg             → served raw
└── assets/           → static assets (images, fonts, JSON data)
```

Public URL: `https://<agent-host>/html/<relative-path>`.

**Anything placed here is publicly accessible.** Do not put secrets in published files.

---

## Config

```toml
[publish]
enabled = true
bind = "127.0.0.1:8744"   # unique per agent on a shared host
# root = ""               # default: <sandbox.dir>/output/html
public_base_url = "https://arlo.camilovalderruten.com"  # log-only
```

Point the Cloudflare Tunnel ingress for that hostname at the same `bind`. Keep `[publish]` off the authenticated `[admin]` mux — public pages stay on their own origin/port.

---

## Quick start

### Option A — Markdown file (most common, ~60%)

1. From a sandbox script (or `sandbox_shell`), write markdown to `/output/html/letter-day-1.md`.
2. Visit `https://<agent-host>/html/letter-day-1.md` — the publish server wraps it in a minimal HTML page that loads marked.js and renders the source client-side.

No need to copy the HTML template for plain `.md` files.

### Option B — Full HTML page (interactive, ~30%)

For dashboards, Chart.js, or Mermaid-heavy pages, copy [`html-template.html`](html-template.html) to `/output/html/letter-day-1.html` and replace the content block. The template loads marked.js, Mermaid, and Chart.js from jsDelivr.

```html
<div id="content">
  <canvas id="weightChart"></canvas>
  <script>
    new Chart(document.getElementById('weightChart'), {
      type: 'line',
      data: { /* ... */ },
    });
  </script>
</div>
```

### Option C — Mermaid in the HTML template (~10%)

Inside a page based on `html-template.html`, use a fenced `mermaid` block inside `<pre data-markdown>` (or a raw `<div class="mermaid">`). The template's scripts render those as SVG.

---

## Two templates, deliberately

| Artifact | Role |
|----------|------|
| Built-in `.md` wrapper (in Go) | Auto-applied to `*.md` / `*.markdown`. marked.js only. Source is JSON-encoded into the page so it cannot break out of the wrapper. |
| [`html-template.html`](html-template.html) | Starter for **hand-authored `.html` pages** that want Mermaid + Chart.js + shared CSS. Not used as the `.md` wrapper (`{{TITLE}}` ≠ `{{CONTENT}}`). |

---

## Features

- **Markdown auto-renders** via marked.js (CDN) for `.md` files
- **Raw HTML / SVG / assets** served with extension-derived MIME types
- **No directory listing** — `/html/` and subdirs serve `index.html` when present, otherwise 404
- **Path-safe** — `..`, absolute paths, and symlink escapes outside the publish root are rejected (`os.OpenRoot`)
- **Inline CSS / CDN deps** — no build step

Cache headers and CSP belong on the Cloudflare / reverse-proxy side, not in this listener.

---

## Use cases

| Use case | Format | Why it fits |
|---|---|---|
| **Letter to Luca** (long-form reflections) | Markdown | Discord is wrong shape; HTML is shareable with family forever |
| **Growth charts** (weight over time) | HTML + Chart.js | Interactive zoom + tooltips |
| **Family HR / sleep dashboards** | HTML + Chart.js | Live-ish data glued in at write time |
| **Monthly money reports** | HTML + inline CSS | Established pattern |
| **Photo galleries** (milestones) | HTML + assets/ | Responsive image grids |
| **"Today" page** | HTML + JS | Single source of truth, refreshable |
| **Interactive tools** | HTML + JS | Client-side computation, no backend needed |

---

## Frequency split

- **~60% Markdown** — letters, docs, journals, daily reflections
- **~30% HTML + JS** — charts, dashboards, interactive widgets
- **~10% SVG / Mermaid** — diagrams, illustrations

---

## Security

- **Public by default.** Treat `output/html/` like a public S3 bucket. Do not include API keys, tokens, private camera URLs, internal prompts, or agent state.
- **First-party trust model.** Published pages are agent-authored content on the agent's public hostname. Raw HTML and JS run with the page's origin. This is not a sandbox for untrusted third-party HTML.
- **No path traversal / symlink escape.** The Go listener roots at the publish directory via `os.OpenRoot`.
- **No write API.** HTTP is read-only; writes go through sandbox tools.
- **Separate from admin.** Do not mount `/html/` on the authenticated admin server.
- **CSP / TLS.** Terminate TLS and set CSP at Cloudflare Tunnel / the reverse proxy (`cdn.jsdelivr.net` must be allowed if you use the CDN templates).

---

## Naming conventions

- Lowercase, dash-separated: `letter-to-luca-day-1.html`, `family-dashboard-2026-08.html`
- Versioned snapshots get a date suffix: `money-report-2026-07.html`
- Avoid spaces, uppercase, and underscores (URL encoding pain)
- Keep names short — they appear in URLs

---

## See also

- [`html-template.html`](html-template.html) — starter scaffold for full HTML pages
- `[publish]` in `config.example.toml`
- [marked.js docs](https://marked.js.org/)
- [Mermaid docs](https://mermaid.js.org/syntax/flowchart.html)
- [Chart.js docs](https://www.chartjs.org/docs/latest/)

---

*Arlo 🕊️ — written Mon Aug 3, 2026; wiring + multi-agent tunnel notes added with the publish adapter.*
