#!/usr/bin/env python3
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

"""Render request-size sweep summaries for the SAW-7500 harness."""

from __future__ import annotations

import argparse
import json
import re
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class SweepRow:
    target_compressed_bytes: int
    target_size: str
    passed: bool
    p95_ms: float | None
    p99_ms: float | None
    over_2s_count: float | None
    rss_mib: float
    inflight_mib: float
    max_oldest_age_ms: float
    rejected_bytes: float
    refused_records: float
    generated: int | None
    delivered: float | None
    generated_equals_delivered: bool
    liveness_kills: int
    restarts: int
    summary_path: str


TARGET_RE = re.compile(r"target-(\d+)$")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--artifacts", required=True)
    parser.add_argument("--output", default="")
    parser.add_argument("--json-output", default="")
    return parser.parse_args()


def load_rows(root: Path) -> list[SweepRow]:
    rows: list[SweepRow] = []
    for path in sorted(root.glob("target-*/green/summary.json")):
        match = TARGET_RE.search(path.parent.parent.name)
        if match is None:
            continue
        target = int(match.group(1))
        summary = json.loads(path.read_text(encoding="utf-8"))
        rows.append(row_from_summary(target, summary, path))
    return sorted(rows, key=lambda row: row.target_compressed_bytes)


def row_from_summary(target: int, summary: dict[str, Any], path: Path) -> SweepRow:
    generated = optional_int(summary.get("loadgen_generated_logs"))
    delivered = optional_float(summary.get("delivered_logs"))
    generated_equals_delivered = False
    if generated is not None and delivered is not None:
        generated_equals_delivered = delivered >= generated * 0.98

    return SweepRow(
        target_compressed_bytes=target,
        target_size=format_size(target),
        passed=bool(summary.get("passed")),
        p95_ms=optional_float(summary.get("settled_backend_latency_p95_ms")),
        p99_ms=optional_float(summary.get("settled_backend_latency_p99_ms")),
        over_2s_count=optional_float(
            summary.get("settled_backend_latency_over_2s_count", summary.get("max_backend_latency_over_2s_count"))
        ),
        rss_mib=bytes_to_mib(summary.get("max_tier_rss_bytes")),
        inflight_mib=bytes_to_mib(summary.get("max_inflight_uncompressed_bytes")),
        max_oldest_age_ms=float(summary.get("max_oldest_age_ms") or 0.0),
        rejected_bytes=float(summary.get("max_rejected_compressed_bytes") or 0.0),
        refused_records=float(summary.get("max_refused_log_records") or 0.0),
        generated=generated,
        delivered=delivered,
        generated_equals_delivered=generated_equals_delivered,
        liveness_kills=int(summary.get("lb_liveness_kill_events") or 0),
        restarts=int(summary.get("lb_restart_delta") or 0),
        summary_path=str(path),
    )


def optional_float(value: object) -> float | None:
    if value is None:
        return None
    return float(value)


def optional_int(value: object) -> int | None:
    if value is None:
        return None
    return int(float(value))


def bytes_to_mib(value: object) -> float:
    return float(value or 0.0) / 1024.0 / 1024.0


def format_size(value: int) -> str:
    kib = 1024
    mib = kib * 1024
    if value < mib and value % kib == 0:
        return f"{value // kib} KiB"
    if value % mib == 0:
        return f"{value // mib} MiB"
    return f"{value / mib:.2f} MiB"


def format_ms(value: float | None) -> str:
    if value is None:
        return "n/a"
    return f"{value:.0f}"


def render_markdown(rows: list[SweepRow]) -> str:
    lines = [
        "# SAW-7500 Request-Size Sweep",
        "",
        "| target_compressed_bytes | size | pass | p95 ms | p99 ms | over2s | RSS MiB | inflight MiB | age ms | rejected | refused | generated=delivered | live kills | restarts |",
        "|---:|---:|:---:|---:|---:|---:|---:|---:|---:|---:|---:|:---:|---:|---:|",
    ]
    for row in rows:
        lines.append(
            "| "
            f"{row.target_compressed_bytes} | "
            f"{row.target_size} | "
            f"{'yes' if row.passed else 'no'} | "
            f"{format_ms(row.p95_ms)} | "
            f"{format_ms(row.p99_ms)} | "
            f"{format_ms(row.over_2s_count)} | "
            f"{row.rss_mib:.1f} | "
            f"{row.inflight_mib:.1f} | "
            f"{row.max_oldest_age_ms:.0f} | "
            f"{row.rejected_bytes:.0f} | "
            f"{row.refused_records:.0f} | "
            f"{'yes' if row.generated_equals_delivered else 'no'} | "
            f"{row.liveness_kills} | "
            f"{row.restarts} |"
        )
    lines.extend(
        [
            "",
            "Decision rule: keep 256 KiB unless another target passes every gate and materially improves an operator-relevant metric without increasing memory, queue age, refusals/rejections, or liveness risk.",
        ]
    )
    return "\n".join(lines) + "\n"


def main() -> int:
    args = parse_args()
    rows = load_rows(Path(args.artifacts))
    rendered = render_markdown(rows)
    if args.output:
        Path(args.output).write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")
    if args.json_output:
        Path(args.json_output).write_text(
            json.dumps([asdict(row) for row in rows], indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
    return 0 if rows else 1


if __name__ == "__main__":
    raise SystemExit(main())
