#!/usr/bin/env python3
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

"""Exercise an actual Collector against SNMP Simulator and Net-SNMP."""

import argparse
import base64
import copy
from contextlib import contextmanager
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import sys
import time
import uuid

import yaml

ROOT = Path(__file__).resolve().parent
UPTIME = "1.3.6.1.2.1.1.3.0"
IDENTITY = "1.3.6.1.6.3.1.1.4.1.0"
NAME = "1.3.6.1.2.1.1.5.0"
TRAP = "1.3.6.1.6.3.1.1.5.1"
WARNING = "Top-level SNMP polling configuration is deprecated"


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def port_free(port):
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
        try:
            sock.bind(("127.0.0.1", port))
            return True
        except OSError:
            return False


def wait_for(predicate, processes=(), timeout=20):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        for process in processes:
            require(process.poll() is None, "Process exited before the assertion completed")
        result = predicate()
        if result:
            return result
        time.sleep(0.1)
    raise AssertionError("Timed out waiting for exported data or process readiness")


@contextmanager
def process(command, directory, log):
    with log.open("w") as output:
        child = subprocess.Popen(command, cwd=directory, stdout=output, stderr=subprocess.STDOUT)
        try:
            yield child
        finally:
            if child.poll() is None:
                child.send_signal(signal.SIGTERM)
            try:
                child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait(timeout=3)
                raise AssertionError(f"Process required SIGKILL; inspect {log}")


def command(arguments, directory, expect_success=True):
    persistent = directory / "net-snmp"
    persistent.mkdir(exist_ok=True)
    environment = dict(os.environ, SNMP_PERSISTENT_DIR=str(persistent), MIBS="")
    path = directory / f"command-{uuid.uuid4().hex[:8]}.json"
    try:
        result = subprocess.run(arguments, capture_output=True, text=True, timeout=8, env=environment)
    except subprocess.TimeoutExpired as error:
        def text_output(output):
            return output.decode(errors="replace") if isinstance(output, bytes) else (output or "")
        path.write_text(json.dumps({"argv": arguments, "timeout_seconds": error.timeout,
                                   "stdout": text_output(error.stdout),
                                   "stderr": text_output(error.stderr)}, indent=2))
        raise AssertionError(f"Command timed out; inspect {path}") from error
    path.write_text(json.dumps({"argv": arguments, "exit_code": result.returncode,
                               "stdout": result.stdout, "stderr": result.stderr}, indent=2))
    if expect_success:
        require(result.returncode == 0, f"Command failed; inspect {path}")
    return result


def documents(path):
    if not path.exists():
        return []
    content = path.read_text()
    lines = content.splitlines()
    result = []
    for index, line in enumerate(lines):
        try:
            result.append(json.loads(line))
        except json.JSONDecodeError:
            require(index == len(lines) - 1 and not content.endswith("\n"), f"Invalid OTLP JSON in {path}")
    return result


def value(item):
    if "kvlistValue" in item:
        return {entry["key"]: value(entry["value"]) for entry in item["kvlistValue"].get("values", [])}
    if "arrayValue" in item:
        return [value(entry) for entry in item["arrayValue"].get("values", [])]
    if "bytesValue" in item:
        return base64.b64decode(item["bytesValue"])
    if "intValue" in item:
        return int(item["intValue"])
    return next(iter(item.values()), None)


def attributes(items):
    return {item["key"]: value(item["value"]) for item in items}


def metrics(directory):
    result = []
    for doc in documents(directory / "metrics.json"):
        for resource in doc.get("resourceMetrics", []):
            for scope in resource.get("scopeMetrics", []):
                for metric in scope.get("metrics", []):
                    kind = "gauge" if metric["name"] == "device.uptime" else "sum"
                    require(metric.get("unit") == ("1" if kind == "gauge" else "By"), "Wrong metric unit")
                    if kind == "sum":
                        require(metric[kind].get("isMonotonic") is True and metric[kind].get("aggregationTemporality") == 2,
                                "Indexed counters must be cumulative monotonic sums")
                    for point in metric.get(kind, {}).get("dataPoints", []):
                        result.append((metric["name"], int(point.get("asInt", -1)),
                                       attributes(point.get("attributes", [])), int(point["timeUnixNano"])))
    return result


