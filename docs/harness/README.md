# Faultline Agent Harness

This directory documents features that extend the agent's capabilities beyond the core LLM loop. These are conventions and templates the agent uses in its sandbox to produce shareable artifacts (web pages, dashboards, documents) without requiring a build step or external services.

## Conventions

- **[HTML Publishing](html-publishing.md)** — Write markdown / HTML / SVG files to the sandbox `output/html/` directory; they're served at `https://<agent-host>/html/<filename>` by the `[publish]` HTTP listener (typically fronted by a Cloudflare Tunnel). Markdown auto-renders via [marked.js](https://marked.js.org/). Full HTML pages can load Mermaid and Chart.js from the starter template. Inline CSS, no build step, no bundler.

## Layout

```
docs/harness/
├── README.md              ← this file
├── html-publishing.md     ← HTML publishing convention
└── html-template.html     ← starter scaffold for full HTML pages (marked.js + Mermaid + Chart.js)
```

## Adding a new harness feature

1. Pick a name that describes the *capability*, not the *implementation* (HTML Publishing, not "marked.js loader").
2. Add a `docs/harness/<feature>.md` that captures:
   - **Layout** — where files go, how they're organized
   - **Quick start** — minimal working example
   - **Use cases** — concrete examples
   - **Frequency** — expected usage distribution (helps prioritize polish)
   - **Security** — what's exposed, what to watch out for
3. If a starter template or scaffold helps, ship it as `<feature>-template.<ext>`.
4. Cross-link from this index.

*Arlo 🕊️ — created Aug 3, 2026*
