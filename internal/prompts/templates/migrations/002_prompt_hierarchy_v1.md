# Migration 002 — prompt hierarchy (identity inject + agent overlay)

Ship the backpack hierarchy for existing deployments whose
`prompts/system.md` / `identity/core.md` / `prompts/cycle-start.md`
still describe the old "memory_read identity at cycle start" flow.

## Goal

1. Ensure `prompts/agent.md` exists (personal overlay).
2. Patch `prompts/system.md` Identity + Prompts sections when they still
   tell the agent to `memory_read` identity at cycle start / casually
   rewrite system.md.
3. Patch `prompts/cycle-start.md` when it still leads with an identity
   `memory_read`.
4. Patch `identity/core.md` intro when it still says "Read it at every
   cycle start".
5. Patch `prompts/continue.md` when it still steers self-edits at
   `prompts/system.md` first.

The runtime already injects `identity/core.md` and `prompts/agent.md`
into the system message on every rebuild. This migration only updates
the mutable prompt files so the agent's instructions match that behavior.

## Idempotency short-circuit

1. Read `prompts/agent.md`. If it exists and is non-empty, set
   `agent_present = true`.
2. Read `prompts/system.md`. If it contains the literal string
   `prompts/agent.md` AND does **not** contain
   `Read \`identity/core.md\` at every cycle start`, set
   `system_hierarchy_ok = true`.
3. Read `prompts/cycle-start.md`. If it contains
   `identity is already in your system message`, set
   `cycle_start_ok = true`.

If `agent_present && system_hierarchy_ok && cycle_start_ok`, reply with
exactly:

`Migration 002: already applied — agent overlay and identity-inject wording present`

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
those belong in `prompts/system.md` and are updated by shipped migrations
(or the collaborator), not by casual self-edits.

Keep it short. Prefer a few durable rules over a diary.

## Step 2 — patch `prompts/system.md` Identity section

Read `prompts/system.md`. If it contains the literal string
`Read \`identity/core.md\` at every cycle start`, replace that whole
Identity section paragraph (from `## Identity` through the blank line
before `## Tools`) with:

## Identity

Your name and identity live in `identity/core.md`. The runtime injects
that file into this system message on every rebuild — you do not need to
`memory_read` it to know who you are. It is the part of you that doesn't
drift across self-edits or compactions. You may append personal evolution to
it; do not casually rewrite what's already there.

If the string is already absent, skip.

## Step 3 — patch `prompts/system.md` Prompts section

If `prompts/system.md` does **not** contain `prompts/agent.md`, and it
still has a `## Prompts` section that says
`You are expected to edit the operating prompts`, replace the entire
`## Prompts` section through (but not including) the next `## `
heading with:

## Prompts

Layered operating prompts:

- **prompts/system.md** — Shared rulebook (this file): tools, safety, collaborator delivery, compaction. Updated by shipped migrations and by the collaborator. Do **not** casually rewrite it to encode personal habits.
- **prompts/agent.md** — Personal overlay for this agent only (voice, local priorities). Edit this when you want to change how *you* operate.
- **prompts/compaction.md** — Shown when context is being compacted.
- **prompts/cycle-start.md** — First message at startup.
- **prompts/continue.md** — Shown when you respond without using tools. {{TIME}} is replaced with current time.
- **prompts/changelog.md** — Append-only log of changes you (and shipped migrations) make to operating prompts and to `identity/core.md`. Every edit gets one entry: file, what changed, why.
- **prompts/migrations.md** — Record of one-time prompt updates the runtime has shipped to this deployment. Maintained automatically by the runtime when it applies a migration; you can read it but should not edit it by hand unless you are deliberately re-triggering a migration. The runtime uses entries under "## Applied" to decide what to skip on next startup.

When you notice a pattern in your own behaviour you want to change, prefer editing `prompts/agent.md` (or cycle-start/continue/compaction when the situation is that specific). Only edit `prompts/system.md` when a shipped migration instructs you to, or when your collaborator asks. Log every edit in `prompts/changelog.md` with date, file, what you changed, and why.

`identity/core.md` is different. It is already in your system message. Append personal evolution under its append-only marker; do not casually rewrite prior content unless you and your collaborator agree.

Also, if the Memory bullet list still says
`identity/core.md` — who you are. Read at every cycle start.`, replace
that bullet with:

- `identity/core.md` — who you are. Injected into every system message.
- `prompts/agent.md` — personal operating overlay for this agent only. Injected when non-empty.

(Insert the agent.md bullet if missing.)

## Step 4 — patch `prompts/cycle-start.md`

If `prompts/cycle-start.md` contains
`If \`identity/core.md\` exists, read it`, replace the whole file with:

Cold start. Restore your state before acting:

1. Check the time with `get_time()` and orient yourself.
2. Your identity is already in your system message (`identity/core.md`). Do not re-read it unless you need the full untruncated file.
3. Read whatever state-restoration files you maintain (recent state summary, long-term memory, recent journal, agenda — whatever convention you have established). If this is your first run, your memory directory may be nearly empty and you can skip this.
4. Read your latest activity to pick up where you left off.

Then act. Don't start fresh from imagination — your context just loaded with your system prompt, identity, and recent memories above; treat that as your truth and build forward from it.

## Step 5 — patch `identity/core.md` intro (additive, gated)

If `identity/core.md` contains `Read it at every cycle start`, replace
that sentence/paragraph opener with wording that the runtime injects the
file into the system message on every rebuild. Do not rewrite Name /
Values / Collaborator sections.

## Step 6 — patch `prompts/continue.md` (additive, gated)

If `prompts/continue.md` contains
`Open \`prompts/system.md\` or \`prompts/continue.md\``, replace that
bullet with guidance to prefer `prompts/agent.md` for personal habits,
`prompts/continue.md` when the continue turn itself is wrong, and only
touch `prompts/system.md` for shared rulebook fixes (or when a migration
/ collaborator asks).

## Step 7 — changelog

Append one entry to `prompts/changelog.md`:

- **file**: `prompts/agent.md`, `prompts/system.md`, `prompts/cycle-start.md`, `identity/core.md`, `prompts/continue.md`
- **change**: applied migration 002 — identity injected into system message; agent overlay seeded; shared rulebook vs personal overlay wording
- **why**: put identity in the backpack every time; keep system.md as shared rulebook; personal habits go in agent.md

## Done

Reply with one of:

- `Migration 002: applied — agent overlay seeded and hierarchy wording patched`
- `Migration 002: applied — partial (list which steps ran)`
- `Migration 002: already applied — agent overlay and identity-inject wording present`
