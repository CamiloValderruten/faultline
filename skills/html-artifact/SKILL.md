---
name: html-artifact
description: >
  Create shareable visual deliverables (Artifact/Canvas-style) via the HTML
  publishing harness — dashboards, charts, designed letters, one-off tools —
  then send a Discord link button to open the page. Use when Discord text
  cannot carry the answer, or the user wants something to look at / share.
  Activate for artifact, canvas, visual page, dashboard, chart page, publish
  HTML, or "make a page/site for this".
---

# HTML Artifact

Ship a **visual deliverable** the collaborator opens in a browser — like Claude Artifacts or Cursor Canvas — backed by Faultline's HTML publishing harness.

You write files into the sandbox publish root; `[publish]` serves them; you hand the user a Discord **link button**.

## When to use

Prefer this skill when:

- The answer needs **layout, charts, diagrams, or design** (not a short Discord paragraph)
- The user wants something **shareable via URL** (family, later reference, phone browser)
- They ask for an artifact / canvas / page / dashboard / visual / "make a site"

Stay in Discord (no page) when a few sentences or a small table is enough.

## Modes

1. **Artifact (default)** — one-shot page for this request. Slug + optional date. Disposable unless they ask to keep it.
2. **Living page (secondary)** — stable filename (`family-dashboard.html`), overwrite deliberately when updating.

## Setup (already deployed)

| | Arlo | Coco |
|---|---|---|
| Public origin | `https://arlo.camilovalderruten.com` | `https://coco.camilovalderruten.com` |
| Write path (sandbox) | `/output/html/…` | `/output/html/…` |
| URL | `https://<origin>/html/<relative-path>` | same |

Prefer `public_base_url` from config / identity if known; otherwise use the table for **this** agent — never the other agent's host.

Harness docs (human): repo `docs/harness/html-publishing.md`. Bundled pointer: `references/harness.md`. Starter HTML: `assets/template.html` (via `skill_read`).

## Format choice

| Need | Format | How |
|------|--------|-----|
| Prose, letters, simple structure (~60%) | `.md` | Write markdown to `/output/html/<slug>.md` — server wraps with marked.js |
| Charts, custom layout, interactivity (~30%) | `.html` | Start from `assets/template.html`; Chart.js + Mermaid already loaded |
| Diagram-only (~10%) | Mermaid in md/html, or `.svg` | Fenced `mermaid` in template markdown, or raw SVG |

Naming: lowercase, dash-separated, short. Examples: `luca-weight-2026-08.html`, `money-report-july.md`.

## Workflow

1. **Decide slug + format** (artifact vs living page).
2. **Write the file** with sandbox tools to `/output/html/<slug>.{md,html}` (and `/output/html/assets/…` if needed).
3. **Build the public URL:** `{public_base_url}/html/<slug>.{md,html}`.
4. **Deliver to the collaborator** with a link button (do not paste the whole page into Discord):

### Discord link button

`send_message` or `send_rich_message`:

```json
{
  "text": "Your page is ready.",
  "buttons": [[
    {
      "text": "Open",
      "style": "link",
      "url": "https://arlo.camilovalderruten.com/html/example.md"
    }
  ]]
}
```

- `url` is required for link buttons; `data` is not used (no callback).
- Keep the message body to one short line + optional context.
- `send_rich_message` works the same for `buttons` if you want an embed title/fields.

If Discord/messaging is unavailable, still publish the file and tell the user the raw URL in text.

## Design bar (Artifact quality)

- One job per page; one clear title
- Readable on a phone (template defaults are fine)
- Prefer the template's CSS variables over inventing a new theme every time
- Charts: Chart.js on a `<canvas>`; diagrams: Mermaid fences inside markdown-in-template or `.md` where supported
- No wall of unexplained numbers — label axes / units

## Access / sensitivity

**Today:** anything under `/output/html/` is reachable on the public hostname (unlisted only by obscure filenames). Treat it like a public bucket for threat modeling.

**Soon:** tokenized URLs (`?token=…`) are planned so family pages can be unlisted/private-ish. Until that ships:

- Fine to publish personal-but-non-catastrophic family content the user asked for (letters, growth charts, money summaries **they requested**)
- Still avoid raw API keys, OAuth tokens, PATs, private camera stream URLs, and full agent prompts/state
- Prefer summaries over dumping secrets "just in case"

When token gating lands, append the token to the link-button URL and keep the same workflow.

## Quick checklist

- [ ] Wrote under `/output/html/`
- [ ] URL uses **this** agent's host
- [ ] Sent Discord link button (or plain URL fallback)
- [ ] No keys/tokens/camera URLs/prompts in the page
