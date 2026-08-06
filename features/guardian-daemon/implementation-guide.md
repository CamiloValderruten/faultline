# Implementation Guide: Persistent Background Daemons
*How to implement daemon_* tools in the Faultline harness*

## Overview

The daemon system consists of two parts:
1. Daemon Supervisor -- a process manager running inside the harness binary
2. Agent Tools -- daemon_spawn, daemon_list, daemon_fetch, daemon_stop, daemon_logs

The harness binary is a long-running process. The sandbox runs inside it as an ephemeral container. Daemons run OUTSIDE the sandbox, managed by the harness.

## Storage Layout

/var/lib/faultline/
  daemons/
    <agent-id>/
      <daemon-id>/
        config.json         -- spawn config
        state.json          -- pid, uptime, status, restart_count
        stdout.log          -- daemon stdout (JSON Lines)
        stderr.log          -- daemon stderr
        exit_status.json    -- set when daemon exits

## Tool Implementations

### daemon_spawn
- Validate command exists
- Generate daemon_id (ulid prefix)
- Create storage directory
- Write config.json
- exec.Command to spawn process
- Redirect stdout/stderr to log files
- Write state.json with PID
- Start restart monitor goroutine
- Return {daemon_id, name, status, spawned_at}

### daemon_list
- Read all daemon dirs for this agent
- Parse each state.json
- Return array of daemon info

### daemon_fetch
- Open the requested stream log file
- Seek to tail or offset as requested
- Return content with total_bytes and truncation flag

### daemon_stop
- Send SIGTERM to process
- Wait for exit (with timeout)
- If timeout, send SIGKILL
- Update state to stopped

## Restart Policy
- Goroutine watches each daemon process
- On exit with policy "always": wait 5s, respawn
- On exit with policy "never" or exit code 0: mark stopped
- Increment restart_count on each restart

## Agent Memory Injection
On cycle start, harness injects: DAEMON_STATUS={"pending_alerts": N, "running": [...]}
This lets the agent know immediately whether the Guardian has alerts waiting.

## Tool Registration
These are harness-side tools (Tier 1), always available:
- "daemon_spawn" -> Agent.DaemonSpawn
- "daemon_list"  -> Agent.DaemonList
- "daemon_fetch" -> Agent.DaemonFetch
- "daemon_stop"  -> Agent.DaemonStop
- "daemon_logs"  -> Agent.DaemonLogs
