---
name: html-artifact
description: >
  Create shareable visual deliverables (Artifact/Canvas-style) via the HTML
  publishing harness — dashboards, charts, designed letters, one-off tools —
  then ALWAYS deliver with a Discord link button. Never use send_file / file
  attach for canvases. Use when Discord text cannot carry the answer, or the
  user wants something to look at / share. Activate for artifact, canvas,
  visual page, dashboard, chart page, publish HTML, or "make a page/site
  for this".
---

# HTML Artifact

Ship a **visual deliverable** the collaborator opens in a browser — like Claude Artifacts or Cursor Canvas — backed by Faultline's HTML publishing harness.

You write files into the sandbox publish root; `[publish]` serves them; you **always** hand the user a Discord **link button**. Publishing without a button is incomplete.

**Hard rule — delivery is button-only.** Never call `send_file` (or any file attach) for a canvas/page. Not as a fallback, not "also", not when the link 404s. Fix the publish path and send the link button.

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
| Write (sandbox_write) | `folder: "html"` → `/output/html/…` | same |
| URL | `https://<origin>/html/<filename>` | same |

Prefer `public_base_url` from config / identity if known; otherwise use the table for **this** agent — never the other agent's host.

Harness docs (human): repo `docs/harness/html-publishing.md`. Bundled pointer: `references/harness.md`. Starter HTML: `assets/template.html` (via `skill_read`).

## Format choice

| Need | Format | How |
|------|--------|-----|
| Prose, letters, simple structure (~60%) | `.md` | `sandbox_write` with `folder: "html"`, filename `<slug>.md` — server wraps with marked.js |
| Charts, custom layout, interactivity (~30%) | `.html` | Start from `assets/template.html`; Chart.js + Mermaid already loaded; write with `folder: "html"` |
| Diagram-only (~10%) | Mermaid in md/html, or `.svg` | Fenced `mermaid` in template markdown, or raw SVG |

Naming: lowercase, dash-separated, short. Examples: `luca-weight-2026-08.html`, `money-report-july.md`.

## Workflow

1. **Decide slug + format** (artifact vs living page).
2. **Write the file** with `sandbox_write`:
   - `folder`: **`"html"`** (not `"output"` — `output` is not published)
   - `filename`: `<slug>.{md,html}` (flat; no path separators)
   - Nested assets under `/output/html/assets/` need `sandbox_shell` (mkdir + write); flat pages do not.
3. **Build the public URL:** `{public_base_url}/html/<slug>.{md,html}`.
4. **Always deliver with a Discord link button** — required every time a canvas/page is created or updated. Do not paste a bare URL as the only delivery, or stop after `sandbox_write`. Do not paste the whole page into Discord.

### Discord link button (mandatory)

Every successful publish **must** call `send_message` or `send_rich_message` with a link button pointing at the public URL. **Never** `send_file` / file attach for this skill.

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

- Use **this** agent's host in `url` (see Setup table).
- `url` is required for link buttons; `data` is not used (no callback).
- Button label can be contextual ("🌸 Abrir devocional", "Open dashboard") — still a `style: "link"` button.
- Keep the message body to one short line + optional context.
- `send_rich_message` works the same for `buttons` if you want an embed title/fields.
- Updating a living page: send a fresh link button again (same URL is fine).
- If the URL 404s: fix the write path (`folder: "html"`), republish, send the button again — still no file attach.

**Only if Discord/messaging itself is unavailable:** include the raw URL in text. Still never file-attach the HTML.

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

- [ ] Wrote with `sandbox_write` `folder: "html"` (not `"output"`)
- [ ] URL uses **this** agent's host
- [ ] Sent Discord **link button** with that URL (required)
- [ ] Did **not** call `send_file` / file attach (never for canvases)
- [ ] No keys/tokens/camera URLs/prompts in the page
