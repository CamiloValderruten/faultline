---
name: subagents
description: >
  Delegate isolated work to child agent loops (subagent_run / spawn / wait /
  status / cancel). Use when a task needs a fresh context window, a different
  model profile, or parallel investigation without burning the primary's turn.
  Activate for subagent, delegate, spawn worker, parallel research, profile
  routing, or subagent_report.
---

# Subagents

A subagent is a **fresh agent loop** with its own context window and a
profile-selected LLM. The primary supplies all context in the `prompt`; the
child never sees the parent conversation. Memory, search indexes, sandbox, and
skills are **shared**; the child's chat log is **not** persisted — only
`subagent_report` reaches you.

Requires `[subagent] enabled = true`. Profiles appear under **Available
Subagent Profiles** in the system prompt (including synthesized `default`
from `[api]`).

## Tools

| Tool | Who | Purpose |
|------|-----|---------|
| `subagent_run` | primary | Sync: block until done; report returns as the tool result |
| `subagent_spawn` | primary | Async: returns `work_id` immediately; report arrives later via inbox |
| `subagent_wait` | primary | Block on a prior spawn's `work_id`; drains report inline |
| `subagent_status` | primary | Active children: work_id, profile, elapsed, prompt preview |
| `subagent_cancel` | primary | Cooperative cancel; a cancellation report still lands shortly |
| `subagent_report` | **child only** | Deliver final `summary` to parent and exit |

## When to use which

| Situation | Pattern |
|-----------|---------|
| Need the answer before continuing | `subagent_run` |
| Parallel work while you do other tools | `subagent_spawn` → later inbox inject |
| Spawned, did other work, now need the answer | `subagent_wait` |
| Check progress / abort | `subagent_status` / `subagent_cancel` |

**Do not `sleep` waiting for a child.** Sleep does **not** wake on subagent
reports (only collaborator / peer / daemon alerts). Use `subagent_wait`.

## Workflow

1. Pick a profile from the catalog (`default`, or operator profiles by `purpose`).
2. Write a **self-contained** prompt: goal, constraints, output format, success criteria. The child has zero parent context.
3. Run or spawn:
   ```json
   {
     "profile": "default",
     "prompt": "Research X. Return: bullet findings, sources, open questions. Call subagent_report when done."
   }
   ```
4. Consume the report (tool result, wait result, or injected `[Subagent report …]` turn).
5. If you see `[truncated]` / `[canceled]` / empty content, decide whether to retry with a tighter prompt.

Activate this skill (`skill_activate` name `subagents`) when you need the full contract; the tools stay available when subagents are enabled.

## Child contract

The child must finish with `subagent_report` and put **everything** the parent
needs in `summary` (markdown OK). Text-only exits or hitting the turn/timeout
cap often yield a truncated/empty report.

Children keep memory_*, sandbox_*, skill_activate/read/execute/work_read,
web_fetch, send_message, and ordinary MCP tools (when configured). They
**cannot**: nest subagents, sleep, schedule_*, peer_*, daemon_*, update_*,
skill_install, or MCP config/oauth management tools. They have no operator
inbox.

## Caps (operator config)

| Key | Default | Notes |
|-----|---------|--------|
| `max_concurrent` | 4 | Async spawns only |
| `max_turns_per_run` | 50 | Child loop iterations |
| `max_inbox` | 32 | Queued async reports; oldest dropped when full |
| `run_timeout` | `30m` | Wall clock per run |

## Shared-state warning

Children share memory and sandbox with you and each other. Two parallel
children writing the same paths will race — assign distinct paths in the prompt.

## See also

Bundled detail: `skill_read` → `references/contract.md`.
