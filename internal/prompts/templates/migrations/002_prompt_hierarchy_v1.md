# Migration 002 — seed personal agent overlay (non-destructive)

Existing deployments need `prompts/agent.md` for the personal overlay
layer. The shared rulebook (`prompts/system.md`) is code-owned: the
running binary always uses the embedded template from GitHub — this
migration does **not** edit system.md, identity, cycle-start, continue,
or any other existing prompt file.

## Goal

1. If `prompts/agent.md` is missing or empty, seed the default overlay.
2. Leave every other file untouched.

## Idempotency short-circuit

1. Read `prompts/agent.md`.
2. If it exists and is non-empty, reply exactly:

`Migration 002: already applied — prompts/agent.md present`

and stop.

## Step 1 — seed `prompts/agent.md` if absent

If `prompts/agent.md` does not exist or is empty, write the following
verbatim with `memory_write` (do not wrap in fences):

# Agent overlay

Personal operating notes for **this agent only** — voice, local habits,
priorities, relationship quirks. The runtime injects this file into your
system message on every rebuild.

Edit this file when you want to change how *you* operate. Do **not** put
shared tool contracts, safety rules, or channel delivery rules here —
those belong in `prompts/system.md` (code-owned; shipped via GitHub) and
are not editable by the agent.

Keep it short. Prefer a few durable rules over a diary.

Do **not** edit `identity/core.md`, `prompts/system.md`,
`prompts/cycle-start.md`, `prompts/continue.md`, or any other existing
file as part of this migration.

## Step 2 — changelog (optional)

If you created `prompts/agent.md`, append one entry to
`prompts/changelog.md`:

- **file**: `prompts/agent.md`
- **change**: applied migration 002 — seeded personal agent overlay
- **why**: personal habits go in agent.md; system.md is code-owned

If `prompts/agent.md` already existed, skip the changelog write.

## Done

Reply with one of:

- `Migration 002: applied — seeded prompts/agent.md`
- `Migration 002: already applied — prompts/agent.md present`
