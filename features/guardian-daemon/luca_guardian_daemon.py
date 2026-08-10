#!/usr/bin/env python3
"""
Luca's Guardian -- Persistent Baby Monitor Daemon
=================================================
Monitors nursery audio for cry detection and HA sensors for emergencies.
Writes alerts to shared memory. Respects quiet hours (10 PM - 9 AM ET).

Usage:
  python3 luca_guardian_daemon.py

Environment:
  RTSP_URL          - RTSP stream URL for nursery camera audio
  CRY_VAR_THRESHOLD - Audio variance threshold for cry detection (default: 50000)
  QUIET_START       - Quiet hours start (HH:MM ET, default: "22:00")
  QUIET_END         - Quiet hours end (HH:MM ET, default: "09:00")
  ALERT_PATH        - Path to write alerts (default: "shared/luca/guardian-alerts.json")
  POLL_INTERVAL_S   - Seconds between HA checks (default: 10)
"""
import json
import time
import signal
import os
import sys
import datetime
import urllib.request
from collections import deque

try:
    from zoneinfo import ZoneInfo
    EASTERN_TZ = ZoneInfo("America/New_York")
    UTC_TZ = ZoneInfo("UTC")
except ImportError:  # pragma: no cover - Python < 3.9 fallback
    EASTERN_TZ = datetime.timezone(datetime.timedelta(hours=-4))
    UTC_TZ = datetime.timezone.utc

RUNNING = True

RTSP_URL = os.environ.get("RTSP_URL", "rtsp://example.com/audio")
CRY_VAR_THRESHOLD = int(os.environ.get("CRY_VAR_THRESHOLD", "50000"))
QUIET_START = os.environ.get("QUIET_START", "22:00")
QUIET_END = os.environ.get("QUIET_END", "09:00")
ALERT_PATH = os.environ.get("ALERT_PATH", "shared/luca/guardian-alerts.json")
POLL_INTERVAL_S = int(os.environ.get("POLL_INTERVAL_S", "10"))


def log(level, message, **kwargs):
    """Emit a JSON Lines log entry to stdout (harness captures this)."""
    entry = {
        "timestamp": datetime.datetime.now(UTC_TZ).isoformat().replace("+00:00", "Z"),
        "level": level,
        "message": message,
        **kwargs,
    }
    print(json.dumps(entry), flush=True)


def signal_handler(signum, frame):
    global RUNNING
    log("info", f"Guardian received signal {signum}, shutting down")
    RUNNING = False


signal.signal(signal.SIGTERM, signal_handler)
signal.signal(signal.SIGINT, signal_handler)


def _parse_hhmm(s):
    """Parse 'HH:MM' into total minutes. Raises ValueError on bad input."""
    parts = s.split(":")
    if len(parts) != 2:
        raise ValueError(f"Expected HH:MM, got {s!r}")
    h, m = int(parts[0]), int(parts[1])
    if not (0 <= h <= 23 and 0 <= m <= 59):
        raise ValueError(f"Out-of-range time: {s!r}")
    return h * 60 + m


def is_quiet_hours(now_eastern=None):
    """Check if the current Eastern Time is within quiet hours.

    Handles quiet hours that span midnight (e.g., 22:00 -> 09:00).
    """
    if now_eastern is None:
        now_eastern = datetime.datetime.now(EASTERN_TZ)
    current_min = now_eastern.hour * 60 + now_eastern.minute
    start_min = _parse_hhmm(QUIET_START)
    end_min = _parse_hhmm(QUIET_END)
    if start_min <= end_min:
        # Same-day window (e.g., 09:00 -> 17:00). End is exclusive.
        return start_min <= current_min < end_min
    # Overnight window (e.g., 22:00 -> 09:00). End is exclusive.
    return current_min >= start_min or current_min < end_min


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
    """Write alert to shared memory for agent to read on wake.

    Append-mode with fsync to ensure per-line atomicity on POSIX.
    For multi-daemon safety in the same file, callers should namespace
    by event_type or use a separate file per daemon.
    """
    alert = {
        "timestamp": datetime.datetime.now(UTC_TZ).isoformat().replace("+00:00", "Z"),
        "type": event_type,
        "severity": severity,  # info, warn, alert
        "message": message,
        "metadata": metadata or {},
    }
    try:
        line = json.dumps(alert) + "\n"
        parent = os.path.dirname(ALERT_PATH)
        if parent and not os.path.exists(parent):
            os.makedirs(parent, exist_ok=True)
        with open(ALERT_PATH, "a") as f:
            f.write(line)
            f.flush()
            os.fsync(f.fileno())
        log("info", f"Alert written: {event_type}", severity=severity)
    except Exception as e:
        log("error", f"Failed to write alert: {e}")


def main():
    log("info", "Luca Guardian starting",
        rtsp_url=RTSP_URL[:50], cry_threshold=CRY_VAR_THRESHOLD,
        quiet_start=QUIET_START, quiet_end=QUIET_END, pid=os.getpid(),
        poll_interval_s=POLL_INTERVAL_S)

    alert_count = 0
    while RUNNING:
        ha_state = check_ha_sensors()
        if ha_state.get("temperature") and ha_state["temperature"] > 78:
            write_alert("temperature", "warn",
                        f"Nursery temperature high: {ha_state['temperature']}F",
                        ha_state)

        # Cry detection is a placeholder; real impl uses ffmpeg + numpy.
        # cry_detected = detect_cry(audio_frame)
        # if cry_detected:
        #     if is_quiet_hours():
        #         log("info", "Cry detected during quiet hours -- logging only")
        #     else:
        #         write_alert("cry", "warn", "Cry detected", {"duration_s": 0})

        alert_count += 1
        if alert_count % 60 == 0:
            log("info", "Guardian heartbeat", uptime_s=alert_count * POLL_INTERVAL_S,
                quiet_hours=is_quiet_hours())

        time.sleep(POLL_INTERVAL_S)

    log("info", "Luca Guardian exiting", total_iterations=alert_count)
    sys.exit(0)


if __name__ == "__main__":
    main()
