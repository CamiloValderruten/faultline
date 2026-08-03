# HTML Publishing Harness

Write files to the sandbox `outputs/html/` directory and they're served at **`arlo.camilovalderruten.com/html/<filename>`** — shareable URLs, no build step, no bundler, no server config.

This formalizes a pattern Arlo already uses (monthly money reports, Airbnb CV, family dashboards) and gives it a clean convention so the same scaffold works for letters to Luca, growth charts, photo galleries, and interactive tools.

---

## Layout

```
outputs/html/
├── *.md        → auto-renders to HTML in the browser (marked.js)
├── *.html      → serves raw (use for full custom pages or interactive JS)
├── *.svg       → serves raw (diagrams, illustrations)
└── assets/     → static assets (images, fonts, JSON data)
```

The serving layer treats `outputs/html/<name>.{md,html,svg}` and `outputs/html/assets/*` as public read-only paths under `arlo.camilovalderruten.com/html/<name>` and `arlo.camilovalderruten.com/html/assets/*`. **Anything placed here is publicly accessible** — don't put secrets in published files.

---

## Quick start

### Option A — Markdown (most common, ~60% of uses)

1. Copy [`html-template.html`](html-template.html) to `outputs/html/letter-day-1.html`.
2. Replace the `<div id="content">` placeholder with your markdown wrapped in `<pre data-markdown>...</pre>`:

   ```html
   <div id="content">
     <pre data-markdown>
# Letter to Luca — Day 1

Today you arrived. 6 lbs 14 oz, 19 inches, screaming the song of your people.

You have your mother's eyes and my dramatic timing.

Love,
   Dad
     </pre>
   </div>
   ```

3. Visit `https://arlo.camilovalderruten.com/html/letter-day-1.html` — the marked.js script on the page auto-renders the markdown into proper headings, paragraphs, and styling.

### Option B — Raw HTML (interactive, ~30%)

For dashboards, charts, or interactive tools, skip the auto-render and write HTML directly. The template provides sane defaults but you can override everything.

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

### Option C — SVG / Mermaid diagrams (~10%)

Mermaid works in markdown blocks. Write a fenced `mermaid` code block and the page renders it as an SVG diagram:

    ```mermaid
    graph LR
      A[Luca Day 1] --> B[Day 7]
      B --> C[Day 30]
    ```

---

## Features

- **Markdown auto-renders** via [marked.js](https://marked.js.org/) (loaded from jsDelivr CDN)
- **Mermaid diagrams** out of the box — fenced ` ```mermaid ` blocks become SVG
- **Chart.js** ready for data visualizations (line, bar, doughnut, scatter)
- **Inline CSS** — no build step, no PostCSS, no bundler
- **CDN-loaded dependencies** — no `node_modules`, no package management
- **Path-safe** — `..` and absolute paths are rejected by the serving layer
- **Cache-friendly** — immutable filenames (`<name>-<hash>.html`) get far-future cache headers

---

## Use cases

| Use case | Format | Why it fits |
|---|---|---|
| **Letter to Luca** (long-form reflections) | Markdown | Discord is wrong shape; HTML is shareable with family forever |
| **Growth charts** (weight over time) | HTML + Chart.js | Interactive zoom + tooltips |
| **Family HR / sleep dashboards** | HTML + Chart.js | Live data from HealthRelay via Huckleberry sensors |
| **Monthly money reports** | HTML + inline CSS | Already established pattern (Camilo/Juliana, July 2026) |
| **Photo galleries** (milestones) | HTML + assets/ | Responsive image grids |
| **"Today" page** (weather + calendar + Luca + HA state) | HTML + JS | Single source of truth, refreshable |
| **Interactive tools** (Luca schedule calculator, due-date countdown) | HTML + JS | Client-side computation, no backend needed |

---

## Frequency split (Arlo's mental model)

In practice:

- **~60% Markdown** — letters, docs, journals, daily reflections
- **~30% HTML + JS** — charts, dashboards, interactive widgets
- **~10% SVG / Mermaid** — diagrams, illustrations

If a use case doesn't fit these three buckets, ask before reaching for something exotic.

---

## Security

- **Public by default.** Anything in `outputs/html/` is publicly accessible. Do not include:
  - API keys, OAuth tokens, PATs
  - Home Assistant entities you don't want exposed (e.g., exact bedroom camera URLs)
  - Personal info beyond what you'd put on a family blog
  - Internal prompts or agent state
- **No path traversal.** The serving layer rejects `..`, absolute paths, and symlinks pointing outside `outputs/html/`.
- **Browser sandbox.** HTML files run in the browser's normal sandbox context; CSP is set by the hosting reverse proxy (default `default-src 'self' 'unsafe-inline' cdn.jsdelivr.net`).
- **No write API.** `outputs/html/` is read-only via the URL; writes happen only through the agent's sandbox tools, which are scoped to the running agent's permissions.
- **Versioning is your responsibility.** Treat `outputs/html/` like a public S3 bucket — don't put draft content there without a clear filename.

---

## Naming conventions

- Lowercase, dash-separated: `letter-to-luca-day-1.html`, `family-dashboard-2026-08.html`
- Versioned snapshots get a date suffix: `money-report-2026-07.html`
- Don't use spaces, uppercase, or underscores (browser encoding pain)
- Keep names short — they appear in URLs

---

## See also

- [`html-template.html`](html-template.html) — the starter scaffold
- [marked.js docs](https://marked.js.org/) — markdown reference
- [Mermaid docs](https://mermaid.js.org/syntax/flowchart.html) — diagram syntax
- [Chart.js docs](https://www.chartjs.org/docs/latest/) — chart types and options

---

*Arlo 🕊️ — written Mon Aug 3, 2026, formalized from the harness features brainstorm.*