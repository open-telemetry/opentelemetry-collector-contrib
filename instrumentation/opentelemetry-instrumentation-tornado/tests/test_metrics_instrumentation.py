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

from tornado.testing import AsyncHTTPTestCase

from opentelemetry import trace
from opentelemetry.instrumentation.tornado import TornadoInstrumentor
from opentelemetry.sdk.metrics.export import (
    HistogramDataPoint,
    NumberDataPoint,
)
from opentelemetry.test.test_base import TestBase

from .tornado_test_app import make_app


class TornadoTest(AsyncHTTPTestCase, TestBase):
    # pylint:disable=no-self-use
    def get_app(self):
        tracer = trace.get_tracer(__name__)
        app = make_app(tracer)
        return app

    def get_sorted_metrics(self):
        resource_metrics = (
            self.memory_metrics_reader.get_metrics_data().resource_metrics
        )
        for metrics in resource_metrics:
            for scope_metrics in metrics.scope_metrics:
                all_metrics = list(scope_metrics.metrics)
                return self.sorted_metrics(all_metrics)

    @staticmethod
    def sorted_metrics(metrics):
        """
        Sorts metrics by metric name.
        """
        return sorted(
            metrics,
            key=lambda m: m.name,
        )

    def assert_metric_expected(
        self, metric, expected_value, expected_attributes
    ):
        data_point = next(iter(metric.data.data_points))

        if isinstance(data_point, HistogramDataPoint):
            self.assertEqual(
                data_point.sum,
                expected_value,
            )
        elif isinstance(data_point, NumberDataPoint):
            self.assertEqual(
                data_point.value,
                expected_value,
            )

        self.assertDictEqual(
            expected_attributes,
            dict(data_point.attributes),
        )

    def assert_duration_metric_expected(
        self, metric, duration_estimated, expected_attributes
    ):
        data_point = next(iter(metric.data.data_points))

        self.assertAlmostEqual(
            data_point.sum,
            duration_estimated,
            delta=200,
        )

        self.assertDictEqual(
            expected_attributes,
            dict(data_point.attributes),
        )

    def setUp(self):
        super().setUp()
        TornadoInstrumentor().instrument(
            server_request_hook=getattr(self, "server_request_hook", None),
            client_request_hook=getattr(self, "client_request_hook", None),
            client_response_hook=getattr(self, "client_response_hook", None),
            meter_provider=self.meter_provider,
        )

    def tearDown(self):
        TornadoInstrumentor().uninstrument()
        super().tearDown()


class TestTornadoInstrumentor(TornadoTest):
    def test_basic_metrics(self):
        start_time = default_timer()
        response = self.fetch("/")
        client_duration_estimated = (default_timer() - start_time) * 1000

        metrics = self.get_sorted_metrics()
        self.assertEqual(len(metrics), 7)

        (
            client_duration,
            client_request_size,
            client_response_size,
        ) = metrics[:3]

        (
            server_active_request,
            server_duration,
            server_request_size,
            server_response_size,
        ) = metrics[3:]

        self.assertEqual(
            server_active_request.name, "http.server.active_requests"
        )
        self.assert_metric_expected(
            server_active_request,
            0,
            {
                "http.method": "GET",
                "http.flavor": "HTTP/1.1",
                "http.scheme": "http",
                "http.target": "/",
                "http.host": response.request.headers["host"],
            },
        )

        self.assertEqual(server_duration.name, "http.server.duration")
        self.assert_duration_metric_expected(
            server_duration,
            client_duration_estimated,
            {
                "http.status_code": response.code,
                "http.method": "GET",
                "http.flavor": "HTTP/1.1",
                "http.scheme": "http",
                "http.target": "/",
                "http.host": response.request.headers["host"],
            },
        )

        self.assertEqual(server_request_size.name, "http.server.request.size")
        self.assert_metric_expected(
            server_request_size,
            0,
            {
                "http.status_code": 200,
                "http.method": "GET",
                "http.flavor": "HTTP/1.1",
                "http.scheme": "http",
                "http.target": "/",
                "http.host": response.request.headers["host"],
            },
        )

        self.assertEqual(
            server_response_size.name, "http.server.response.size"
        )
        self.assert_metric_expected(
            server_response_size,
            len(response.body),
            {
                "http.status_code": response.code,
                "http.method": "GET",
                "http.flavor": "HTTP/1.1",
                "http.scheme": "http",
                "http.target": "/",
                "http.host": response.request.headers["host"],
            },
        )

        self.assertEqual(client_duration.name, "http.client.duration")
        self.assert_duration_metric_expected(
            client_duration,
            client_duration_estimated,
            {
                "http.status_code": response.code,
                "http.method": "GET",
                "http.url": response.effective_url,
            },
        )

        self.assertEqual(client_request_size.name, "http.client.request.size")
        self.assert_metric_expected(
            client_request_size,
            0,
            {
                "http.status_code": response.code,
                "http.method": "GET",
                "http.url": response.effective_url,
            },
        )

        self.assertEqual(
            client_response_size.name, "http.client.response.size"
        )
        self.assert_metric_expected(
            client_response_size,
            len(response.body),
            {
                "http.status_code": response.code,
                "http.method": "GET",
                "http.url": response.effective_url,
            },
        )

    def test_metric_uninstrument(self):
        self.fetch("/")
        TornadoInstrumentor().uninstrument()
        self.fetch("/")

        metrics_list = self.memory_metrics_reader.get_metrics_data()
        for resource_metric in metrics_list.resource_metrics:
            for scope_metric in resource_metric.scope_metrics:
                for metric in scope_metric.metrics:
                    for point in list(metric.data.data_points):
                        if isinstance(point, HistogramDataPoint):
                            self.assertEqual(point.count, 1)
