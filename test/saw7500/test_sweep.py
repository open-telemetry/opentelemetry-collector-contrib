#!/usr/bin/env python3
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import sweep


def write_summary(root: Path, target: int, summary: dict[str, object]) -> None:
    directory = root / f"target-{target}" / "green"
    directory.mkdir(parents=True)
    (directory / "summary.json").write_text(json.dumps(summary), encoding="utf-8")


class SweepTest(unittest.TestCase):
    def test_load_rows_sorts_by_target_and_extracts_gates(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write_summary(
                root,
                262144,
                {
                    "passed": True,
                    "settled_backend_latency_p95_ms": 100.0,
                    "settled_backend_latency_p99_ms": 250.0,
                    "settled_backend_latency_over_2s_count": 0.0,
                    "max_tier_rss_bytes": 1024.0,
                    "max_inflight_uncompressed_bytes": 512.0,
                    "max_oldest_age_ms": 20.0,
                    "max_rejected_compressed_bytes": 0.0,
                    "max_refused_log_records": 0.0,
                    "loadgen_generated_logs": 100,
                    "delivered_logs": 100.0,
                    "lb_liveness_kill_events": 0,
                    "lb_restart_delta": 0,
                },
            )
            write_summary(
                root,
                131072,
                {
                    "passed": False,
                    "settled_backend_latency_p95_ms": 300.0,
                    "settled_backend_latency_p99_ms": 5000.0,
                    "settled_backend_latency_over_2s_count": 7.0,
                    "max_tier_rss_bytes": 2048.0,
                    "max_inflight_uncompressed_bytes": 1024.0,
                    "max_oldest_age_ms": 6000.0,
                    "max_rejected_compressed_bytes": 1.0,
                    "max_refused_log_records": 2.0,
                    "loadgen_generated_logs": 100,
                    "delivered_logs": 97.0,
                    "lb_liveness_kill_events": 1,
                    "lb_restart_delta": 1,
                },
            )

            rows = sweep.load_rows(root)

        self.assertEqual([row.target_compressed_bytes for row in rows], [131072, 262144])
        self.assertFalse(rows[0].passed)
        self.assertFalse(rows[0].generated_equals_delivered)
        self.assertTrue(rows[1].passed)
        self.assertTrue(rows[1].generated_equals_delivered)

    def test_render_markdown_table_includes_decision_metrics(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write_summary(
                root,
                1048576,
                {
                    "passed": True,
                    "settled_backend_latency_p95_ms": 100.0,
                    "settled_backend_latency_p99_ms": 200.0,
                    "settled_backend_latency_over_2s_count": 0.0,
                    "max_tier_rss_bytes": 1048576.0,
                    "max_inflight_uncompressed_bytes": 524288.0,
                    "max_oldest_age_ms": 1000.0,
                    "max_rejected_compressed_bytes": 0.0,
                    "max_refused_log_records": 0.0,
                    "loadgen_generated_logs": 10,
                    "delivered_logs": 10.0,
                    "lb_liveness_kill_events": 0,
                    "lb_restart_delta": 0,
                },
            )

            table = sweep.render_markdown(sweep.load_rows(root))

        self.assertIn("target_compressed_bytes", table)
        self.assertIn("over2s", table)
        self.assertIn("1048576", table)
        self.assertIn("1 MiB", table)
        self.assertIn("generated=delivered", table)

    def test_format_size_preserves_sub_mib_precision(self) -> None:
        self.assertEqual(sweep.format_size(128 * 1024), "128 KiB")
        self.assertEqual(sweep.format_size(256 * 1024), "256 KiB")
        self.assertEqual(sweep.format_size(512 * 1024), "512 KiB")
        self.assertEqual(sweep.format_size(1024 * 1024), "1 MiB")


if __name__ == "__main__":
    unittest.main()
