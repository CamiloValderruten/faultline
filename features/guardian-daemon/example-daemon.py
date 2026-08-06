#!/usr/bin/env python3
"""
Minimal Example Daemon for Faultline Harness
-----------------------------------------
This demonstrates the daemon interface: JSON Lines on stdout,
signal handling for graceful shutdown, structured logging.
"""

import json, time, signal, os, sys
from datetime import datetime

RUNNING = True

def log(level, message, **kwargs):
    entry = {"timestamp": datetime.utcnow().isoformat() + "Z",
             "level": level, "message": message, **kwargs}
    print(json.dumps(entry), flush=True)

def signal_handler(signum, frame):
    global RUNNING
    log("info", f"Received signal {signum}, shutting down gracefully")
    RUNNING = False

signal.signal(signal.SIGTERM, signal_handler)
signal.signal(signal.SIGINT, signal_handler)

def main():
    interval = int(os.environ.get("INTERVAL_S", "10"))
    counter = 0
    log("info", f"Example daemon started", interval_s=interval, pid=os.getpid())
    while RUNNING:
        counter += 1
        log("info", f"Heartbeat #{counter}",
            counter=counter, uptime_s=counter * interval)
        if counter % 6 == 0:
            log("warn", "Periodic status report",
                total_heartbeats=counter, uptime_minutes=round(counter * interval / 60, 1))
        try:
            time.sleep(interval)
        except KeyboardInterrupt:
            break
    log("info", "Example daemon exiting", total_heartbeats=counter)
    sys.exit(0)

if __name__ == "__main__":
    main()