def checked_metrics(directory, minimum=2):
    points = metrics(directory)
    times = sorted({point[3] for point in points if point[0] == "device.uptime"})
    if len(times) < minimum:
        return None
    require(all(point[1] == 123456 for point in points if point[0] == "device.uptime"), "Wrong scalar value")
    require(all(right - left >= 500_000_000 for left, right in zip(times, times[1:])), "Incorrect metric cadence")
    for timestamp in times:
        columns = {(point[2].get("interface.index"), point[1]) for point in points
                   if point[0] == "device.interface.in_octets" and point[3] == timestamp}
        require(columns == {("if.1", 1000), ("if.2", 2000)}, "Missing or incorrect indexed counter values")
    return times


def logs(directory):
    result = []
    for doc in documents(directory / "logs.json"):
        for resource in doc.get("resourceLogs", []):
            for scope in resource.get("scopeLogs", []):
                for record in scope.get("logRecords", []):
                    body = value(record.get("body", {}))
                    bindings = body.get("varbinds", [])
                    marker = next((binding["value"] for binding in bindings if binding["oid"] == NAME), b"")
                    result.append((marker, body, attributes(record.get("attributes", [])), record))
    return result


def checked_log(directory, marker, version="v2c", kind="trap"):
    records = [record for record in logs(directory) if record[0] == marker.encode()]
    if not records:
        return None
    require(len(records) == 1, f"Duplicate notification: {marker}")
    _, body, attrs, record = records[0]
    require(body["version"] == version and body["pdu_type"] == kind, "Wrong notification type")
    require(body["trap_oid"] == TRAP and body["sys_up_time"] == 123456, "Wrong notification identity or uptime")
    require("community" not in body, "Community should be omitted by default")
    require(attrs["snmp.version"] == version and attrs["snmp.pdu.type"] == kind
            and attrs["snmp.trap.oid"] == TRAP, "Log attributes do not match body")
    require(attrs["network.peer.address"] == "127.0.0.1" and int(attrs["network.peer.port"]) > 0, "Wrong sender")
    require(int(record.get("observedTimeUnixNano", 0)) > 0 and int(record.get("timeUnixNano", 0)) == 0,
            "Incorrect notification timestamps")
    bindings = body["varbinds"]
    require(any(binding["oid"] == NAME and binding["type"] == "OctetString" for binding in bindings), "Missing binding")
    if version != "v1":
        require([(binding["oid"], binding["type"], binding["value"]) for binding in bindings[:2]]
                == [(UPTIME, "TimeTicks", 123456), (IDENTITY, "ObjectIdentifier", TRAP)], "Wrong standard bindings")
    else:
        require(body["generic_trap"] == 0 and body["specific_trap"] == 0 and body["enterprise"], "Wrong v1 fields")
    if version == "v3":
        require(body["context_engine_id"] and "context_name" in body, "Missing v3 context")
    return records[0]


