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

from opentelemetry import metrics
from opentelemetry.exporter.prometheus_remote_write import (
    PrometheusRemoteWriteMetricsExporter,
)
from opentelemetry.test.test_base import TestBase


def observer_callback(observer):
    array = [1.0, 15.0, 25.0, 26.0]
    for (index, usage) in enumerate(array):
        labels = {"test_label": str(index)}
        observer.observe(usage, labels)


class TestPrometheusRemoteWriteExporterCortex(TestBase):
    def setUp(self):
        super().setUp
        self.exporter = PrometheusRemoteWriteMetricsExporter(
            endpoint="http://localhost:9009/api/prom/push",
            headers={"X-Scope-Org-ID": "5"},
        )
        self.labels = {"environment": "testing"}
        self.meter = self.meter_provider.get_meter(__name__)
        metrics.get_meter_provider().start_pipeline(
            self.meter, self.exporter, 1,
        )

    def test_export_counter(self):
        try:
            requests_counter = self.meter.create_counter(
                name="counter",
                description="test_export_counter",
                unit="1",
                value_type=int,
            )
            requests_counter.add(25, self.labels)
        except Exception as e:
            self.fail(
                "Export counter failed with unexpected error {}".format(e)
            )

    def test_export_valuerecorder(self):
        try:
            requests_size = self.meter.create_valuerecorder(
                name="valuerecorder",
                description="test_export_valuerecorder",
                unit="1",
                value_type=int,
            )
            requests_size.record(25, self.labels)
        except Exception as e:
            self.fail(
                "Export valuerecorder failed with unexpected error {}".format(
                    e
                )
            )

    def test_export_updowncounter(self):
        try:
            requests_size = self.meter.create_updowncounter(
                name="updowncounter",
                description="test_export_updowncounter",
                unit="1",
                value_type=int,
            )
            requests_size.add(-25, self.labels)
        except Exception as e:
            self.fail(
                "Export updowncounter failed with unexpected error {}".format(
                    e
                )
            )

    def test_export_sumobserver(self):
        try:
            self.meter.register_sumobserver(
                callback=observer_callback,
                name="sumobserver",
                description="test_export_sumobserver",
                unit="1",
                value_type=float,
            )
        except Exception as e:
            self.fail(
                "Export sumobserver failed with unexpected error {}".format(e)
            )

    def test_export_updownsumobserver(self):
        try:
            self.meter.register_updownsumobserver(
                callback=observer_callback,
                name="updownsumobserver",
                description="test_export_updownsumobserver",
                unit="1",
                value_type=float,
            )
        except Exception as e:
            self.fail(
                "Export updownsumobserver failed with unexpected error {}".format(
                    e
                )
            )
