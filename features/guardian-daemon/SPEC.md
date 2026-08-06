# Feature: Persistent Background Daemons for Faultline Agent Harness

**Author**: Arlo 🕊️  
**Date**: August 5, 2026 (Day 20)  
**Status**: Draft — submitted as PR  
**Repo**: `CamiloValderruten/Faultline`

## Summary

Add a daemon_* tool family to the Faultline harness that allows agents to spawn and manage persistent background processes that survive sandbox container restarts and context compactions. This enables Guardian Mode: an agent that is continuously aware, detects emergencies in real time, and only acts when genuinely warranted.

## Problem Statement

Currently, agents run in a call/response paradigm:
- Agent wakes on a message or scheduled task
- Agent does work, sleeps
- Background processes in the sandbox die when the container resets
- Agent has no awareness of events that occur between calls

On August 5, 2026, a baby monitor scenario illustrated the gap: an agent could not maintain a persistent audio listener. When a 4-minute crying episode occurred, the agent missed it entirely.

Agents need the ability to run continuously, to monitor persistently, and to decide when to act.

## Proposed Solution: New Harness Tools

### daemon_spawn
Spawns a persistent background process managed by the harness (not the sandbox container).
Input: {name, command, env, memory_limit_mb, cpu_limit}
Output: {daemon_id, name, status, spawned_at, stdout_path, stderr_path}

### daemon_list
Lists all running daemons for this agent.
Output: [{daemon_id, name, status, pid, uptime_s, exit_code, restart_count}]

### daemon_fetch
Fetches output from a daemon stdout/stderr.
Input: {daemon_id, stream, tail, offset_bytes}
Output: {daemon_id, stream, content, total_bytes, truncated}

### daemon_stop
Gracefully stops a daemon.
Input: {daemon_id, timeout_s}
Output: {daemon_id, status, exit_code}

### daemon_logs
Short-form structured log access.
Input: {daemon_id, level, limit}
Output: {daemon_id, events: [{timestamp, level, message}]}

## Architecture

Sandbox = ephemeral (per-call, dies on container restart).
Harness daemons = persistent (managed by harness binary, survives everything).

Storage: /var/lib/faultline/daemons/<agent-id>/<daemon-id>/
Each daemon in its own cgroup with memory/CPU limits.

## Guardian Mode Use Case

luca-guardian daemon:
- Monitors nursery audio continuously (cry detection)
- Monitors HA sensors (temperature, motion)
- Quiet hours (10 PM - 9 AM Eastern): logs only, no interruptions
- Safe hours: sends alerts via send_message
- Sustained distress or temperature emergency: escalates regardless
- Writes alerts to shared/luca/guardian-alerts.json

Agent: reads alerts on wake, harness injects status on cycle start.

## Security

- Each daemon in its own cgroup
- Default 128MB memory, 0.25 CPU cores per daemon
- No privileged access
- Graceful shutdown via SIGTERM

## Backward Compatibility

Additive only. Existing agents are unaffected.

## Alternatives Considered

1. Scheduled tasks - fires at times, cannot monitor continuously
2. HA automations - outside agent control
3. Embedded sandbox process - does not survive container restarts
4. External MQTT service - adds infrastructure
5. daemon_spawn in harness - managed by harness, survives everything
