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
from unittest.mock import patch

import pytest

from opentelemetry.exporter.prometheus_remote_write import (
    PrometheusRemoteWriteMetricsExporter,
)
from opentelemetry.exporter.prometheus_remote_write.gen.types_pb2 import (  # pylint: disable=E0611
    TimeSeries,
)
from opentelemetry.sdk.metrics.export import (
    Histogram,
    HistogramDataPoint,
    MetricExportResult,
    MetricsData,
    NumberDataPoint,
    ResourceMetrics,
    ScopeMetrics,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.util.instrumentation import InstrumentationScope


@pytest.mark.parametrize(
    "name,result",
    [
        ("abc.124", "abc_124"),
        (":abc", ":abc"),
        ("abc.name.hi", "abc_name_hi"),
        ("service.name...", "service_name___"),
        ("4hellowor:ld5∂©∑", "_hellowor:ld5___"),
    ],
)
def test_regex(name, result, prom_rw):
    assert prom_rw._sanitize_string(name, "name") == result


def test_regex_invalid(prom_rw):
    with pytest.raises(TypeError):
        prom_rw("foo_bar", "A random type")


def test_parse_data_point(prom_rw):

    attrs = {"Foo": "Bar", "Baz": 42}
    timestamp = 1641946016139533244
    value = 242.42
    dp = NumberDataPoint(attrs, 0, timestamp, value)
    name = "abc.123_42"
    labels, sample = prom_rw._parse_data_point(dp, name)

    name = "abc_123_42"
    assert labels == (("Foo", "Bar"), ("Baz", 42), ("__name__", name))
    assert sample == (value, timestamp // 1_000_000)


def test_parse_histogram_dp(prom_rw):
    attrs = {"foo": "bar", "baz": 42}
    timestamp = 1641946016139533244
    bounds = [10.0, 20.0]
    dp = HistogramDataPoint(
        attributes=attrs,
        start_time_unix_nano=1641946016139533244,
        time_unix_nano=timestamp,
        count=9,
        sum=180,
        bucket_counts=[1, 4, 4],
        explicit_bounds=bounds,
        min=8,
        max=80,
    )
    name = "foo_histogram"
    label_sample_pairs = prom_rw._parse_histogram_data_point(dp, name)
    timestamp = timestamp // 1_000_000
    bounds.append("+Inf")
    for pos, bound in enumerate(bounds):
        # We have to attributes, we kinda assume the bucket label is last...
        assert ("le", str(bound)) == label_sample_pairs[pos][0][-1]
        # Check and make sure we are putting the bucket counts in there
        assert (dp.bucket_counts[pos], timestamp) == label_sample_pairs[pos][1]

    # Last two are the sum & total count
    assert ("__name__", f"{name}_sum") in label_sample_pairs[-2][0]
    assert (dp.sum, timestamp) == label_sample_pairs[-2][1]

    assert ("__name__", f"{name}_count") in label_sample_pairs[-1][0]
    assert (dp.count, timestamp) == label_sample_pairs[-1][1]


@pytest.mark.parametrize(
    "metric",
    [
        "gauge",
        "sum",
        "histogram",
    ],
    indirect=["metric"],
)
def test_parse_metric(metric, prom_rw):
    """
    Ensures output from parse_metrics are TimeSeries with expected data/size
    """
    attributes = {
        "service_name": "foo",
        "bool_value": True,
    }

    assert (
        len(metric.data.data_points) == 1
    ), "We can only support a single datapoint in tests"
    series = prom_rw._parse_metric(metric, tuple(attributes.items()))
    timestamp = metric.data.data_points[0].time_unix_nano // 1_000_000
    for single_series in series:
        labels = str(single_series.labels)
        # Its a bit easier to validate these stringified where we dont have to
        # worry about ordering and protobuf TimeSeries object structure
        # This doesn't guarantee the labels aren't mixed up, but our other
        # test cases already do.
        assert "__name__" in labels
        assert prom_rw._sanitize_string(metric.name, "name") in labels
        combined_attrs = list(attributes.items()) + list(
            metric.data.data_points[0].attributes.items()
        )
        for name, value in combined_attrs:
            assert prom_rw._sanitize_string(name, "label") in labels
            assert str(value) in labels
        if isinstance(metric.data, Histogram):
            values = [
                metric.data.data_points[0].count,
                metric.data.data_points[0].sum,
                metric.data.data_points[0].bucket_counts[0],
                metric.data.data_points[0].bucket_counts[1],
            ]
        else:
            values = [
                metric.data.data_points[0].value,
            ]
        for sample in single_series.samples:
            assert sample.timestamp == timestamp
            assert sample.value in values


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


# Ensures export is successful with valid export_records and config
@patch("requests.post")
def test_valid_export(mock_post, prom_rw, metric):
    mock_post.return_value.configure_mock(**{"status_code": 200})

    # Assumed a "None" for Scope or Resource aren't valid, so build them here
    scope = ScopeMetrics(
        InstrumentationScope(name="prom-rw-test"), [metric], None
    )
    resource = ResourceMetrics(
        Resource({"service.name": "foo"}), [scope], None
    )
    record = MetricsData([resource])

    result = prom_rw.export(record)
    assert result == MetricExportResult.SUCCESS
    assert mock_post.call_count == 1

    result = prom_rw.export([])
    assert result == MetricExportResult.SUCCESS


def test_invalid_export(prom_rw):
    record = MetricsData([])

    result = prom_rw.export(record)
    assert result == MetricExportResult.FAILURE


@patch("requests.post")
def test_valid_send_message(mock_post, prom_rw):
    mock_post.return_value.configure_mock(**{"ok": True})
    result = prom_rw._send_message(bytes(), {})
    assert mock_post.call_count == 1
    assert result == MetricExportResult.SUCCESS


def test_invalid_send_message(prom_rw):
    result = prom_rw._send_message(bytes(), {})
    assert result == MetricExportResult.FAILURE


# Verifies that build_message calls snappy.compress and returns SerializedString
@patch("snappy.compress", return_value=bytes())
def test_build_message(mock_compress, prom_rw):
    message = prom_rw._build_message([TimeSeries()])
    assert mock_compress.call_count == 1
    assert isinstance(message, bytes)


# Ensure correct headers are added when valid config is provided
def test_build_headers(prom_rw):
    prom_rw.headers = {"Custom Header": "test_header"}

    headers = prom_rw._build_headers()
    assert headers["Content-Encoding"] == "snappy"
    assert headers["Content-Type"] == "application/x-protobuf"
    assert headers["X-Prometheus-Remote-Write-Version"] == "0.1.0"
    assert headers["Custom Header"] == "test_header"
