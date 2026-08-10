# Subagent contract

## Primary → child

`prompt` must stand alone. Include:

- Goal and success criteria
- Constraints (what not to do)
- Desired report shape
- Any paths, URLs, or facts the child cannot discover alone

The child does not see the parent conversation, pending collaborator messages, or sibling subagent state.

## Child → primary

Only `subagent_report(summary=...)` is delivered. Prefer a structured summary
the parent can act on without re-reading child logs.

Async reports inject as:

```text
[Subagent report - <time>, work_id=<id>, profile=<name>]
[optional flags: truncated / canceled / error]

<summary>
```

`subagent_wait` drains that report so it will not also inject between turns.
`subagent_run` returns the report only as the tool result (never inbox).

## Waiting

| Goal | Use |
|------|-----|
| Wait for one spawn | `subagent_wait` |
| Nap for wall-clock / human | `sleep` (will **not** wake on subagent report) |
| Wait but collaborator may interrupt | `subagent_wait` (wakes early on collaborator pending; child keeps running) |

## Deny list (children)

No: `sleep`, `update_*`, nested `subagent_*`, `schedule_*`, `peer_*`,
`daemon_*`, `skill_install`, MCP list/discover/config/oauth/restart tools.

Yes (when enabled for the deployment): memory_*, sandbox_*, skills
activate/read/execute/work_read, web/wiki, send_message, tool MCP calls,
email_fetch, etc.

## Profiles

- `default` is always synthesized from `[api]` when `[subagent]` is on.
- Operator `[[subagent.profiles]]` must not use the name `default`.
- Route by `purpose` in the system-prompt catalog.
- Sampler fields: `0` / omitted inherits `[agent]`.
