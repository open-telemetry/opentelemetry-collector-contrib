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
from logging import Logger
from unittest.mock import MagicMock, Mock, patch

from opentelemetry.exporter.prometheus_remote_write import (
    PrometheusRemoteWriteMetricsExporter,
)
from opentelemetry.exporter.prometheus_remote_write.gen.types_pb2 import (
    Label,
    Sample,
    TimeSeries,
)
from opentelemetry.sdk.metrics import Counter
from opentelemetry.sdk.metrics.export import ExportRecord, MetricsExportResult
from opentelemetry.sdk.metrics.export.aggregate import (
    HistogramAggregator,
    LastValueAggregator,
    MinMaxSumCountAggregator,
    SumAggregator,
    ValueObserverAggregator,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.util import get_dict_as_key


class TestValidation(unittest.TestCase):
    # Test cases to ensure exporter parameter validation works as intended
    def test_valid_standard_param(self):
        exporter = PrometheusRemoteWriteMetricsExporter(
            endpoint="/prom/test_endpoint",
        )
        self.assertEqual(exporter.endpoint, "/prom/test_endpoint")

    def test_valid_basic_auth_param(self):
        exporter = PrometheusRemoteWriteMetricsExporter(
            endpoint="/prom/test_endpoint",
            basic_auth={
                "username": "test_username",
                "password": "test_password",
            },
        )
        self.assertEqual(exporter.basic_auth["username"], "test_username")
        self.assertEqual(exporter.basic_auth["password"], "test_password")

    def test_invalid_no_endpoint_param(self):
        with self.assertRaises(ValueError):
            PrometheusRemoteWriteMetricsExporter("")

    def test_invalid_no_username_param(self):
        with self.assertRaises(ValueError):
            PrometheusRemoteWriteMetricsExporter(
                endpoint="/prom/test_endpoint",
                basic_auth={"password": "test_password"},
            )

    def test_invalid_no_password_param(self):
        with self.assertRaises(ValueError):
            PrometheusRemoteWriteMetricsExporter(
                endpoint="/prom/test_endpoint",
                basic_auth={"username": "test_username"},
            )

    def test_invalid_conflicting_passwords_param(self):
        with self.assertRaises(ValueError):
            PrometheusRemoteWriteMetricsExporter(
                endpoint="/prom/test_endpoint",
                basic_auth={
                    "username": "test_username",
                    "password": "test_password",
                    "password_file": "test_file",
                },
            )

    def test_invalid_timeout_param(self):
        with self.assertRaises(ValueError):
            PrometheusRemoteWriteMetricsExporter(
                endpoint="/prom/test_endpoint", timeout=0
            )

    def test_valid_tls_config_param(self):
        tls_config = {
            "ca_file": "test_ca_file",
            "cert_file": "test_cert_file",
            "key_file": "test_key_file",
            "insecure_skip_verify": True,
        }
        exporter = PrometheusRemoteWriteMetricsExporter(
            endpoint="/prom/test_endpoint", tls_config=tls_config
        )
        self.assertEqual(exporter.tls_config["ca_file"], tls_config["ca_file"])
        self.assertEqual(
            exporter.tls_config["cert_file"], tls_config["cert_file"]
        )
        self.assertEqual(
            exporter.tls_config["key_file"], tls_config["key_file"]
        )
        self.assertEqual(
            exporter.tls_config["insecure_skip_verify"],
            tls_config["insecure_skip_verify"],
        )

    # if cert_file is provided, then key_file must also be provided
    def test_invalid_tls_config_cert_only_param(self):
        tls_config = {"cert_file": "value"}
        with self.assertRaises(ValueError):
            PrometheusRemoteWriteMetricsExporter(
                endpoint="/prom/test_endpoint", tls_config=tls_config
            )

    # if cert_file is provided, then key_file must also be provided
    def test_invalid_tls_config_key_only_param(self):
        tls_config = {"cert_file": "value"}
        with self.assertRaises(ValueError):
            PrometheusRemoteWriteMetricsExporter(
                endpoint="/prom/test_endpoint", tls_config=tls_config
            )


class TestConversion(unittest.TestCase):
    # Initializes test data that is reused across tests
    def setUp(self):
        self.exporter = PrometheusRemoteWriteMetricsExporter(
            endpoint="/prom/test_endpoint"
        )

    # Ensures conversion to timeseries function works with valid aggregation types
    def test_valid_convert_to_timeseries(self):
        test_records = [
            ExportRecord(
                Counter("testname", "testdesc", "testunit", int, None),
                None,
                SumAggregator(),
                Resource({}),
            ),
            ExportRecord(
                Counter("testname", "testdesc", "testunit", int, None),
                None,
                MinMaxSumCountAggregator(),
                Resource({}),
            ),
            ExportRecord(
                Counter("testname", "testdesc", "testunit", int, None),
                None,
                HistogramAggregator(),
                Resource({}),
            ),
            ExportRecord(
                Counter("testname", "testdesc", "testunit", int, None),
                None,
                LastValueAggregator(),
                Resource({}),
            ),
            ExportRecord(
                Counter("testname", "testdesc", "testunit", int, None),
                None,
                ValueObserverAggregator(),
                Resource({}),
            ),
        ]
        for record in test_records:
            record.aggregator.update(5)
            record.aggregator.take_checkpoint()
        data = self.exporter._convert_to_timeseries(test_records)
        self.assertIsInstance(data, list)
        self.assertEqual(len(data), 13)
        for timeseries in data:
            self.assertIsInstance(timeseries, TimeSeries)

    # Ensures conversion to timeseries fails for unsupported aggregation types
    def test_invalid_convert_to_timeseries(self):
        data = self.exporter._convert_to_timeseries(
            [ExportRecord(None, None, None, Resource({}))]
        )
        self.assertIsInstance(data, list)
        self.assertEqual(len(data), 0)

    # Ensures sum aggregator is correctly converted to timeseries
    def test_convert_from_sum(self):
        sum_record = ExportRecord(
            Counter("testname", "testdesc", "testunit", int, None),
            None,
            SumAggregator(),
            Resource({}),
        )
        sum_record.aggregator.update(3)
        sum_record.aggregator.update(2)
        sum_record.aggregator.take_checkpoint()

        expected_timeseries = self.exporter._create_timeseries(
            sum_record, "testname_sum", 5.0
        )
        timeseries = self.exporter._convert_from_sum(sum_record)
        self.assertEqual(timeseries[0], expected_timeseries)

    # Ensures sum min_max_count aggregator is correctly converted to timeseries
    def test_convert_from_min_max_sum_count(self):
        min_max_sum_count_record = ExportRecord(
            Counter("testname", "testdesc", "testunit", int, None),
            None,
            MinMaxSumCountAggregator(),
            Resource({}),
        )
        min_max_sum_count_record.aggregator.update(5)
        min_max_sum_count_record.aggregator.update(1)
        min_max_sum_count_record.aggregator.take_checkpoint()

        expected_min_timeseries = self.exporter._create_timeseries(
            min_max_sum_count_record, "testname_min", 1.0
        )
        expected_max_timeseries = self.exporter._create_timeseries(
            min_max_sum_count_record, "testname_max", 5.0
        )
        expected_sum_timeseries = self.exporter._create_timeseries(
            min_max_sum_count_record, "testname_sum", 6.0
        )
        expected_count_timeseries = self.exporter._create_timeseries(
            min_max_sum_count_record, "testname_count", 2.0
        )

        timeseries = self.exporter._convert_from_min_max_sum_count(
            min_max_sum_count_record
        )
        self.assertEqual(timeseries[0], expected_min_timeseries)
        self.assertEqual(timeseries[1], expected_max_timeseries)
        self.assertEqual(timeseries[2], expected_sum_timeseries)
        self.assertEqual(timeseries[3], expected_count_timeseries)

    # Ensures histogram aggregator is correctly converted to timeseries
    def test_convert_from_histogram(self):
        histogram_record = ExportRecord(
            Counter("testname", "testdesc", "testunit", int, None),
            None,
            HistogramAggregator(),
            Resource({}),
        )
        histogram_record.aggregator.update(5)
        histogram_record.aggregator.update(2)
        histogram_record.aggregator.update(-1)
        histogram_record.aggregator.take_checkpoint()

        expected_le_0_timeseries = self.exporter._create_timeseries(
            histogram_record, "testname_histogram", 1.0, ("le", "0")
        )
        expected_le_inf_timeseries = self.exporter._create_timeseries(
            histogram_record, "testname_histogram", 2.0, ("le", "+Inf")
        )
        timeseries = self.exporter._convert_from_histogram(histogram_record)
        self.assertEqual(timeseries[0], expected_le_0_timeseries)
        self.assertEqual(timeseries[1], expected_le_inf_timeseries)

    # Ensures last value aggregator is correctly converted to timeseries
    def test_convert_from_last_value(self):
        last_value_record = ExportRecord(
            Counter("testname", "testdesc", "testunit", int, None),
            None,
            LastValueAggregator(),
            Resource({}),
        )
        last_value_record.aggregator.update(1)
        last_value_record.aggregator.update(5)
        last_value_record.aggregator.take_checkpoint()

        expected_timeseries = self.exporter._create_timeseries(
            last_value_record, "testname_last", 5.0
        )
        timeseries = self.exporter._convert_from_last_value(last_value_record)
        self.assertEqual(timeseries[0], expected_timeseries)

    # Ensures value observer aggregator is correctly converted to timeseries
    def test_convert_from_value_observer(self):
        value_observer_record = ExportRecord(
            Counter("testname", "testdesc", "testunit", int, None),
            None,
            ValueObserverAggregator(),
            Resource({}),
        )
        value_observer_record.aggregator.update(5)
        value_observer_record.aggregator.update(1)
        value_observer_record.aggregator.update(2)
        value_observer_record.aggregator.take_checkpoint()

        expected_min_timeseries = self.exporter._create_timeseries(
            value_observer_record, "testname_min", 1.0
        )
        expected_max_timeseries = self.exporter._create_timeseries(
            value_observer_record, "testname_max", 5.0
        )
        expected_sum_timeseries = self.exporter._create_timeseries(
            value_observer_record, "testname_sum", 8.0
        )
        expected_count_timeseries = self.exporter._create_timeseries(
            value_observer_record, "testname_count", 3.0
        )
        expected_last_timeseries = self.exporter._create_timeseries(
            value_observer_record, "testname_last", 2.0
        )
        timeseries = self.exporter._convert_from_value_observer(
            value_observer_record
        )
        self.assertEqual(timeseries[0], expected_min_timeseries)
        self.assertEqual(timeseries[1], expected_max_timeseries)
        self.assertEqual(timeseries[2], expected_sum_timeseries)
        self.assertEqual(timeseries[3], expected_count_timeseries)
        self.assertEqual(timeseries[4], expected_last_timeseries)

    # Ensures quantile aggregator is correctly converted to timeseries
    # TODO: Add test_convert_from_quantile once method is implemented

    # Ensures timeseries produced contains appropriate sample and labels
    def test_create_timeseries(self):
        def create_label(name, value):
            label = Label()
            label.name = name
            label.value = value
            return label

        sum_aggregator = SumAggregator()
        sum_aggregator.update(5)
        sum_aggregator.take_checkpoint()
        export_record = ExportRecord(
            Counter("testname", "testdesc", "testunit", int, None),
            get_dict_as_key({"record_name": "record_value"}),
            sum_aggregator,
            Resource({"resource_name": "resource_value"}),
        )

        expected_timeseries = TimeSeries()
        expected_timeseries.labels.append(create_label("__name__", "testname"))
        expected_timeseries.labels.append(
            create_label("resource_name", "resource_value")
        )
        expected_timeseries.labels.append(
            create_label("record_name", "record_value")
        )

        sample = expected_timeseries.samples.add()
        sample.timestamp = int(sum_aggregator.last_update_timestamp / 1000000)
        sample.value = 5.0

        timeseries = self.exporter._create_timeseries(
            export_record, "testname", 5.0
        )
        self.assertEqual(timeseries, expected_timeseries)


class TestExport(unittest.TestCase):
    # Initializes test data that is reused across tests
    def setUp(self):
        self.exporter = PrometheusRemoteWriteMetricsExporter(
            endpoint="/prom/test_endpoint"
        )

    # Ensures export is successful with valid export_records and config
    @patch("requests.post")
    def test_valid_export(self, mock_post):
        mock_post.return_value.configure_mock(**{"status_code": 200})
        test_metric = Counter("testname", "testdesc", "testunit", int, None)
        labels = get_dict_as_key({"environment": "testing"})
        record = ExportRecord(
            test_metric, labels, SumAggregator(), Resource({})
        )
        result = self.exporter.export([record])
        self.assertIs(result, MetricsExportResult.SUCCESS)
        self.assertEqual(mock_post.call_count, 1)

        result = self.exporter.export([])
        self.assertIs(result, MetricsExportResult.SUCCESS)

    def test_invalid_export(self):
        record = ExportRecord(None, None, None, None)
        result = self.exporter.export([record])
        self.assertIs(result, MetricsExportResult.FAILURE)

    @patch("requests.post")
    def test_valid_send_message(self, mock_post):
        mock_post.return_value.configure_mock(**{"ok": True})
        result = self.exporter._send_message(bytes(), {})
        self.assertEqual(mock_post.call_count, 1)
        self.assertEqual(result, MetricsExportResult.SUCCESS)

    def test_invalid_send_message(self):
        result = self.exporter._send_message(bytes(), {})
        self.assertEqual(result, MetricsExportResult.FAILURE)

    # Verifies that build_message calls snappy.compress and returns SerializedString
    @patch("snappy.compress", return_value=bytes())
    def test_build_message(self, mock_compress):
        message = self.exporter._build_message([TimeSeries()])
        self.assertEqual(mock_compress.call_count, 1)
        self.assertIsInstance(message, bytes)

    # Ensure correct headers are added when valid config is provided
    def test_build_headers(self):
        self.exporter.headers = {"Custom Header": "test_header"}

        headers = self.exporter._build_headers()
        self.assertEqual(headers["Content-Encoding"], "snappy")
        self.assertEqual(headers["Content-Type"], "application/x-protobuf")
        self.assertEqual(headers["X-Prometheus-Remote-Write-Version"], "0.1.0")
        self.assertEqual(headers["Custom Header"], "test_header")
