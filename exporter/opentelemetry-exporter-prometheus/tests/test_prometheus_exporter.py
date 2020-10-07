# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import unittest
from unittest import mock

from prometheus_client import generate_latest
from prometheus_client.core import CounterMetricFamily

from opentelemetry.exporter.prometheus import (
    CustomCollector,
    PrometheusMetricsExporter,
)
from opentelemetry.metrics import get_meter_provider, set_meter_provider
from opentelemetry.sdk import metrics
from opentelemetry.sdk.metrics.export import MetricRecord, MetricsExportResult
from opentelemetry.sdk.metrics.export.aggregate import (
    MinMaxSumCountAggregator,
    SumAggregator,
)
from opentelemetry.sdk.util import get_dict_as_key


class TestPrometheusMetricExporter(unittest.TestCase):
    def setUp(self):
        set_meter_provider(metrics.MeterProvider())
        self._meter = get_meter_provider().get_meter(__name__)
        self._test_metric = self._meter.create_metric(
            "testname", "testdesc", "unit", int, metrics.Counter,
        )
        labels = {"environment": "staging"}
        self._labels_key = get_dict_as_key(labels)

        self._mock_registry_register = mock.Mock()
        self._registry_register_patch = mock.patch(
            "prometheus_client.core.REGISTRY.register",
            side_effect=self._mock_registry_register,
        )

    # pylint: disable=protected-access
    def test_constructor(self):
        """Test the constructor."""
        with self._registry_register_patch:
            exporter = PrometheusMetricsExporter("testprefix")
            self.assertEqual(exporter._collector._prefix, "testprefix")
            self.assertTrue(self._mock_registry_register.called)

    def test_shutdown(self):
        with mock.patch(
            "prometheus_client.core.REGISTRY.unregister"
        ) as registry_unregister_patch:
            exporter = PrometheusMetricsExporter()
            exporter.shutdown()
            self.assertTrue(registry_unregister_patch.called)

    def test_export(self):
        with self._registry_register_patch:
            record = MetricRecord(
                self._test_metric,
                self._labels_key,
                SumAggregator(),
                get_meter_provider().resource,
            )
            exporter = PrometheusMetricsExporter()
            result = exporter.export([record])
            # pylint: disable=protected-access
            self.assertEqual(len(exporter._collector._metrics_to_export), 1)
            self.assertIs(result, MetricsExportResult.SUCCESS)

    def test_min_max_sum_aggregator_to_prometheus(self):
        meter = get_meter_provider().get_meter(__name__)
        metric = meter.create_metric(
            "test@name", "testdesc", "unit", int, metrics.ValueRecorder, []
        )
        labels = {}
        key_labels = get_dict_as_key(labels)
        aggregator = MinMaxSumCountAggregator()
        aggregator.update(123)
        aggregator.update(456)
        aggregator.take_checkpoint()
        record = MetricRecord(
            metric, key_labels, aggregator, get_meter_provider().resource
        )
        collector = CustomCollector("testprefix")
        collector.add_metrics_data([record])
        result_bytes = generate_latest(collector)
        result = result_bytes.decode("utf-8")
        self.assertIn("testprefix_test_name_count 2.0", result)
        self.assertIn("testprefix_test_name_sum 579.0", result)

    def test_counter_to_prometheus(self):
        meter = get_meter_provider().get_meter(__name__)
        metric = meter.create_metric(
            "test@name", "testdesc", "unit", int, metrics.Counter,
        )
        labels = {"environment@": "staging", "os": "Windows"}
        key_labels = get_dict_as_key(labels)
        aggregator = SumAggregator()
        aggregator.update(123)
        aggregator.take_checkpoint()
        record = MetricRecord(
            metric, key_labels, aggregator, get_meter_provider().resource
        )
        collector = CustomCollector("testprefix")
        collector.add_metrics_data([record])

        for prometheus_metric in collector.collect():
            self.assertEqual(type(prometheus_metric), CounterMetricFamily)
            self.assertEqual(prometheus_metric.name, "testprefix_test_name")
            self.assertEqual(prometheus_metric.documentation, "testdesc")
            self.assertTrue(len(prometheus_metric.samples) == 1)
            self.assertEqual(prometheus_metric.samples[0].value, 123)
            self.assertTrue(len(prometheus_metric.samples[0].labels) == 2)
            self.assertEqual(
                prometheus_metric.samples[0].labels["environment_"], "staging"
            )
            self.assertEqual(
                prometheus_metric.samples[0].labels["os"], "Windows"
            )

    # TODO: Add unit test once GaugeAggregator is available
    # TODO: Add unit test once Measure Aggregators are available

    def test_invalid_metric(self):
        meter = get_meter_provider().get_meter(__name__)
        metric = meter.create_metric(
            "tesname", "testdesc", "unit", int, StubMetric
        )
        labels = {"environment": "staging"}
        key_labels = get_dict_as_key(labels)
        record = MetricRecord(
            metric, key_labels, None, get_meter_provider().resource
        )
        collector = CustomCollector("testprefix")
        collector.add_metrics_data([record])
        collector.collect()
        self.assertLogs("opentelemetry.exporter.prometheus", level="WARNING")

    def test_sanitize(self):
        collector = CustomCollector("testprefix")
        self.assertEqual(
            collector._sanitize("1!2@3#4$5%6^7&8*9(0)_-"),
            "1_2_3_4_5_6_7_8_9_0___",
        )
        self.assertEqual(collector._sanitize(",./?;:[]{}"), "__________")
        self.assertEqual(collector._sanitize("TestString"), "TestString")
        self.assertEqual(collector._sanitize("aAbBcC_12_oi"), "aAbBcC_12_oi")


class StubMetric(metrics.Metric):
    def __init__(
        self,
        name: str,
        description: str,
        unit: str,
        value_type,
        meter,
        enabled: bool = True,
    ):
        super().__init__(
            name, description, unit, value_type, meter, enabled=enabled,
        )
