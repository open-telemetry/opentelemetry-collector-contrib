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

from opentelemetry.exporter.prometheus_remote_write import (
    PrometheusRemoteWriteMetricsExporter,
)


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
        pass

    # Ensures conversion to timeseries function works with valid aggregation types
    def test_valid_convert_to_timeseries(self):
        pass

    # Ensures conversion to timeseries fails for unsupported aggregation types
    def test_invalid_convert_to_timeseries(self):
        pass

    # Ensures sum aggregator is correctly converted to timeseries
    def test_convert_from_sum(self):
        pass

    # Ensures sum min_max_count aggregator is correctly converted to timeseries
    def test_convert_from_min_max_sum_count(self):
        pass

    # Ensures histogram aggregator is correctly converted to timeseries
    def test_convert_from_histogram(self):
        pass

    # Ensures last value aggregator is correctly converted to timeseries
    def test_convert_from_last_value(self):
        pass

    # Ensures value observer aggregator is correctly converted to timeseries
    def test_convert_from_value_observer(self):
        pass

    # Ensures quantile aggregator is correctly converted to timeseries
    # TODO: Add test once method is implemented
    def test_convert_from_quantile(self):
        pass

    # Ensures timeseries produced contains appropriate sample and labels
    def test_create_timeseries(self):
        pass


class TestExport(unittest.TestCase):
    # Initializes test data that is reused across tests
    def setUp(self):
        pass

    # Ensures export is successful with valid export_records and config
    def test_export(self):
        pass

    def test_valid_send_message(self):
        pass

    def test_invalid_send_message(self):
        pass

    # Verifies that build_message calls snappy.compress and returns SerializedString
    def test_build_message(self):
        pass

    # Ensure correct headers are added when valid config is provided
    def test_get_headers(self):
        pass