def configuration(template, directory, polling=True, notifications=False):
    config = copy.deepcopy(template)
    receiver = config["receivers"]["snmp/device"]
    for role, enabled in (("poll", polling), ("traps", notifications)):
        if not enabled:
            receiver.pop(role, None)
    for signal_name, enabled in (("metrics", polling), ("logs", notifications)):
        if not enabled:
            config["service"]["pipelines"].pop(signal_name, None)
            config["exporters"].pop(f"file/{signal_name}", None)
        else:
            config["exporters"][f"file/{signal_name}"]["path"] = str(directory / f"{signal_name}.json")
    return config


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--collector", default=str(ROOT / ".work/collector/snmp-simulation"))
    parser.add_argument("--simulator", default=str(ROOT / ".venv/bin/snmpsim-command-responder"))
    parser.add_argument("--poll-only", action="store_true")
    args = parser.parse_args()
    directory = ROOT / ".work/runs" / (datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ-") + uuid.uuid4().hex[:8])
    directory.mkdir(parents=True)
    summary = {"collector": str(Path(args.collector).resolve()), "poll_only": args.poll_only, "cases": [], "status": "failed"}
    try:
        for binary in (args.collector, args.simulator, "snmpget", "snmpset", "snmpinform", "snmptrap"):
            require(shutil.which(binary), f"Missing executable: {binary}")
        args.collector = str(Path(shutil.which(args.collector)).resolve())
        args.simulator = str(Path(shutil.which(args.simulator)).resolve())
        summary["collector"] = args.collector
        summary["collector_sha256"] = hashlib.sha256(Path(args.collector).read_bytes()).hexdigest()
        summary["python_version"] = sys.version
        build_info = Path(args.collector).parent / "build-info.json"
        if build_info.exists():
            summary["build_info"] = json.loads(build_info.read_text())
        for port in (1161, 1620):
            require(port_free(port), f"UDP port {port} is already in use; stop its owner manually")
        template = yaml.safe_load((ROOT / "collector.yaml").read_text())
        sim_command = [str(Path(args.simulator).resolve()), f"--data-dir={ROOT / 'data'}",
                       f"--cache-dir={directory / 'cache'}", "--agent-udpv4-endpoint=127.0.0.1:1161",
                       "--v3-user=sim-user", "--v3-auth-proto=SHA", "--v3-auth-key=authpass123",
                       "--v3-priv-proto=AES", "--v3-priv-key=privpass123"]
        with process(sim_command, ROOT, directory / "simulator.log") as simulator:
            def source_ready():
                result = command(["snmpget", "-v2c", "-c", "public", "-r", "0", "-t", "1",
                                  "-Otqv", "127.0.0.1:1161", UPTIME], directory, False)
                return result.returncode == 0 and result.stdout.strip() == "123456"
            wait_for(source_ready, (simulator,))
            cases = ["nested-v2c", "nested-v1", "nested-v3", "legacy-v2c", "mixed"]
            if not args.poll_only:
                cases += ["combined", "traps-only"]
            for name in cases:
                case = directory / name
                case.mkdir()
                config = configuration(template, case, name != "traps-only", name in ("combined", "traps-only"))
                receiver = config["receivers"]["snmp/device"]
                if name in ("nested-v1", "nested-v3"):
                    receiver["poll"]["version"] = name.removeprefix("nested-")
                if name == "nested-v3":
                    receiver["poll"].update(template["receivers"]["snmp/device"]["traps"]["v3"])
                    receiver["poll"].pop("community", None)
                if name == "legacy-v2c":
                    receiver.update(receiver.pop("poll"))
                if name == "mixed":
                    receiver["endpoint"] = "udp://127.0.0.1:1161"
                path = case / "collector.yaml"
                path.write_text(yaml.safe_dump(config, sort_keys=False))
                if name == "mixed":
                    result = command([args.collector, "validate", f"--config={path}"], case, False)
                    require(result.returncode != 0 and "poll cannot be combined" in result.stderr + result.stdout,
                            "Mixed polling forms must fail validation")
                else:
                    with process([str(Path(args.collector).resolve()), f"--config={path}"], ROOT, case / "collector.log") as collector:
                        active = (simulator, collector)
                        before = wait_for(lambda: checked_metrics(case), active) if name != "traps-only" else []
                        if name in ("combined", "traps-only"):
                            wait_for(lambda: not port_free(1620), active)
                            triggers = [(1, "sim-v2c-trap", "v2c", "trap"),
                                        (3, "sim-v1-trap", "v1", "trap"), (4, "sim-v3-trap", "v3", "trap")]
                            for index, marker, version, kind in triggers:
                                command(["snmpset", "-v2c", "-c", "public", "-r", "0", "-t", "3", "127.0.0.1:1161",
                                         f"1.3.6.1.4.1.8072.9999.{index}.0", "i", "1"], case)
                                wait_for(lambda: checked_log(case, marker, version, kind), active)
                            inform = ["snmpinform", "-v2c", "-c", "public", "-r", "0", "-t", "2",
                                      "127.0.0.1:1620", "123456", TRAP, NAME, "s", "net-snmp-inform"]
                            command(inform, case)
                            wait_for(lambda: checked_log(case, "net-snmp-inform", kind="inform"), active)
                            denied = inform.copy()
                            denied[3], denied[-1] = "denied", "denied-community"
                            result = command(denied, case, False)
                            require(result.returncode != 0 and "Timeout" in result.stderr, "Unauthorized Inform was acknowledged")
                            command(["snmptrap", "-v3", "-l", "authPriv", "-u", "sim-user", "-a", "SHA", "-A", "wrongpass123",
                                     "-x", "AES", "-X", "privpass123", "-e", "0x80001f888001", "127.0.0.1:1620",
                                     "123456", TRAP, NAME, "s", "denied-v3-auth"], case)
                            command(["snmptrap", "-v3", "-l", "authPriv", "-u", "sim-user", "-a", "SHA", "-A", "authpass123",
                                     "-x", "AES", "-X", "privpass123", "-e", "0x80001f888001", "127.0.0.1:1620",
                                     "123456", TRAP, NAME, "s", "valid-v3-auth"], case)
                            wait_for(lambda: checked_log(case, "valid-v3-auth", version="v3"), active)
                            typed_oid = "1.3.6.1.4.1.8072.9999"
                            command(["snmptrap", "-v2c", "-c", "public", "127.0.0.1:1620", "123456", TRAP,
                                     NAME, "s", "net-snmp-types", typed_oid + ".10.0", "i", "-42",
                                     typed_oid + ".11.0", "c", "4294967295", typed_oid + ".12.0", "C", "18446744073709551615",
                                     typed_oid + ".13.0", "x", "00ff80", typed_oid + ".14.0", "o", UPTIME], case)
                            typed = wait_for(lambda: checked_log(case, "net-snmp-types"), active)
                            require([(binding["oid"], binding["type"], binding["value"])
                                     for binding in typed[1]["varbinds"][3:]] == [
                                         (typed_oid + ".10.0", "Integer", -42),
                                         (typed_oid + ".11.0", "Counter32", 4294967295),
                                         (typed_oid + ".12.0", "Counter64", "18446744073709551615"),
                                         (typed_oid + ".13.0", "OctetString", b"\x00\xff\x80"),
                                         (typed_oid + ".14.0", "ObjectIdentifier", UPTIME),
                                     ], "Typed bindings were changed or reordered")
                            inform[-1] = "after-rejection-inform"
                            command(inform, case)
                            wait_for(lambda: checked_log(case, "after-rejection-inform", kind="inform"), active)
                            quiet_until = time.monotonic() + 1
                            def rejected_remain_absent():
                                require(not any(record[0].startswith(b"denied-") for record in logs(case)),
                                        "Unauthorized notification reached the exporter")
                                return time.monotonic() >= quiet_until
                            wait_for(rejected_remain_absent, active)
                            if name == "combined":
                                checkpoint = len(checked_metrics(case))
                                wait_for(lambda: checked_metrics(case, checkpoint + 2), active)
                        warning = WARNING in (case / "collector.log").read_text()
                        require(warning == (name == "legacy-v2c"), "Unexpected legacy configuration warning")
                    require(collector.returncode == 0, f"Collector shutdown failed: {case}")
                    require(port_free(1620), "Trap port was not released")
                    if name in ("combined", "traps-only"):
                        require(len(logs(case)) == 7, "Unexpected exported notifications, duplicates or unauthorized packet")
                summary["cases"].append({"name": name, "status": "passed"})
                print(f"PASS {name}", flush=True)
        require(port_free(1161), "Simulator port was not released")
        summary["status"] = "passed"
    except Exception as error:
        summary["error"] = str(error)
        raise
    finally:
        (directory / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
        print(f"Artifacts: {directory}", flush=True)


if __name__ == "__main__":
    main()
