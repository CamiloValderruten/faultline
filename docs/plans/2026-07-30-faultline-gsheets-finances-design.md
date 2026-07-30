# Faultline Google Sheets finances (read-only)

> Design approved 2026-07-30. Approach: config + per-agent skill; no Go changes.

## Goal

Arlo and Coco answer personal-finance questions from the shared Tiller **Financials** spreadsheet via MCP, read-only.

## Architecture

- Each agent process has its own `mcp.json` entry for stdio `mcp-gsheets` (v1.10.0).
- Per-agent service account (Viewer on the sheet):
  - Arlo: `arlo-faultline@tiller-orders.iam.gserviceaccount.com`
  - Coco: `coco-faultline@tiller-orders.iam.gserviceaccount.com`
- Credentials live under `<agent>/sandbox/mcp/gsheets/google-sheets-sa.json` (visible as `/mcp/gsheets/...` inside the stdio sandbox).
- `GSHEETS_TOOLSETS=core` + `GSHEETS_READ_ONLY=true` → six read tools.
- Faultline `allow_tools` lists those six exact MCP tool names (Tier 2; unlock via `search_available_tools`).
- Identical `finances` skill copied into each agent’s `skills/finances/` (name + description in system prompt; body via `skill_activate`).

## Out of scope

- Writes, categorization, orders-mcp
- Native `finance_*` Go tools
- Shared skills directory

## Deploy notes

- Host: pi5 (`/home/camilovalderruten/.faultline/agents/{arlo,coco}`)
- Requires `[skills] enabled = true`, existing `[mcp]` / `[sandbox].network = true`
- Restart agents after `mcp.json` / skills / config changes
