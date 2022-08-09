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

from timeit import default_timer

import urllib3
import urllib3.exceptions
from urllib3.request import encode_multipart_formdata

from opentelemetry import trace
from opentelemetry.instrumentation.urllib3 import URLLib3Instrumentor
from opentelemetry.test.httptest import HttpTestBase
from opentelemetry.test.test_base import TestBase
from opentelemetry.util.http.httplib import HttpClientInstrumentor


class TestURLLib3InstrumentorWithRealSocket(HttpTestBase, TestBase):
    def setUp(self):
        super().setUp()
        self.assert_ip = self.server.server_address[0]
        self.http_host = ":".join(map(str, self.server.server_address[:2]))
        self.http_url_base = "http://" + self.http_host
        self.http_url = self.http_url_base + "/status/200"
        HttpClientInstrumentor().instrument()
        URLLib3Instrumentor().instrument()

    def tearDown(self):
        super().tearDown()
        HttpClientInstrumentor().uninstrument()
        URLLib3Instrumentor().uninstrument()

    @staticmethod
    def perform_request(url: str) -> urllib3.response.HTTPResponse:
        with urllib3.PoolManager() as pool:
            resp = pool.request("GET", url)
            resp.close()
        return resp

    def test_basic_http_success(self):
        response = self.perform_request(self.http_url)
        self.assert_success_span(response, self.http_url)

    def test_basic_http_success_using_connection_pool(self):
        with urllib3.HTTPConnectionPool(self.http_host, timeout=3) as pool:
            response = pool.request("GET", "/status/200")

            self.assert_success_span(response, self.http_url)

            # Test that when re-using an existing connection, everything still works.
            # Especially relevant for IP capturing.
            response = pool.request("GET", "/status/200")

            self.assert_success_span(response, self.http_url)

    def assert_span(self, num_spans=1):
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(num_spans, len(span_list))
        if num_spans == 0:
            return None
        self.memory_exporter.clear()
        if num_spans == 1:
            return span_list[0]
        return span_list

    def assert_success_span(
        self, response: urllib3.response.HTTPResponse, url: str
    ):
        self.assertEqual(b"Hello!", response.data)

        span = self.assert_span()
        self.assertIs(trace.SpanKind.CLIENT, span.kind)
        self.assertEqual("HTTP GET", span.name)

        attributes = {
            "http.status_code": 200,
            "net.peer.ip": self.assert_ip,
        }
        self.assertGreaterEqual(span.attributes.items(), attributes.items())


class TestURLLib3InstrumentorMetric(HttpTestBase, TestBase):
    def setUp(self):
        super().setUp()
        self.assert_ip = self.server.server_address[0]
        self.assert_port = self.server.server_address[1]
        self.http_host = ":".join(map(str, self.server.server_address[:2]))
        self.http_url_base = "http://" + self.http_host
        self.http_url = self.http_url_base + "/status/200"
        URLLib3Instrumentor().instrument(meter_provider=self.meter_provider)

    def tearDown(self):
        super().tearDown()
        URLLib3Instrumentor().uninstrument()

    def test_metric_uninstrument(self):
        with urllib3.PoolManager() as pool:
            pool.request("GET", self.http_url)
            URLLib3Instrumentor().uninstrument()
            pool.request("GET", self.http_url)

            metrics_list = self.memory_metrics_reader.get_metrics_data()
            for resource_metric in metrics_list.resource_metrics:
                for scope_metric in resource_metric.scope_metrics:
                    for metric in scope_metric.metrics:
                        for point in list(metric.data.data_points):
                            self.assertEqual(point.count, 1)

    def test_basic_metric_check_client_size_get(self):
        with urllib3.PoolManager() as pool:
            start_time = default_timer()
            response = pool.request("GET", self.http_url)
            client_duration_estimated = (default_timer() - start_time) * 1000

            expected_attributes = {
                "http.status_code": 200,
                "http.host": self.assert_ip,
                "http.method": "GET",
                "http.flavor": "1.1",
                "http.scheme": "http",
                "net.peer.name": self.assert_ip,
                "net.peer.port": self.assert_port,
            }
            expected_data = {
                "http.client.request.size": 0,
                "http.client.response.size": len(response.data),
            }
            expected_metrics = [
                "http.client.duration",
                "http.client.request.size",
                "http.client.response.size",
            ]

            resource_metrics = (
                self.memory_metrics_reader.get_metrics_data().resource_metrics
            )
            for metrics in resource_metrics:
                for scope_metrics in metrics.scope_metrics:
                    self.assertEqual(len(scope_metrics.metrics), 3)
                    for metric in scope_metrics.metrics:
                        for data_point in metric.data.data_points:
                            if metric.name in expected_data:
                                self.assertEqual(
                                    data_point.sum, expected_data[metric.name]
                                )
                            if metric.name == "http.client.duration":
                                self.assertAlmostEqual(
                                    data_point.sum,
                                    client_duration_estimated,
                                    delta=1000,
                                )
                            self.assertIn(metric.name, expected_metrics)
                            self.assertDictEqual(
                                expected_attributes,
                                dict(data_point.attributes),
                            )
                            self.assertEqual(data_point.count, 1)

    def test_basic_metric_check_client_size_post(self):
        with urllib3.PoolManager() as pool:
            start_time = default_timer()
            data_fields = {"data": "test"}
            response = pool.request("POST", self.http_url, fields=data_fields)
            client_duration_estimated = (default_timer() - start_time) * 1000

            expected_attributes = {
                "http.status_code": 501,
                "http.host": self.assert_ip,
                "http.method": "POST",
                "http.flavor": "1.1",
                "http.scheme": "http",
                "net.peer.name": self.assert_ip,
                "net.peer.port": self.assert_port,
            }

            body = encode_multipart_formdata(data_fields)[0]

            expected_data = {
                "http.client.request.size": len(body),
                "http.client.response.size": len(response.data),
            }
            expected_metrics = [
                "http.client.duration",
                "http.client.request.size",
                "http.client.response.size",
            ]

            resource_metrics = (
                self.memory_metrics_reader.get_metrics_data().resource_metrics
            )
            for metrics in resource_metrics:
                for scope_metrics in metrics.scope_metrics:
                    self.assertEqual(len(scope_metrics.metrics), 3)
                    for metric in scope_metrics.metrics:
                        for data_point in metric.data.data_points:
                            if metric.name in expected_data:
                                self.assertEqual(
                                    data_point.sum, expected_data[metric.name]
                                )
                            if metric.name == "http.client.duration":
                                self.assertAlmostEqual(
                                    data_point.sum,
                                    client_duration_estimated,
                                    delta=1000,
                                )
                            self.assertIn(metric.name, expected_metrics)

                            self.assertDictEqual(
                                expected_attributes,
                                dict(data_point.attributes),
                            )
                            self.assertEqual(data_point.count, 1)
