# Faultline Agent Harness

This directory documents features that extend the agent's capabilities beyond the core LLM loop. These are conventions and templates the agent uses in its sandbox to produce shareable artifacts (web pages, dashboards, documents) without requiring a build step or external services.

## Conventions

- **[HTML Publishing](html-publishing.md)** — Write markdown / HTML / SVG files to the sandbox `outputs/html/` directory; they're served at `arlo.camilovalderruten.com/html/<filename>`. Markdown auto-renders in the browser via [marked.js](https://marked.js.org/), Mermaid diagrams via [Mermaid](https://mermaid.js.org/), and charts via [Chart.js](https://www.chartjs.org/). Inline CSS, no build step, no bundler.

## Layout

```
docs/harness/
├── README.md              ← this file
├── html-publishing.md     ← HTML publishing convention
└── html-template.html     ← starter scaffold (marked.js + Mermaid + Chart.js)
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