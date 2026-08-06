#!/usr/bin/env python3
"""
Luca's Guardian -- Persistent Baby Monitor Daemon
=================================================
Monitors nursery audio for cry detection and HA sensors for emergencies.
Writes alerts to shared memory. Respects quiet hours (10 PM - 9 AM ET).

Usage:
  python3 luca-guardian-daemon.py

Environment:
  RTSP_URL         - RTSP stream URL for nursery camera audio
  CRY_VAR_THRESHOLD - Audio variance threshold for cry detection (default: 50000)
  QUIET_START      - Quiet hours start (HH:MM ET, default: "22:00")
  QUIET_END        - Quiet hours end (HH:MM ET, default: "09:00")
  ALERT_PATH       - Path to write alerts (default: "shared/luca/guardian-alerts.json")
"""

import json, time, signal, os, sys, datetime, subprocess, urllib.request
from collections import deque

RUNNING = True

# Load config from environment
RTSP_URL = os.environ.get("RTSP_URL", "rtsp://example.com/audio")
CRY_VAR_THRESHOLD = int(os.environ.get("CRY_VAR_THRESHOLD", "50000"))
QUIET_START = os.environ.get("QUIET_START", "22:00")
QUIET_END = os.environ.get("QUIET_END", "09:00")
ALERT_PATH = os.environ.get("ALERT_PATH", "shared/luca/guardian-alerts.json")

def log(level, message, **kwargs):
    entry = {"timestamp": datetime.datetime.utcnow().isoformat() + "Z",
             "level": level, "message": message, **kwargs}
    print(json.dumps(entry), flush=True)

def signal_handler(signum, frame):
    global RUNNING
    log("info", f"Guardian received signal {signum}, shutting down")
    RUNNING = False

signal.signal(signal.SIGTERM, signal_handler)
signal.signal(signal.SIGINT, signal_handler)

def is_quiet_hours():
    """Check if current time is within quiet hours (Eastern Time)."""
    now = datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=-5)))
    current_min = now.hour * 60 + now.minute
    start_min = int(QUIET_START.split(":")[0]) * 60 + int(QUIET_START.split(":")[1])
    end_min = int(QUIET_END.split(":")[0]) * 60 + int(QUIET_END.split(":")[1])
    if start_min < end_min:
        return start_min <= current_min <= end_min
    else:
        return current_min >= start_min or current_min <= end_min

def check_ha_sensors():
    """Query HA for nursery temperature and motion. Returns dict."""
    # HA API call would go here -- placeholder for now
    return {"temperature": None, "motion": None}

def detect_cry(audio_frame):
    """Detect baby cry from audio frame. Returns True if cry detected."""
    # Variance-based detection: compute RMS energy of audio frame
    # High variance = sustained sound = likely crying
    # This is a placeholder -- real implementation uses ffmpeg + numpy
    return False

def write_alert(event_type, severity, message, metadata=None):
    """Write alert to shared memory for agent to read on wake."""
    alert = {
        "timestamp": datetime.datetime.utcnow().isoformat() + "Z",
        "type": event_type,
        "severity": severity,  # info, warn, alert
        "message": message,
        "metadata": metadata or {}
    }
    try:
        with open(ALERT_PATH, "a") as f:
            f.write(json.dumps(alert) + "\n")
        log("info", f"Alert written: {event_type}", severity=severity)
    except Exception as e:
        log("error", f"Failed to write alert: {e}")

def main():
    log("info", "Luca Guardian starting",
        rtsp_url=RTSP_URL[:50], cry_threshold=CRY_VAR_THRESHOLD,
        quiet_start=QUIET_START, quiet_end=QUIET_END, pid=os.getpid())

    alert_count = 0
    while RUNNING:
        # Check HA sensors
        ha_state = check_ha_sensors()
        if ha_state.get("temperature") and ha_state["temperature"] > 78:
            write_alert("temperature", "warn",
                        f"Nursery temperature high: {ha_state['temperature']}F",
                        ha_state)

        # Check audio for cry (simplified -- real impl uses ffmpeg + numpy)
        # cry_detected = detect_cry(audio_frame)
        # if cry_detected:
        #     quiet = is_quiet_hours()
        #     if quiet:
        #         log("info", "Cry detected during quiet hours -- logging only")
        #     else:
        #         write_alert("cry", "warn", "Cry detected", {"duration_s": 0})

        alert_count += 1
        if alert_count % 60 == 0:
            log("info", f"Guardian heartbeat", uptime_s=alert_count * 10,
                quiet_hours=is_quiet_hours())

        time.sleep(10)

    log("info", "Luca Guardian exiting")
    sys.exit(0)

if __name__ == "__main__":
    main()
