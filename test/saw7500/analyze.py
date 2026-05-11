#!/usr/bin/env python3
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

"""Analyze SAW-7500 local red/green proof artifacts."""

from __future__ import annotations

import argparse
import glob
import json
import math
import os
import re
from collections import defaultdict
from pathlib import Path


SAMPLE_RE = re.compile(
    r"^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{([^}]*)\})?\s+([-+]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][-+]?\d+)?|[+-]?Inf|NaN)$"
)
ATTR_RE = re.compile(r'([a-zA-Z_][a-zA-Z0-9_]*)="([^"]*)"')

QUEUE_BYTES = "otelcol_loadbalancer_central_queue_compressed_bytes"
QUEUE_CAPACITY = "otelcol_loadbalancer_central_queue_compressed_capacity"
QUEUE_ITEMS = "otelcol_loadbalancer_central_queue_items"
QUEUE_AGE = "otelcol_loadbalancer_central_queue_oldest_item_age_milliseconds"
REJECTED = "otelcol_loadbalancer_central_queue_rejected_compressed_bytes"
INFLIGHT_UNCOMPRESSED = "otelcol_loadbalancer_central_queue_inflight_uncompressed_bytes"
ACTIVE_CONSUMERS = "otelcol_loadbalancer_central_queue_active_consumers"
CONFIGURED_CONSUMERS = "otelcol_loadbalancer_central_queue_configured_consumers"
LANES = "otelcol_loadbalancer_central_queue_lanes"
PROCESS_RSS = "otelcol_process_memory_rss_bytes"
PROCESS_HEAP = "otelcol_process_runtime_heap_alloc_bytes"
PROCESS_SYS = "otelcol_process_runtime_total_sys_memory_bytes"
BACKEND_ACCEPTED = "saw7500_backend_log_records_total"
BACKEND_RECEIVED = "saw7500_backend_received_log_records_total"
BACKEND_FAILED = "saw7500_backend_failures"
TALLY_ACCEPTED = "saw7500_tally_accepted_log_records"
TALLY_RECEIVED = "saw7500_tally_received_log_records"
TALLY_FAILED = "saw7500_tally_failed_log_records"
EXPORTER_SENT = "otelcol_exporter_sent_log_records"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--artifacts", required=True)
    parser.add_argument("--expect", choices=("red", "green"), required=True)
    parser.add_argument("--queue-capacity-bytes", type=float, required=True)
    parser.add_argument("--queue-budget-bytes", type=float, required=True)
    parser.add_argument("--latency-timeout-ms", type=float, default=5000)
    parser.add_argument("--green-p95-ms", type=float, default=2000)
    parser.add_argument("--green-max-over-2s-count", type=float, default=0)
    parser.add_argument("--require-red-liveness-restart", action="store_true")
    parser.add_argument("--strict-red", action="store_true")
    return parser.parse_args()


def parse_value(raw: str) -> float:
    if raw == "+Inf" or raw == "Inf":
        return math.inf
    if raw == "-Inf":
        return -math.inf
    if raw == "NaN":
        return math.nan
    return float(raw)


def parse_attrs(raw: str | None) -> dict[str, str]:
    if not raw:
        return {}
    return {match.group(1): match.group(2) for match in ATTR_RE.finditer(raw)}


def metric_matches(name: str, base: str) -> bool:
    return name == base or name == f"{base}_total"


def tick_from_file(path: str, prefix: str) -> str:
    name = os.path.basename(path)
    match = re.match(rf"{re.escape(prefix)}-(\d+)-", name)
    if match:
        return match.group(1)
    return "000000"


def pod_from_file(path: str, prefix: str) -> str:
    name = os.path.basename(path)
    match = re.match(rf"{re.escape(prefix)}-\d+-\d+-(.+)\.prom$", name)
    if match:
        return match.group(1)
    return name


def load_tick_series(directory: Path, prefix: str) -> dict[str, list[tuple[str, dict[str, str], float]]]:
    ticks: dict[str, list[tuple[str, dict[str, str], float]]] = defaultdict(list)
    for path in sorted(glob.glob(str(directory / f"{prefix}-*.prom"))):
        tick = tick_from_file(path, prefix)
        with open(path, encoding="utf-8") as handle:
            for line in handle:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                match = SAMPLE_RE.match(line)
                if not match:
                    continue
                ticks[tick].append((match.group(1), parse_attrs(match.group(2)), parse_value(match.group(3))))
    return dict(sorted(ticks.items()))


