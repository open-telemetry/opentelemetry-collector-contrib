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

"""
This library allows export of metrics data to `Prometheus <https://prometheus.io/>`_.

Usage
-----

The **OpenTelemetry Prometheus Exporter** allows export of `OpenTelemetry`_ metrics to `Prometheus`_.


.. _Prometheus: https://prometheus.io/
.. _OpenTelemetry: https://github.com/open-telemetry/opentelemetry-python/

.. code:: python

    from opentelemetry import metrics
    from opentelemetry.exporter.prometheus import PrometheusMetricsExporter
    from opentelemetry.sdk.metrics import Counter, Meter
    from prometheus_client import start_http_server

    # Start Prometheus client
    start_http_server(port=8000, addr="localhost")

    # Meter is responsible for creating and recording metrics
    metrics.set_meter_provider(MeterProvider())
    meter = metrics.get_meter(__name__)
    # exporter to export metrics to Prometheus
    prefix = "MyAppPrefix"
    exporter = PrometheusMetricsExporter(prefix)
    # Starts the collect/export pipeline for metrics
    metrics.get_meter_provider().start_pipeline(meter, exporter, 5)

    counter = meter.create_metric(
        "requests",
        "number of requests",
        "requests",
        int,
        Counter,
        ("environment",),
    )

    # Labels are used to identify key-values that are associated with a specific
    # metric that you want to record. These are useful for pre-aggregation and can
    # be used to store custom dimensions pertaining to a metric
    labels = {"environment": "staging"}

    counter.add(25, labels)
    input("Press any key to exit...")

API
---
"""

import collections
import logging
import re
from typing import Iterable, Optional, Sequence, Union

from prometheus_client.core import (
    REGISTRY,
    CounterMetricFamily,
    SummaryMetricFamily,
    UnknownMetricFamily,
)

from opentelemetry.metrics import Counter, ValueRecorder
from opentelemetry.sdk.metrics.export import (
    MetricRecord,
    MetricsExporter,
    MetricsExportResult,
)
from opentelemetry.sdk.metrics.export.aggregate import MinMaxSumCountAggregator

logger = logging.getLogger(__name__)


class PrometheusMetricsExporter(MetricsExporter):
    """Prometheus metric exporter for OpenTelemetry.

    Args:
        prefix: single-word application prefix relevant to the domain
            the metric belongs to.
    """

    def __init__(self, prefix: str = ""):
        self._collector = CustomCollector(prefix)
        REGISTRY.register(self._collector)

    def export(
        self, metric_records: Sequence[MetricRecord]
    ) -> MetricsExportResult:
        self._collector.add_metrics_data(metric_records)
        return MetricsExportResult.SUCCESS

    def shutdown(self) -> None:
        REGISTRY.unregister(self._collector)


class CustomCollector:
    """CustomCollector represents the Prometheus Collector object
    https://github.com/prometheus/client_python#custom-collectors
    """

    def __init__(self, prefix: str = ""):
        self._prefix = prefix
        self._metrics_to_export = collections.deque()
        self._non_letters_nor_digits_re = re.compile(
            r"[^\w]", re.UNICODE | re.IGNORECASE
        )

    def add_metrics_data(self, metric_records: Sequence[MetricRecord]) -> None:
        self._metrics_to_export.append(metric_records)

    def collect(self):
        """Collect fetches the metrics from OpenTelemetry
        and delivers them as Prometheus Metrics.
        Collect is invoked every time a prometheus.Gatherer is run
        for example when the HTTP endpoint is invoked by Prometheus.
        """

        while self._metrics_to_export:
            for metric_record in self._metrics_to_export.popleft():
                prometheus_metric = self._translate_to_prometheus(
                    metric_record
                )
                if prometheus_metric is not None:
                    yield prometheus_metric

    def _translate_to_prometheus(self, metric_record: MetricRecord):
        prometheus_metric = None
        label_values = []
        label_keys = []
        for label_tuple in metric_record.labels:
            label_keys.append(self._sanitize(label_tuple[0]))
            label_values.append(label_tuple[1])

        metric_name = ""
        if self._prefix != "":
            metric_name = self._prefix + "_"
        metric_name += self._sanitize(metric_record.instrument.name)

        description = getattr(metric_record.instrument, "description", "")
        if isinstance(metric_record.instrument, Counter):
            prometheus_metric = CounterMetricFamily(
                name=metric_name, documentation=description, labels=label_keys
            )
            prometheus_metric.add_metric(
                labels=label_values, value=metric_record.aggregator.checkpoint
            )
        # TODO: Add support for histograms when supported in OT
        elif isinstance(metric_record.instrument, ValueRecorder):
            value = metric_record.aggregator.checkpoint
            if isinstance(metric_record.aggregator, MinMaxSumCountAggregator):
                prometheus_metric = SummaryMetricFamily(
                    name=metric_name,
                    documentation=description,
                    labels=label_keys,
                )
                prometheus_metric.add_metric(
                    labels=label_values,
                    count_value=value.count,
                    sum_value=value.sum,
                )
            else:
                prometheus_metric = UnknownMetricFamily(
                    name=metric_name,
                    documentation=description,
                    labels=label_keys,
                )
                prometheus_metric.add_metric(labels=label_values, value=value)

        else:
            logger.warning(
                "Unsupported metric type. %s", type(metric_record.instrument)
            )
        return prometheus_metric

    def _sanitize(self, key: str) -> str:
        """sanitize the given metric name or label according to Prometheus rule.
        Replace all characters other than [A-Za-z0-9_] with '_'.
        """
        return self._non_letters_nor_digits_re.sub("_", key)
