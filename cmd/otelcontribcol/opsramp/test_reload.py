"""Run with python3 -m unittest discover -s cmd/otelcontribcol/opsramp -p 'test_*.py'."""

import json
import os
from pathlib import Path
import subprocess
import socket
import tempfile
import time
import unittest
import urllib.error
import urllib.request


LAUNCHER = Path(__file__).with_name("run-otelcontribcol.sh")
MOCK = r"""#!/usr/bin/env python3
import json, os, pathlib, signal, sys, time
root = pathlib.Path(os.environ["TEST_ROOT"])
path = pathlib.Path(sys.argv[sys.argv.index("--config") + 1])
text = path.read_text()
def event(kind):
    with (root / "events").open("a") as stream:
        stream.write(json.dumps([kind, text, os.getpid(), str(path)]) + "\n")
if "validate" in sys.argv:
    event("validate")
    if text == "race":
        replacement = root / "replacement"
        replacement.write_text("after-race")
        replacement.replace(root / "dynamic.json")
    if text in ("invalid", ""):
        sys.exit(1)
    sys.exit(0)
event("start")
if text == "crash":
    sys.exit(2)
def stop(*args):
    event("stop")
    sys.exit(0)
signal.signal(signal.SIGTERM, stop)
while True:
    time.sleep(0.05)
"""


class ReloadTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.static = self.root / "static.json"
        self.dynamic = self.root / "dynamic.json"
        # A legacy fallback must never be used, even if its file and environment remain.
        self.static.write_text("static")
        mock = self.root / "collector"
        mock.write_text(MOCK)
        mock.chmod(0o700)
        self.env = {
            **os.environ,
            "TEST_ROOT": str(self.root),
            "OTELCOL_BINARY": str(mock),
            "OTELCOL_CONFIG": str(self.static),
            "OTEL_LOGS_CONFIG_FILE": str(self.dynamic),
            "OTELCOL_OUTPUT_DIR": str(self.root),
            "OTELCOL_WATCH_INTERVAL": "1",
            "OTELCOL_STARTUP_TIMEOUT": "1",
            "OTELCOL_STOP_TIMEOUT": "2",
            "OTEL_LOGS_HEALTH_PORT": "",
        }
        self.log = (self.root / "supervisor.log").open("w+")
        self.addCleanup(self.log.close)
        self.process = None
        self.addCleanup(self.stop)

    def start(self):
        self.process = subprocess.Popen(
            ["sh", str(LAUNCHER)], env=self.env, stdout=self.log, stderr=self.log
        )

    def stop(self):
        if self.process and self.process.poll() is None:
            self.process.terminate()
            self.process.wait(timeout=10)

    def events(self, kind):
        path = self.root / "events"
        return [e for line in path.read_text().splitlines()
                if (e := json.loads(line))[0] == kind] if path.exists() else []

    def wait_event(self, kind, text, count=1):
        deadline = time.monotonic() + 12
        while time.monotonic() < deadline:
            matches = [e for e in self.events(kind) if e[1] == text]
            if len(matches) >= count:
                return matches[-1]
            if self.process.poll() is not None:
                break
            time.sleep(0.05)
        self.fail(f"Missing {kind}/{text}: {(self.root / 'supervisor.log').read_text()}")

    def replace(self, path, text):
        replacement = self.root / "replacement"
        replacement.write_text(text)
        replacement.replace(path)

    def test_missing_and_empty_do_not_invoke_collector_then_start(self):
        self.start()
        time.sleep(2.2)
        self.assertEqual([], self.events("validate"))
        self.assertEqual([], self.events("start"))
        self.dynamic.write_text("")
        time.sleep(2.2)
        self.assertEqual([], self.events("validate"))
        self.assertEqual([], self.events("start"))
        self.assertIsNone(self.process.poll())
        self.replace(self.dynamic, "dynamic")
        self.wait_event("start", "dynamic")

    def test_replacements_invalid_empty_delete_and_recreate(self):
        self.dynamic.write_text("first")
        self.start()
        self.wait_event("start", "first")
        self.replace(self.dynamic, "dynamic")
        dynamic = self.wait_event("start", "dynamic")
        self.replace(self.dynamic, "invalid")
        self.wait_event("validate", "invalid")
        time.sleep(2.2)
        self.assertEqual(1, len([e for e in self.events("validate") if e[1] == "invalid"]))
        self.assertNotIn(dynamic[2], [e[2] for e in self.events("stop")])
        self.replace(self.dynamic, "")
        self.wait_event("stop", "dynamic")
        time.sleep(1.2)
        self.assertEqual([], list(self.root.glob(".otel-config.*/active.json")))
        self.replace(self.dynamic, "dynamic")
        self.wait_event("start", "dynamic", count=2)
        self.dynamic.unlink()
        self.wait_event("stop", "dynamic", count=2)
        time.sleep(1.2)
        self.replace(self.dynamic, "recreated")
        self.wait_event("start", "recreated")
        self.stop()
        self.assertEqual(0, self.process.returncode)
        self.assertEqual({e[2] for e in self.events("start")},
                         {e[2] for e in self.events("stop")})
        self.assertFalse(list(self.root.glob(".otel-config.*")))

    def test_source_replacement_during_validation_uses_validated_snapshot(self):
        self.dynamic.write_text("race")
        self.start()
        first = self.wait_event("start", "race")
        self.assertNotEqual(str(self.dynamic), first[3])
        self.wait_event("start", "after-race")

    def test_startup_failure_rolls_back_and_later_recovers(self):
        self.dynamic.write_text("first")
        self.start()
        self.wait_event("start", "first")
        self.replace(self.dynamic, "crash")
        self.wait_event("start", "crash")
        self.wait_event("start", "first", count=2)
        time.sleep(2.2)
        self.assertEqual(1, len([e for e in self.events("start") if e[1] == "crash"]))
        self.replace(self.dynamic, "recovered")
        self.wait_event("start", "recovered")

    def test_invalid_dynamic_at_boot_waits_for_correction(self):
        self.dynamic.write_text("invalid")
        self.start()
        self.wait_event("validate", "invalid")
        time.sleep(2.2)
        self.assertEqual([], self.events("start"))
        self.assertIsNone(self.process.poll())
        self.replace(self.dynamic, "corrected")
        self.wait_event("start", "corrected")

    def test_initial_startup_failure_waits_for_correction(self):
        self.dynamic.write_text("crash")
        self.start()
        self.wait_event("start", "crash")
        time.sleep(2.2)
        self.assertIsNone(self.process.poll())
        self.replace(self.dynamic, "corrected")
        self.wait_event("start", "corrected")

    def test_invalid_interval_is_rejected(self):
        self.env["OTELCOL_WATCH_INTERVAL"] = "0"
        self.start()
        self.assertNotEqual(0, self.process.wait(timeout=5))
        self.assertEqual([], self.events("start"))

    @unittest.skipUnless(os.environ.get("OTELCOL_TEST_BINARY"), "requires a real collector binary")
    def test_real_collector_runtime_failure_restores_healthy_previous_config(self):
        def port():
            with socket.socket() as sock:
                sock.bind(("127.0.0.1", 0))
                return sock.getsockname()[1]

        health_port = port()
        self.env["OTELCOL_BINARY"] = os.environ["OTELCOL_TEST_BINARY"]
        self.env["OTEL_LOGS_HEALTH_PORT"] = str(health_port)
        self.env["OTELCOL_STARTUP_TIMEOUT"] = "4"
        config = {
            "extensions": {"health_check": {"endpoint": f"127.0.0.1:{health_port}"}},
            "receivers": {"otlp": {"protocols": {"http": {"endpoint": f"127.0.0.1:{port()}"}}}},
            "exporters": {"debug": {}},
            "service": {"extensions": ["health_check"],
                        "telemetry": {"metrics": {"level": "none"}},
                        "pipelines": {"logs": {"receivers": ["otlp"], "exporters": ["debug"]}}},
        }
        original = json.dumps(config)

        def wait_healthy():
            deadline = time.monotonic() + 12
            while time.monotonic() < deadline:
                try:
                    with urllib.request.urlopen(f"http://127.0.0.1:{health_port}/", timeout=1) as response:
                        if response.status == 200:
                            return
                except urllib.error.URLError:
                    pass
                self.assertIsNone(self.process.poll(), (self.root / "supervisor.log").read_text())
                time.sleep(0.1)
            self.fail((self.root / "supervisor.log").read_text())

        self.start()
        time.sleep(2.2)
        self.assertIsNone(self.process.poll())
        self.assertNotIn("starting otelcontribcol", (self.root / "supervisor.log").read_text())
        self.dynamic.write_text("")
        time.sleep(1.2)
        self.assertNotIn("starting otelcontribcol", (self.root / "supervisor.log").read_text())
        self.replace(self.dynamic, original)
        wait_healthy()
        with socket.socket() as occupied:
            occupied.bind(("127.0.0.1", 0))
            occupied.listen()
            config["receivers"]["otlp"]["protocols"]["http"]["endpoint"] = f"127.0.0.1:{occupied.getsockname()[1]}"
            self.replace(self.dynamic, json.dumps(config))
            deadline = time.monotonic() + 12
            while "rolling back" not in (self.root / "supervisor.log").read_text():
                self.assertLess(time.monotonic(), deadline, (self.root / "supervisor.log").read_text())
                time.sleep(0.1)
            wait_healthy()
        self.dynamic.unlink()
        deadline = time.monotonic() + 12
        while True:
            try:
                with urllib.request.urlopen(f"http://127.0.0.1:{health_port}/", timeout=1):
                    pass
            except urllib.error.URLError:
                break
            self.assertLess(time.monotonic(), deadline)
            time.sleep(0.1)
        self.assertIsNone(self.process.poll())
        self.replace(self.dynamic, original)
        wait_healthy()
        self.stop()
        self.assertEqual(0, self.process.returncode)
        self.assertFalse(list(self.root.glob(".otel-config.*")))


if __name__ == "__main__":
    unittest.main()