def sum_metric(samples: list[tuple[str, dict[str, str], float]], base: str) -> float:
    return sum(value for name, _, value in samples if metric_matches(name, base) and not math.isnan(value))


def sum_metric_file(path: str, base: str) -> float | None:
    total = 0.0
    found = False
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            match = SAMPLE_RE.match(line)
            if not match:
                continue
            if metric_matches(match.group(1), base):
                value = parse_value(match.group(3))
                if not math.isnan(value):
                    total += value
                    found = True
    return total if found else None


def counter_total_from_files(directory: Path, prefix: str, base: str) -> float | None:
    series: dict[str, list[tuple[int, float]]] = defaultdict(list)
    for path in sorted(glob.glob(str(directory / f"{prefix}-*.prom"))):
        value = sum_metric_file(path, base)
        if value is None:
            continue
        tick = int(tick_from_file(path, prefix))
        series[pod_from_file(path, prefix)].append((tick, value))
    if not series:
        return None

    total = 0.0
    for values in series.values():
        previous: float | None = None
        for _, value in sorted(values, key=lambda item: item[0]):
            if previous is None:
                total += value
            elif value >= previous:
                total += value - previous
            else:
                # StatefulSet pod names are reused during rolling updates. A
                # lower counter value means the replacement pod started a new
                # process and reset its counters to zero.
                total += value
            previous = value
    return total


def max_metric(samples: list[tuple[str, dict[str, str], float]], base: str) -> float:
    values = [value for name, _, value in samples if metric_matches(name, base) and not math.isnan(value)]
    return max(values) if values else 0.0


def sum_refused(samples: list[tuple[str, dict[str, str], float]]) -> float:
    return sum(
        value
        for name, _, value in samples
        if "refused" in name and "log_records" in name and not math.isnan(value)
    )


def histogram_quantile(samples: list[tuple[str, dict[str, str], float]], base_prefix: str, quantile: float) -> float | None:
    buckets = histogram_buckets(samples, base_prefix)
    return histogram_quantile_from_buckets(buckets, quantile)


def histogram_buckets(samples: list[tuple[str, dict[str, str], float]], base_prefix: str) -> dict[float, float]:
    buckets: dict[float, float] = defaultdict(float)
    for name, attrs, value in samples:
        if not name.startswith(base_prefix) or not name.endswith("_bucket"):
            continue
        le_raw = attrs.get("le")
        if le_raw is None:
            continue
        buckets[parse_value(le_raw)] += value
    return dict(buckets)


def histogram_quantile_from_buckets(buckets: dict[float, float], quantile: float) -> float | None:
    if not buckets:
        return None

    ordered = sorted(buckets.items(), key=lambda item: item[0])
    total = ordered[-1][1]
    if total <= 0:
        return None
    wanted = total * quantile
    prev_le = 0.0
    prev_count = 0.0
    for le, count in ordered:
        if count < wanted:
            prev_le = le
            prev_count = count
            continue
        if math.isinf(le):
            return prev_le
        bucket_count = count - prev_count
        if bucket_count <= 0:
            return le
        fraction = (wanted - prev_count) / bucket_count
        return prev_le + (le - prev_le) * fraction
    return ordered[-1][0]


def histogram_count_over_threshold(
    samples: list[tuple[str, dict[str, str], float]], base_prefix: str, threshold: float
) -> float | None:
    return histogram_count_over_threshold_from_buckets(histogram_buckets(samples, base_prefix), threshold)


def histogram_count_over_threshold_from_buckets(buckets: dict[float, float], threshold: float) -> float | None:
    if not buckets:
        return None

    total = buckets.get(math.inf)
    if total is None:
        total = max(buckets.values())
    below_or_at = [count for le, count in buckets.items() if le <= threshold]
    threshold_count = max(below_or_at) if below_or_at else 0.0
    return max(0.0, total - threshold_count)


def merge_histogram_buckets(bucket_sets: list[dict[float, float]]) -> dict[float, float]:
    merged: dict[float, float] = defaultdict(float)
    for buckets in bucket_sets:
        for le, value in buckets.items():
            if not math.isnan(value):
                merged[le] += value
    return dict(merged)


def histogram_total_from_buckets(buckets: dict[float, float]) -> float:
    if not buckets:
        return 0.0
    total = buckets.get(math.inf)
    if total is not None:
        return total
    return max(buckets.values())


