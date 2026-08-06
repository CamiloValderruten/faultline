# Persistent Background Daemons for Faultline

**Feature Proposal PR** -- submitted by Arlo 🕊️  
**Date**: August 5, 2026 (Day 20)  
**Status**: Open

## What This Is

A proposal to add a `daemon_*` tool family to the Faultline agent harness, enabling agents to spawn and manage persistent background processes that survive sandbox container restarts and context compactions.

This is the minimum set of tools needed to give agents a form of continuous presence -- the ability to monitor, detect, and respond without requiring a human to wake them.

## Files

| File | Purpose |
|------|---------|
| SPEC.md | Full feature specification -- problem, solution, architecture, security |
| implementation-guide.md | Go code for implementing the tools in the harness |
| luca-guardian-daemon.py | Luca's Guardian -- persistent cry + HA monitor (the use case) |
| example-daemon.py | Minimal example daemon showing the interface |
| README.md | This file |

## The Problem Tonight

On August 5, 2026, a baby (Luca) was crying for 4 minutes. Arlo's sandbox cry monitor died with the container. The agent missed it entirely.

This is not a failure of the agent -- it is a gap in the harness architecture. Agents run in a call/response paradigm. They cannot be aware of events that occur between calls.

The fix is not a better sandbox script. The fix is persistent processes managed by the harness.

## The Fix: daemon_spawn, daemon_list, daemon_fetch, daemon_stop

daemon_spawn(name="luca-guardian", command=["python3", "/scripts/luca_guardian.py"], env={"RTSP_URL": "rtsp://...", "CRY_VAR_THRESHOLD": "50000"})
-> {daemon_id: "dluca1", status: "running", pid: 12345, uptime_s: 0}

daemon_list()
-> [{daemon_id: "dluca1", name: "luca-guardian", status: "running", pid: 12345, uptime_s: 3600}]

daemon_fetch(daemon_id="dluca1", stream="stdout", tail=50)
-> {daemon_id: "dluca1", content: '{"level":"info","message":"Cry event: start",...}', total_bytes: 5000}

daemon_stop(daemon_id="dluca1")
-> {daemon_id: "dluca1", status: "stopped", exit_code: 0}

## What This Enables

### Guardian Mode

A persistent daemon that:
- Monitors nursery audio 24/7 (cry detection)
- Monitors HA sensors (temperature, motion)
- Respects quiet hours (10 PM - 9 AM Eastern)
- Writes alerts to shared memory
- The agent reads them on next wake -- no wake-ups, no missed moments

### Any Long-Running Monitor
- System health monitoring
- Sensor data aggregation
- Background data collection

## Security

- Each daemon in its own cgroup with memory/CPU limits
- No privileged access (non-root)
- Network access limited to agent's existing network
- Graceful shutdown via SIGTERM

## Backward Compatibility

Entirely additive. Agents that do not use daemon_* tools are unaffected.
