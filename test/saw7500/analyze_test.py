# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

from __future__ import annotations

import argparse
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import analyze  # noqa: E402


class AnalyzeTest(unittest.TestCase):
    def test_histogram_quantile_interpolates_and_handles_inf_bucket(self) -> None:
        samples = [
            ("otelcol_loadbalancer_backend_latency_bucket", {"le": "100"}, 50.0),
            ("otelcol_loadbalancer_backend_latency_bucket", {"le": "200"}, 90.0),
            ("otelcol_loadbalancer_backend_latency_bucket", {"le": "+Inf"}, 100.0),
        ]

        self.assertEqual(
            analyze.histogram_quantile(samples, "otelcol_loadbalancer_backend_latency", 0.95),
            200.0,
        )
        self.assertEqual(
            analyze.histogram_quantile(samples, "otelcol_loadbalancer_backend_latency", 0.75),
            162.5,
        )

    def test_counter_total_from_files_treats_lower_value_as_reset(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.write_prom(root / "backend-metrics-000001-13133-pod-0.prom", "records_total 10\n")
            self.write_prom(root / "backend-metrics-000002-13133-pod-0.prom", "records_total 15\n")
            self.write_prom(root / "backend-metrics-000003-13133-pod-0.prom", "records_total 3\n")

            self.assertEqual(
                analyze.counter_total_from_files(root, "backend-metrics", "records"),
                18.0,
            )

    def test_summarize_green_passes_and_red_required_stays_empty(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.write_green_artifact(root)

            summary = analyze.summarize(self.args(root, "green"))

            self.assertTrue(summary["passed"])
            self.assertEqual(summary["red_required"], {})
            self.assertTrue(summary["green_checks"]["generated_vs_received_match"])
            self.assertEqual(summary["max_queue_capacity_bytes"], 100.0)

    def test_summarize_strict_red_requires_liveness_and_failure_signatures(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.write_red_artifact(root)

            summary = analyze.summarize(
                self.args(root, "red", require_red_liveness_restart=True, strict_red=True)
            )

            self.assertTrue(summary["passed"])
            self.assertTrue(summary["red_required"]["lb_liveness_probe_kill_event"])
            self.assertTrue(summary["red_required"]["queue_pressure"])
            self.assertTrue(summary["red_required"]["delivery_failure"])

    @staticmethod
    def args(
        root: Path,
        expect: str,
        require_red_liveness_restart: bool = False,
        strict_red: bool = False,
    ) -> argparse.Namespace:
        return argparse.Namespace(
            artifacts=str(root),
            expect=expect,
            queue_capacity_bytes=100.0,
            queue_budget_bytes=100.0,
            latency_timeout_ms=5000.0,
            green_p95_ms=2000.0,
            require_red_liveness_restart=require_red_liveness_restart,
            strict_red=strict_red,
        )

    def write_green_artifact(self, root: Path) -> None:
        self.write_common_artifact(root, restarts_after=0, liveness_kill=False)
        self.write_prom(
            root / "lb-metrics-000001-8888-lb-0.prom",
            self.lb_metrics(queue=10, age=10, rejected=0, refused=0, sent=100),
        )
        self.write_prom(
            root / "backend-metrics-000001-13133-backend-0.prom",
            self.backend_metrics(100),
        )
        self.write_prom(
            root / "tally-metrics-000001-13134-tally-0.prom",
            self.tally_metrics(100, 100, 0),
        )
        self.write_loadgen(root, 100)

    def write_red_artifact(self, root: Path) -> None:
        self.write_common_artifact(root, restarts_after=1, liveness_kill=True)
        self.write_prom(
            root / "lb-metrics-000001-8888-lb-0.prom",
            self.lb_metrics(queue=250, age=7000, rejected=1, refused=5, sent=100),
        )
        self.write_prom(
            root / "backend-metrics-000001-13133-backend-0.prom",
            self.backend_metrics(100, pinned=True),
        )
        self.write_prom(
            root / "tally-metrics-000001-13134-tally-0.prom",
            self.tally_metrics(100, 20, 80),
        )
        self.write_loadgen(root, 100)

    @staticmethod
    def write_common_artifact(root: Path, restarts_after: int, liveness_kill: bool) -> None:
        pods_before = {"items": [{"status": {"containerStatuses": [{"name": "main-collector", "restartCount": 0}]}}]}
        pods_after = {
            "items": [
                {
                    "status": {
                        "containerStatuses": [
                            {
                                "name": "main-collector",
                                "restartCount": restarts_after,
                                "lastState": {"terminated": {"reason": "Completed"}},
                            }
                        ]
                    }
                }
            ]
        }
        (root / "pods-before.json").write_text(json.dumps(pods_before), encoding="utf-8")
        (root / "pods-after.json").write_text(json.dumps(pods_after), encoding="utf-8")
        (root / "events-after.txt").write_text(
            "failed liveness probe\n" if liveness_kill else "",
            encoding="utf-8",
        )
        (root / "rollout-status.txt").write_text(
            "statefulset successfully rolled out\n",
            encoding="utf-8",
        )

    @staticmethod
    def lb_metrics(queue: int, age: int, rejected: int, refused: int, sent: int) -> str:
        return f"""
otelcol_loadbalancer_central_queue_compressed_bytes {queue}
otelcol_loadbalancer_central_queue_compressed_capacity 100
otelcol_loadbalancer_central_queue_items 1
otelcol_loadbalancer_central_queue_oldest_item_age_milliseconds {age}
otelcol_loadbalancer_central_queue_rejected_compressed_bytes {rejected}
otelcol_receiver_refused_log_records {refused}
otelcol_loadbalancer_central_queue_inflight_uncompressed_bytes 10
otelcol_loadbalancer_central_queue_active_consumers 1
otelcol_loadbalancer_central_queue_configured_consumers 1
otelcol_loadbalancer_central_queue_lanes 64
otelcol_process_memory_rss_bytes 10
otelcol_process_runtime_heap_alloc_bytes 10
otelcol_process_runtime_total_sys_memory_bytes 10
otelcol_exporter_sent_log_records_total {sent}
otelcol_loadbalancer_backend_latency_bucket{{le="100"}} {0 if age > 5000 else 100}
otelcol_loadbalancer_backend_latency_bucket{{le="5000"}} 100
otelcol_loadbalancer_backend_latency_bucket{{le="+Inf"}} 100
"""

    @staticmethod
    def backend_metrics(records: int, pinned: bool = False) -> str:
        return f"""
saw7500_backend_log_records_total {records}
saw7500_backend_received_log_records_total {records}
saw7500_backend_failures_total {1 if pinned else 0}
"""

    @staticmethod
    def tally_metrics(received: int, accepted: int, failed: int) -> str:
        return f"""
saw7500_tally_received_log_records_total {received}
saw7500_tally_accepted_log_records_total {accepted}
saw7500_tally_failed_log_records_total {failed}
"""

    @staticmethod
    def write_loadgen(root: Path, count: int) -> None:
        (root / "loadgen.log").write_text(f'{{"logs": {count}}}\n', encoding="utf-8")

    @staticmethod
    def write_prom(path: Path, text: str) -> None:
        path.write_text(text.strip() + "\n", encoding="utf-8")


if __name__ == "__main__":
    unittest.main()