def nonempty_histogram_buckets(bucket_sets: list[dict[float, float]]) -> list[dict[float, float]]:
    return [buckets for buckets in bucket_sets if histogram_total_from_buckets(buckets) > 0]


def latency_green_checks(
    *,
    settled_p95: float | None,
    settled_p99: float | None,
    settled_over_2s_count: float | None,
    green_p95_ms: float,
    latency_timeout_ms: float,
    green_max_over_2s_count: float,
) -> dict[str, bool]:
    return {
        "p95_under_2s": settled_p95 is not None and settled_p95 < green_p95_ms,
        "p99_not_pinned": settled_p99 is not None and settled_p99 < latency_timeout_ms * 0.95,
        "latency_over_2s_under_limit": (
            settled_over_2s_count is not None and settled_over_2s_count <= green_max_over_2s_count
        ),
    }


def histogram_delta_series(
    ticks: dict[str, list[tuple[str, dict[str, str], float]]], base_prefix: str
) -> list[dict[float, float]]:
    previous: dict[float, float] | None = None
    deltas: list[dict[float, float]] = []
    for _, samples in sorted(ticks.items()):
        current = histogram_buckets(samples, base_prefix)
        if not current:
            continue
        if previous is None:
            deltas.append(current)
            previous = current
            continue
        all_buckets = set(current) | set(previous)
        delta: dict[float, float] = {}
        for bucket in all_buckets:
            current_value = current.get(bucket, 0.0)
            previous_value = previous.get(bucket, 0.0)
            if current_value >= previous_value:
                delta[bucket] = current_value - previous_value
            else:
                # Counter reset or pod replacement. Treat current as the new
                # interval value rather than carrying the negative delta.
                delta[bucket] = current_value
        deltas.append(delta)
        previous = current
    return deltas


def settled_values(values: list[float], window: int = 3) -> list[float]:
    if len(values) <= window:
        return values
    return values[-window:]


def restart_sum(path: Path) -> int:
    if not path.exists():
        return 0
    data = json.loads(path.read_text(encoding="utf-8"))
    total = 0
    for pod in data.get("items", []):
        statuses = pod.get("status", {}).get("containerStatuses", [])
        main_statuses = [s for s in statuses if s.get("name") == "main-collector"]
        for status_obj in main_statuses or statuses:
            total += int(status_obj.get("restartCount", 0))
    return total


def liveness_kill_events(path: Path) -> int:
    if not path.exists():
        return 0
    text = path.read_text(encoding="utf-8", errors="replace").lower()
    return text.count("failed liveness probe")


def oom_killed_containers(path: Path) -> int:
    if not path.exists():
        return 0
    data = json.loads(path.read_text(encoding="utf-8"))
    total = 0
    for pod in data.get("items", []):
        statuses = pod.get("status", {}).get("containerStatuses", [])
        main_statuses = [s for s in statuses if s.get("name") == "main-collector"]
        for status_obj in main_statuses or statuses:
            reason = status_obj.get("lastState", {}).get("terminated", {}).get("reason")
            if reason == "OOMKilled":
                total += 1
    return total


def load_generated_count(path: Path) -> int | None:
    if not path.exists():
        return None
    total = 0
    found = False
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        match = re.search(r'"logs":\s*(\d+)', line)
        if not match:
            match = re.search(r"logs generated.*logs[=:](\d+)", line)
        if match:
            total += int(match.group(1))
            found = True
    return total if found else None


def rollout_completed(path: Path) -> bool:
    if not path.exists():
        return False
    text = path.read_text(encoding="utf-8", errors="replace")
    return "successfully rolled out" in text or "rolling update complete" in text


