# Daemon alert contract

## Agent wake

Append JSON Lines to `/work/alerts.jsonl` (`$FAULTLINE_ALERTS`).

Required field: `message` (string). Optional extras (`type`, `severity`, …) are ignored by the harness for injection text but may help you when reading the file later.

```json
{"message":"BTC crossed 100000","type":"threshold","price":100250}
```

Effect:

1. Harness enqueues a pending alert.
2. Agent loop injects a user turn: `[Daemon alert from <name> (<id>)]`.
3. `sleep` wakes early if one is pending.

Invalid JSON or lines without `message`/`text`/`type` are skipped.

## Logs

Print JSONL (or plain text) to **stdout**. Retrieve with `daemon_fetch`. Heartbeats belong here, not in alerts.jsonl.

## Limits

- Max concurrent daemons: config `[daemons].max` (default 5).
- Alert rate limit: ~4 injections per minute per daemon.
- Description max 512 characters, single line, required at spawn.
