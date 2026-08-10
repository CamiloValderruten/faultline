---
name: daemons
description: >
  Run and manage long-lived Docker background daemons (daemon_spawn / list /
  fetch / stop) that survive Faultline and host restarts. Use when work must
  keep running while you sleep or compact — monitors, long jobs, watchers.
  Activate for daemon, background process, long-running monitor, watchdog,
  persistent script, or alerts.jsonl wakeups.
---

# Daemons

Long-lived processes managed by the harness as **Docker containers**, not host
shell jobs. They keep running across agent turns, Faultline restarts, and host
reboots (`--restart unless-stopped`).

Requires `[sandbox]` + `[daemons] enabled = true` in config. Ownership is an
auto UUID in `sandbox/.daemon-owner` — you never set an agent name.

## Tools

| Tool | Purpose |
|------|---------|
| `daemon_spawn` | Start a daemon (name, description, command, optional env) |
| `daemon_list` | Rediscover this agent's daemons via Docker labels |
| `daemon_fetch` | Tail stdout/stderr (`docker logs`) |
| `daemon_stop` | Clear restart policy, stop, and remove (frees a slot toward `max`) |

Hard caps: `description` required (single line); at most `[daemons].max` containers (default 5); command must reference a **flat** `/scripts/<file>` you wrote with `sandbox_write`.

## Workflow

1. Write the script: `sandbox_write` → `folder: "scripts"`, filename e.g. `watch_prices.py`.
2. Spawn:
   ```json
   {
     "name": "price-watch",
     "description": "Poll BTC; alert if above threshold",
     "command": ["python3", "/scripts/watch_prices.py"],
     "env": {"THRESHOLD": "100000"}
   }
   ```
3. Do other work or `sleep`. Routine progress: `daemon_fetch`. Status: `daemon_list`.
4. Stop when done: `daemon_stop` with `daemon_id`.

Activate this skill (`skill_activate` name `daemons`) when you need the full contract; the tools stay available when daemons are enabled.

## How to wake the agent (alerts)

**Stdout** = logs only (`daemon_fetch`).

**`/work/alerts.jsonl`** (also `$FAULTLINE_ALERTS`) = push notifications.

Append one JSON object per line with a `message` field:

```python
import json, os
ALERTS = os.environ["FAULTLINE_ALERTS"]  # /work/alerts.jsonl

def notify(message, **meta):
    line = json.dumps({"message": message, **meta}) + "\n"
    with open(ALERTS, "a") as f:
        f.write(line)
        f.flush()
```

The harness polls that file, injects `[Daemon alert from …]` into the
conversation (like Telegram/peers), and **interrupts sleep**. Rate-limited
(~4/min/daemon). Offsets are remembered so restarts do not replay old lines.

Do **not** write heartbeats to alerts.jsonl — use stdout for those.

## Mounts and env

Inside the container you get the usual sandbox mounts plus:

| Path | Use |
|------|-----|
| `/scripts` (ro) | Your scripts |
| `/work` (rw) | Per-daemon scratch + `alerts.jsonl` |
| `/output`, `/input`, `/venv`, … | Same as sandbox_execute |

Env always includes `FAULTLINE_ALERTS=/work/alerts.jsonl`. Network/memory follow `[sandbox]`.

## After restarts

`daemon_list` queries Docker by `faultline.owner=<uuid>`. Descriptions live on
container labels, so you can rediscover what each daemon does even after
compaction. Intentional `daemon_stop` removes the container so reboot will not
bring it back.

## See also

Bundled detail: `skill_read` → `references/contract.md`.