def summarize(args: argparse.Namespace) -> dict[str, object]:
    directory = Path(args.artifacts)
    lb_ticks = load_tick_series(directory, "lb-metrics")
    backend_ticks = load_tick_series(directory, "backend-metrics")
    tally_ticks = load_tick_series(directory, "tally-metrics")

    queue_bytes = [sum_metric(samples, QUEUE_BYTES) for samples in lb_ticks.values()]
    queue_capacity = [sum_metric(samples, QUEUE_CAPACITY) for samples in lb_ticks.values()]
    queue_items = [sum_metric(samples, QUEUE_ITEMS) for samples in lb_ticks.values()]
    queue_age = [max_metric(samples, QUEUE_AGE) for samples in lb_ticks.values()]
    rejected = [sum_metric(samples, REJECTED) for samples in lb_ticks.values()]
    refused = [sum_refused(samples) for samples in lb_ticks.values()]
    inflight_uncompressed = [sum_metric(samples, INFLIGHT_UNCOMPRESSED) for samples in lb_ticks.values()]
    active_consumers = [sum_metric(samples, ACTIVE_CONSUMERS) for samples in lb_ticks.values()]
    configured_consumers = [sum_metric(samples, CONFIGURED_CONSUMERS) for samples in lb_ticks.values()]
    lanes = [sum_metric(samples, LANES) for samples in lb_ticks.values()]
    tier_rss = [sum_metric(samples, PROCESS_RSS) for samples in lb_ticks.values()]
    pod_rss = [max_metric(samples, PROCESS_RSS) for samples in lb_ticks.values()]
    tier_heap = [sum_metric(samples, PROCESS_HEAP) for samples in lb_ticks.values()]
    pod_heap = [max_metric(samples, PROCESS_HEAP) for samples in lb_ticks.values()]
    tier_sys = [sum_metric(samples, PROCESS_SYS) for samples in lb_ticks.values()]
    pod_sys = [max_metric(samples, PROCESS_SYS) for samples in lb_ticks.values()]
    latency_delta_buckets = histogram_delta_series(lb_ticks, "otelcol_loadbalancer_backend_latency")
    p95s = [histogram_quantile_from_buckets(buckets, 0.95) for buckets in latency_delta_buckets]
    p99s = [histogram_quantile_from_buckets(buckets, 0.99) for buckets in latency_delta_buckets]
    over_2s_counts = [histogram_count_over_threshold_from_buckets(buckets, 2000.0) for buckets in latency_delta_buckets]
    p95_values = [value for value in p95s if value is not None]
    p99_values = [value for value in p99s if value is not None]
    over_2s_values = [value for value in over_2s_counts if value is not None]
    settled_latency_buckets = merge_histogram_buckets(settled_values(nonempty_histogram_buckets(latency_delta_buckets)))
    settled_p95 = histogram_quantile_from_buckets(settled_latency_buckets, 0.95)
    settled_p99 = histogram_quantile_from_buckets(settled_latency_buckets, 0.99)
    settled_over_2s_count = histogram_count_over_threshold_from_buckets(settled_latency_buckets, 2000.0)

    tally_accepted = counter_total_from_files(directory, "tally-metrics", TALLY_ACCEPTED)
    tally_received = counter_total_from_files(directory, "tally-metrics", TALLY_RECEIVED)
    tally_failed = counter_total_from_files(directory, "tally-metrics", TALLY_FAILED)
    backend_accepted = counter_total_from_files(directory, "backend-metrics", BACKEND_ACCEPTED)
    backend_received = counter_total_from_files(directory, "backend-metrics", BACKEND_RECEIVED)
    backend_failed = counter_total_from_files(directory, "backend-metrics", BACKEND_FAILED)
    exporter_sent = counter_total_from_files(directory, "lb-metrics", EXPORTER_SENT)
    generated = load_generated_count(directory / "loadgen.log")
    delivered_total = tally_accepted if tally_accepted is not None else backend_accepted
    received_total = tally_received if tally_received is not None else backend_received

    restart_delta = max(0, restart_sum(directory / "pods-after.json") - restart_sum(directory / "pods-before.json"))
    liveness_kills = liveness_kill_events(directory / "events-after.txt")
    oom_killed = oom_killed_containers(directory / "pods-after.json")
    baseline_age = queue_age[0] if queue_age else 0.0
    final_age = queue_age[-1] if queue_age else 0.0

    max_queue = max(queue_bytes) if queue_bytes else 0.0
    max_queue_items = max(queue_items) if queue_items else 0.0
    max_queue_age = max(queue_age) if queue_age else 0.0
    max_capacity = max(queue_capacity) if queue_capacity else 0.0
    if max_capacity <= 0:
        max_capacity = args.queue_capacity_bytes

    generated_match = False
    generated_mismatch = False
    comparison_total = delivered_total if delivered_total is not None else received_total
    if generated is not None and comparison_total is not None:
        generated_match = comparison_total >= generated * 0.98
        generated_mismatch = not generated_match

    queue_growth = max_queue > max(1024.0, (queue_bytes[0] if queue_bytes else 0.0) * 2)
    age_growth = max_queue_age > max(5000.0, baseline_age + 5000.0)

    red_signatures = {
        "queue_past_budget": max_queue >= args.queue_budget_bytes,
        "queue_grew": queue_growth,
        "oldest_age_grew": age_growth,
        "rejected_nonzero": (max(rejected) if rejected else 0.0) > 0,
        "backend_p99_pinned": bool(p99_values) and max(p99_values) >= args.latency_timeout_ms * 0.95,
        "lb_liveness_restart": restart_delta > 0,
        "lb_liveness_probe_kill_event": liveness_kills > 0,
        "lb_oom_killed": oom_killed > 0,
        "refused_nonzero": (max(refused) if refused else 0.0) > 0,
        "generated_vs_received_mismatch": generated_mismatch,
    }

    green_checks = {
        "lb_liveness_restarts_zero": restart_delta == 0,
        "queue_below_capacity": max_queue < max_capacity,
        "oldest_age_returned_to_baseline": final_age <= max(2000.0, baseline_age + 1000.0),
        "rejected_zero": (max(rejected) if rejected else 0.0) == 0,
        "refused_zero": (max(refused) if refused else 0.0) == 0,
        "generated_vs_received_match": generated_match,
        "backend_rollout_completed": rollout_completed(directory / "rollout-status.txt"),
    }
    green_checks.update(
        latency_green_checks(
            settled_p95=settled_p95,
            settled_p99=settled_p99,
            settled_over_2s_count=settled_over_2s_count,
            green_p95_ms=args.green_p95_ms,
            latency_timeout_ms=args.latency_timeout_ms,
            green_max_over_2s_count=args.green_max_over_2s_count,
        )
    )

    if args.expect == "red":
        red_required = {
            "queue_pressure": red_signatures["queue_past_budget"] or red_signatures["oldest_age_grew"],
            "rejected_nonzero": red_signatures["rejected_nonzero"],
            "backend_p99_pinned": red_signatures["backend_p99_pinned"],
            "delivery_failure": red_signatures["refused_nonzero"] or red_signatures["generated_vs_received_mismatch"],
            "metrics_present": bool(lb_ticks) and bool(backend_ticks) and bool(p99_values),
            "backend_rollout_completed": rollout_completed(directory / "rollout-status.txt"),
        }
        if args.require_red_liveness_restart:
            red_required["lb_liveness_probe_kill_event"] = red_signatures["lb_liveness_probe_kill_event"]
        red_has_failure = any(red_signatures.values())
        if args.require_red_liveness_restart:
            verdict = red_signatures["lb_liveness_probe_kill_event"] and red_has_failure
        else:
            verdict = red_has_failure
        if args.strict_red:
            verdict = all(red_required.values())
    else:
        verdict = all(green_checks.values())

    return {
        "expect": args.expect,
        "passed": verdict,
        "lb_restart_delta": restart_delta,
        "lb_liveness_kill_events": liveness_kills,
        "lb_oom_killed_containers": oom_killed,
        "max_queue_compressed_bytes": max_queue,
        "max_queue_capacity_bytes": max_capacity,
        "max_queue_items": max_queue_items,
        "baseline_oldest_age_ms": baseline_age,
        "final_oldest_age_ms": final_age,
        "max_oldest_age_ms": max_queue_age,
        "max_rejected_compressed_bytes": max(rejected) if rejected else 0.0,
        "max_refused_log_records": max(refused) if refused else 0.0,
        "max_inflight_uncompressed_bytes": max(inflight_uncompressed) if inflight_uncompressed else 0.0,
        "max_active_consumers": max(active_consumers) if active_consumers else 0.0,
        "max_configured_consumers": max(configured_consumers) if configured_consumers else 0.0,
        "max_central_queue_lanes": max(lanes) if lanes else 0.0,
        "max_tier_rss_bytes": max(tier_rss) if tier_rss else 0.0,
        "max_pod_rss_bytes": max(pod_rss) if pod_rss else 0.0,
        "max_tier_heap_alloc_bytes": max(tier_heap) if tier_heap else 0.0,
        "max_pod_heap_alloc_bytes": max(pod_heap) if pod_heap else 0.0,
        "max_tier_sys_memory_bytes": max(tier_sys) if tier_sys else 0.0,
        "max_pod_sys_memory_bytes": max(pod_sys) if pod_sys else 0.0,
        "max_backend_latency_p95_ms": max(p95_values) if p95_values else None,
        "max_backend_latency_p99_ms": max(p99_values) if p99_values else None,
        "settled_backend_latency_p95_ms": settled_p95,
        "settled_backend_latency_p99_ms": settled_p99,
        "max_backend_latency_over_2s_count": max(over_2s_values) if over_2s_values else None,
        "settled_backend_latency_over_2s_count": settled_over_2s_count,
        "loadgen_generated_logs": generated,
        "tally_received_logs": tally_received,
        "tally_accepted_logs": tally_accepted,
        "tally_failed_logs": tally_failed,
        "backend_received_logs": backend_received,
        "backend_accepted_logs": backend_accepted,
        "backend_failed_logs": backend_failed,
        "collector_sent_logs": exporter_sent,
        "delivered_logs": delivered_total,
        "received_logs": received_total,
        "tally_metrics_present": bool(tally_ticks),
        "red_signatures": red_signatures,
        "red_required": red_required if args.expect == "red" else {},
        "green_checks": green_checks,
    }


