#!/usr/bin/env python3
"""
Unit tests for luca-guardian-daemon.py
Run: python3 -m pytest features/guardian-daemon/test_guardian_daemon.py -v
Or:  python3 features/guardian-daemon/test_guardian_daemon.py
"""
import datetime
import io
import json
import os
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from unittest import mock

sys.path.insert(0, os.path.join(os.path.dirname(__file__)))
import luca_guardian_daemon as guardian


class TestParseHhmm(unittest.TestCase):
    def test_valid(self):
        self.assertEqual(guardian._parse_hhmm("00:00"), 0)
        self.assertEqual(guardian._parse_hhmm("09:30"), 570)
        self.assertEqual(guardian._parse_hhmm("22:00"), 1320)
        self.assertEqual(guardian._parse_hhmm("23:59"), 1439)

    def test_invalid_raises(self):
        for bad in ["", "09-00", "24:00", "12:60", "abc"]:
            with self.subTest(bad=bad):
                with self.assertRaises(ValueError):
                    guardian._parse_hhmm(bad)


class TestIsQuietHours(unittest.TestCase):
    """Overnight window 22:00 -> 09:00 (default). Same-day window 09:00 -> 17:00."""

    def _east(self, hh, mm):
        return datetime.datetime(2026, 8, 9, hh, mm, tzinfo=guardian.EASTERN_TZ)

    def test_overnight_late_night(self):
        self.assertTrue(guardian.is_quiet_hours(self._east(23, 0)))
        self.assertTrue(guardian.is_quiet_hours(self._east(0, 30)))
        self.assertTrue(guardian.is_quiet_hours(self._east(8, 59)))

    def test_overnight_safe_hours(self):
        self.assertFalse(guardian.is_quiet_hours(self._east(9, 0)))
        self.assertFalse(guardian.is_quiet_hours(self._east(12, 0)))
        self.assertFalse(guardian.is_quiet_hours(self._east(21, 59)))

    def test_overnight_boundary(self):
        # 22:00 inclusive (start), 09:00 exclusive (end)
        self.assertTrue(guardian.is_quiet_hours(self._east(22, 0)))
        self.assertFalse(guardian.is_quiet_hours(self._east(9, 0)))
        self.assertTrue(guardian.is_quiet_hours(self._east(8, 59)))

    def test_daytime_window(self):
        with mock.patch.object(guardian, "QUIET_START", "09:00"), \
             mock.patch.object(guardian, "QUIET_END", "17:00"):
            self.assertTrue(guardian.is_quiet_hours(self._east(12, 0)))
            self.assertFalse(guardian.is_quiet_hours(self._east(8, 59)))
            self.assertFalse(guardian.is_quiet_hours(self._east(17, 0)))


class TestLog(unittest.TestCase):
    def test_emits_valid_jsonl(self):
        buf = io.StringIO()
        with redirect_stdout(buf):
            guardian.log("info", "test message", counter=42)
        line = buf.getvalue().strip()
        parsed = json.loads(line)
        self.assertEqual(parsed["level"], "info")
        self.assertEqual(parsed["message"], "test message")
        self.assertEqual(parsed["counter"], 42)
        self.assertIn("timestamp", parsed)
        self.assertTrue(parsed["timestamp"].endswith("Z"))


class TestWriteAlert(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.alert_path = os.path.join(self.tmpdir, "alerts.json")

    def tearDown(self):
        import shutil
        if os.path.exists(self.tmpdir):
            shutil.rmtree(self.tmpdir)

    def test_appends_jsonl_line(self):
        with mock.patch.object(guardian, "ALERT_PATH", self.alert_path):
            guardian.write_alert("cry", "warn", "Cry detected", {"duration_s": 30})
        with open(self.alert_path) as f:
            lines = [l for l in f.read().splitlines() if l.strip()]
        self.assertEqual(len(lines), 1)
        parsed = json.loads(lines[0])
        self.assertEqual(parsed["type"], "cry")
        self.assertEqual(parsed["severity"], "warn")
        self.assertEqual(parsed["message"], "Cry detected")
        self.assertEqual(parsed["metadata"]["duration_s"], 30)

    def test_creates_parent_dir(self):
        nested = os.path.join(self.tmpdir, "shared", "luca", "alerts.json")
        with mock.patch.object(guardian, "ALERT_PATH", nested):
            guardian.write_alert("test", "info", "test")
        self.assertTrue(os.path.exists(nested))

    def test_multiple_appends(self):
        with mock.patch.object(guardian, "ALERT_PATH", self.alert_path):
            guardian.write_alert("first", "info", "1")
            guardian.write_alert("second", "info", "2")
            guardian.write_alert("third", "info", "3")
        with open(self.alert_path) as f:
            lines = [l for l in f.read().splitlines() if l.strip()]
        self.assertEqual(len(lines), 3)
        self.assertEqual(json.loads(lines[0])["message"], "1")
        self.assertEqual(json.loads(lines[2])["message"], "3")


class TestDetectCry(unittest.TestCase):
    def test_placeholder_returns_false(self):
        self.assertFalse(guardian.detect_cry(b"\x00" * 1024))


class TestCheckHASensors(unittest.TestCase):
    def test_returns_expected_shape(self):
        result = guardian.check_ha_sensors()
        self.assertIn("temperature", result)
        self.assertIn("motion", result)


if __name__ == "__main__":
    unittest.main(verbosity=2)
