#!/usr/bin/env python3
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


from __future__ import annotations

import unittest

import analyze


class AnalyzeTest(unittest.TestCase):
    def test_histogram_count_over_threshold_uses_total_minus_threshold_bucket(self) -> None:
        samples = [
            ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "1000"}, 3.0),
            ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "2000"}, 5.0),
            ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "+Inf"}, 8.0),
        ]

        self.assertEqual(
            analyze.histogram_count_over_threshold(
                samples,
                "otelcol_loadbalancer_backend_latency",
                2000.0,
            ),
            3.0,
        )

    def test_histogram_count_over_threshold_uses_largest_bucket_below_threshold(self) -> None:
        samples = [
            ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "1000"}, 2.0),
            ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "5000"}, 7.0),
            ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "+Inf"}, 10.0),
        ]

        self.assertEqual(
            analyze.histogram_count_over_threshold(
                samples,
                "otelcol_loadbalancer_backend_latency",
                2000.0,
            ),
            8.0,
        )

    def test_histogram_delta_series_removes_prior_timeout_from_settled_window(self) -> None:
        ticks = {
            "000001": [
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "100"}, 90.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "2000"}, 90.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "+Inf"}, 100.0),
            ],
            "000002": [
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "100"}, 190.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "2000"}, 190.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "+Inf"}, 200.0),
            ],
            "000003": [
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "100"}, 290.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "2000"}, 290.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "+Inf"}, 300.0),
            ],
            "000004": [
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "100"}, 390.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "2000"}, 390.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "+Inf"}, 400.0),
            ],
            "000005": [
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "100"}, 490.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "2000"}, 490.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "+Inf"}, 500.0),
            ],
            "000006": [
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "100"}, 590.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "2000"}, 590.0),
                ("otelcol_loadbalancer_backend_latency_milliseconds_bucket", {"le": "+Inf"}, 600.0),
            ],
        }

        deltas = analyze.histogram_delta_series(ticks, "otelcol_loadbalancer_backend_latency")

        self.assertEqual(len(deltas), 6)
        self.assertEqual(analyze.histogram_count_over_threshold_from_buckets(deltas[-1], 2000.0), 0.0)
        self.assertLess(analyze.histogram_quantile_from_buckets(deltas[-1], 0.95), 2000.0)

    def test_merge_buckets_uses_full_settled_window_for_quantiles(self) -> None:
        buckets = analyze.merge_histogram_buckets(
            [
                {100.0: 100.0, 2000.0: 100.0, float("inf"): 100.0},
                {100.0: 100.0, 2000.0: 100.0, float("inf"): 101.0},
                {100.0: 100.0, 2000.0: 100.0, float("inf"): 100.0},
            ],
        )

        self.assertEqual(analyze.histogram_count_over_threshold_from_buckets(buckets, 2000.0), 1.0)
        self.assertLess(analyze.histogram_quantile_from_buckets(buckets, 0.99), 2000.0)

    def test_nonempty_buckets_excludes_zero_sample_settle_ticks(self) -> None:
        buckets = analyze.nonempty_histogram_buckets(
            [
                {100.0: 0.0, 2000.0: 0.0, float("inf"): 0.0},
                {100.0: 100.0, 2000.0: 100.0, float("inf"): 100.0},
                {100.0: 0.0, 2000.0: 0.0, float("inf"): 0.0},
            ],
        )

        self.assertEqual(buckets, [{100.0: 100.0, 2000.0: 100.0, float("inf"): 100.0}])

    def test_latency_green_checks_require_no_settled_over_2s_by_default(self) -> None:
        checks = analyze.latency_green_checks(
            settled_p95=50.0,
            settled_p99=100.0,
            settled_over_2s_count=1.0,
            green_p95_ms=2000.0,
            latency_timeout_ms=5000.0,
            green_max_over_2s_count=0.0,
        )

        self.assertFalse(checks["latency_over_2s_under_limit"])


if __name__ == "__main__":
    unittest.main()