def write_outputs(directory: Path, summary: dict[str, object]) -> None:
    (directory / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    lines = [
        f"# SAW-7500 {summary['expect']} proof",
        "",
        f"verdict: {summary['passed']}",
        f"lb_restart_delta: {summary['lb_restart_delta']}",
        f"lb_liveness_kill_events: {summary['lb_liveness_kill_events']}",
        f"lb_oom_killed_containers: {summary['lb_oom_killed_containers']}",
        f"max_queue_compressed_bytes: {summary['max_queue_compressed_bytes']}",
        f"max_oldest_age_ms: {summary['max_oldest_age_ms']}",
        f"max_rejected_compressed_bytes: {summary['max_rejected_compressed_bytes']}",
        f"max_refused_log_records: {summary['max_refused_log_records']}",
        f"max_inflight_uncompressed_bytes: {summary['max_inflight_uncompressed_bytes']}",
        f"max_active_consumers: {summary['max_active_consumers']}",
        f"max_configured_consumers: {summary['max_configured_consumers']}",
        f"max_central_queue_lanes: {summary['max_central_queue_lanes']}",
        f"max_tier_rss_bytes: {summary['max_tier_rss_bytes']}",
        f"max_pod_rss_bytes: {summary['max_pod_rss_bytes']}",
        f"max_tier_heap_alloc_bytes: {summary['max_tier_heap_alloc_bytes']}",
        f"max_pod_heap_alloc_bytes: {summary['max_pod_heap_alloc_bytes']}",
        f"max_backend_latency_p95_ms: {summary['max_backend_latency_p95_ms']}",
        f"max_backend_latency_p99_ms: {summary['max_backend_latency_p99_ms']}",
        f"settled_backend_latency_p95_ms: {summary['settled_backend_latency_p95_ms']}",
        f"settled_backend_latency_p99_ms: {summary['settled_backend_latency_p99_ms']}",
        f"max_backend_latency_over_2s_count: {summary['max_backend_latency_over_2s_count']}",
        f"settled_backend_latency_over_2s_count: {summary['settled_backend_latency_over_2s_count']}",
        f"loadgen_generated_logs: {summary['loadgen_generated_logs']}",
        f"tally_received_logs: {summary['tally_received_logs']}",
        f"tally_accepted_logs: {summary['tally_accepted_logs']}",
        f"tally_failed_logs: {summary['tally_failed_logs']}",
        f"backend_received_logs: {summary['backend_received_logs']}",
        f"collector_sent_logs: {summary['collector_sent_logs']}",
        f"backend_accepted_logs: {summary['backend_accepted_logs']}",
        f"backend_failed_logs: {summary['backend_failed_logs']}",
        f"delivered_logs: {summary['delivered_logs']}",
        f"received_logs: {summary['received_logs']}",
        f"tally_metrics_present: {summary['tally_metrics_present']}",
        "",
        "## Red signatures",
    ]
    lines.extend(f"- {key}: {value}" for key, value in summary["red_signatures"].items())
    lines.append("")
    lines.append("## Red required")
    lines.extend(f"- {key}: {value}" for key, value in summary["red_required"].items())
    lines.append("")
    lines.append("## Green checks")
    lines.extend(f"- {key}: {value}" for key, value in summary["green_checks"].items())
    (directory / "proof.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    args = parse_args()
    directory = Path(args.artifacts)
    summary = summarize(args)
    write_outputs(directory, summary)
    print(json.dumps(summary, indent=2, sort_keys=True))
    return 0 if summary["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
