# HTML publishing harness (skill reference)

Canonical human docs live in the Faultline repo:

- `docs/harness/html-publishing.md` — layout, formats, security, CF tunnel notes
- `docs/harness/html-template.html` — full-page starter (also bundled here as `assets/template.html`)
- `docs/harness/README.md` — harness index

## Paths

| Layer | Path |
|-------|------|
| Prefer | `sandbox_write` with `folder: "html"`, flat `filename` |
| Sandbox path | `/output/html/<file>` |
| Host (typical) | `<sandbox.dir>/output/html/<file>` |
| HTTP | `GET /html/<file>` on the `[publish]` listener |
| Public | `{public_base_url}/html/<file>` |

Do **not** use `folder: "output"` for publishable pages — that writes `/output/<file>`, which is **not** served.

Nested `assets/` still need `sandbox_shell` (flat `sandbox_*` tools only).

## Formats

- `*.md` / `*.markdown` — server wraps with marked.js (JSON-safe embed)
- `*.html` — served raw (use `assets/template.html` for Mermaid + Chart.js)
- `*.svg`, `assets/*` — static

No directory listing: `/html/` is 404 unless `index.html` exists.

## Discord delivery

Link button via `send_message` / `send_rich_message`:

`{"text":"Open","style":"link","url":"https://<host>/html/<file>"}`
