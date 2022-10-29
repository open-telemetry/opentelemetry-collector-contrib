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
from unittest.mock import patch

from pyramid.config import Configurator

from opentelemetry import trace
from opentelemetry.instrumentation.pyramid import PyramidInstrumentor
from opentelemetry.sdk.metrics.export import (
    HistogramDataPoint,
    NumberDataPoint,
)
from opentelemetry.test.globals_test import reset_trace_globals
from opentelemetry.test.wsgitestutil import WsgiTestBase
from opentelemetry.trace import SpanKind
from opentelemetry.trace.status import StatusCode
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SANITIZE_FIELDS,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
    _active_requests_count_attrs,
    _duration_attrs,
)

# pylint: disable=import-error
from .pyramid_base_test import InstrumentationTest

_expected_metric_names = [
    "http.server.active_requests",
    "http.server.duration",
]
_recommended_attrs = {
    "http.server.active_requests": _active_requests_count_attrs,
    "http.server.duration": _duration_attrs,
}


class TestAutomatic(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()

        PyramidInstrumentor().instrument()

        self.config = Configurator()

        self._common_initialization(self.config)

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            PyramidInstrumentor().uninstrument()

    def test_uninstrument(self):
        # pylint: disable=access-member-before-definition
        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        PyramidInstrumentor().uninstrument()
        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_tween_list(self):
        tween_list = "pyramid.tweens.excview_tween_factory"
        config = Configurator(settings={"pyramid.tweens": tween_list})
        self._common_initialization(config)
        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        PyramidInstrumentor().uninstrument()

        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_registry_name_is_this_module(self):
        config = Configurator()
        self.assertEqual(
            config.registry.__name__, __name__.rsplit(".", maxsplit=1)[0]
        )

    def test_redirect_response_is_not_an_error(self):
        tween_list = "pyramid.tweens.excview_tween_factory"
        config = Configurator(settings={"pyramid.tweens": tween_list})
        self._common_initialization(config)
        resp = self.client.get("/hello/302")
        self.assertEqual(302, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].status.status_code, StatusCode.UNSET)

        PyramidInstrumentor().uninstrument()

        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/302")
        self.assertEqual(302, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_204_empty_response_is_not_an_error(self):
        tween_list = "pyramid.tweens.excview_tween_factory"
        config = Configurator(settings={"pyramid.tweens": tween_list})
        self._common_initialization(config)
        resp = self.client.get("/hello/204")
        self.assertEqual(204, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].status.status_code, StatusCode.UNSET)

        PyramidInstrumentor().uninstrument()

        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/204")
        self.assertEqual(204, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_400s_response_is_not_an_error(self):
        tween_list = "pyramid.tweens.excview_tween_factory"
        config = Configurator(settings={"pyramid.tweens": tween_list})
        self._common_initialization(config)
        resp = self.client.get("/hello/404")
        self.assertEqual(404, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].status.status_code, StatusCode.UNSET)

        PyramidInstrumentor().uninstrument()

        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/404")
        self.assertEqual(404, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_pyramid_metric(self):
        self.client.get("/hello/756")
        self.client.get("/hello/756")
        self.client.get("/hello/756")
        metrics_list = self.memory_metrics_reader.get_metrics_data()
        number_data_point_seen = False
        histogram_data_point_seen = False
        self.assertTrue(len(metrics_list.resource_metrics) == 1)
        for resource_metric in metrics_list.resource_metrics:
            self.assertTrue(len(resource_metric.scope_metrics) == 1)
            for scope_metric in resource_metric.scope_metrics:
                self.assertTrue(len(scope_metric.metrics) == 2)
                for metric in scope_metric.metrics:
                    self.assertIn(metric.name, _expected_metric_names)
                    data_points = list(metric.data.data_points)
                    self.assertEqual(len(data_points), 1)
                    for point in data_points:
                        if isinstance(point, HistogramDataPoint):
                            self.assertEqual(point.count, 3)
                            histogram_data_point_seen = True
                        if isinstance(point, NumberDataPoint):
                            number_data_point_seen = True
                        for attr in point.attributes:
                            self.assertIn(
                                attr, _recommended_attrs[metric.name]
                            )
        self.assertTrue(number_data_point_seen and histogram_data_point_seen)

    def test_basic_metric_success(self):
        start = default_timer()
        self.client.get("/hello/756")
        duration = max(round((default_timer() - start) * 1000), 0)
        expected_duration_attributes = {
            "http.method": "GET",
            "http.host": "localhost",
            "http.scheme": "http",
            "http.flavor": "1.1",
            "http.server_name": "localhost",
            "net.host.port": 80,
            "http.status_code": 200,
        }
        expected_requests_count_attributes = {
            "http.method": "GET",
            "http.host": "localhost",
            "http.scheme": "http",
            "http.flavor": "1.1",
            "http.server_name": "localhost",
        }
        metrics_list = self.memory_metrics_reader.get_metrics_data()
        for metric in (
            metrics_list.resource_metrics[0].scope_metrics[0].metrics
        ):
            for point in list(metric.data.data_points):
                if isinstance(point, HistogramDataPoint):
                    self.assertDictEqual(
                        expected_duration_attributes,
                        dict(point.attributes),
                    )
                    self.assertEqual(point.count, 1)
                    self.assertAlmostEqual(duration, point.sum, delta=20)
                if isinstance(point, NumberDataPoint):
                    self.assertDictEqual(
                        expected_requests_count_attributes,
                        dict(point.attributes),
                    )
                    self.assertEqual(point.value, 0)

    def test_metric_uninstruemnt(self):
        self.client.get("/hello/756")
        PyramidInstrumentor().uninstrument()
        self.config = Configurator()
        self._common_initialization(self.config)
        self.client.get("/hello/756")
        metrics_list = self.memory_metrics_reader.get_metrics_data()
        for metric in (
            metrics_list.resource_metrics[0].scope_metrics[0].metrics
        ):
            for point in list(metric.data.data_points):
                if isinstance(point, HistogramDataPoint):
                    self.assertEqual(point.count, 1)
                if isinstance(point, NumberDataPoint):
                    self.assertEqual(point.value, 0)


class TestWrappedWithOtherFramework(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()
        PyramidInstrumentor().instrument()
        self.config = Configurator()
        self._common_initialization(self.config)

    def tearDown(self) -> None:
        super().tearDown()
        with self.disable_logging():
            PyramidInstrumentor().uninstrument()

    def test_with_existing_span(self):
        tracer_provider, _ = self.create_tracer_provider()
        tracer = tracer_provider.get_tracer(__name__)

        with tracer.start_as_current_span(
            "test", kind=SpanKind.SERVER
        ) as parent_span:
            resp = self.client.get("/hello/123")
            self.assertEqual(200, resp.status_code)
            span_list = self.memory_exporter.get_finished_spans()
            self.assertEqual(SpanKind.INTERNAL, span_list[0].kind)
            self.assertEqual(
                parent_span.get_span_context().span_id,
                span_list[0].parent.span_id,
            )


@patch.dict(
    "os.environ",
    {
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SANITIZE_FIELDS: ".*my-secret.*",
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,invalid-header,Regex-Test-Header-.*,Regex-Invalid-Test-Header-.*,.*my-secret.*",
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "content-type,content-length,my-custom-header,invalid-header,my-custom-regex-header-.*,invalid-regex-header-.*,.*my-secret.*",
    },
)
class TestCustomRequestResponseHeaders(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()
        PyramidInstrumentor().instrument()
        self.config = Configurator()
        self._common_initialization(self.config)

    def tearDown(self) -> None:
        super().tearDown()
        with self.disable_logging():
            PyramidInstrumentor().uninstrument()

    def test_custom_request_header_added_in_server_span(self):
        headers = {
            "Custom-Test-Header-1": "Test Value 1",
            "Custom-Test-Header-2": "TestValue2,TestValue3",
            "Custom-Test-Header-3": "TestValue4",
            "Regex-Test-Header-1": "Regex Test Value 1",
            "regex-test-header-2": "RegexTestValue2,RegexTestValue3",
            "My-Secret-Header": "My Secret Value",
        }
        resp = self.client.get("/hello/123", headers=headers)
        self.assertEqual(200, resp.status_code)
        span = self.memory_exporter.get_finished_spans()[0]
        expected = {
            "http.request.header.custom_test_header_1": ("Test Value 1",),
            "http.request.header.custom_test_header_2": (
                "TestValue2,TestValue3",
            ),
            "http.request.header.regex_test_header_1": ("Regex Test Value 1",),
            "http.request.header.regex_test_header_2": (
                "RegexTestValue2,RegexTestValue3",
            ),
            "http.request.header.my_secret_header": ("[REDACTED]",),
        }
        not_expected = {
            "http.request.header.custom_test_header_3": ("TestValue4",),
        }
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)
        for key, _ in not_expected.items():
            self.assertNotIn(key, span.attributes)

    def test_custom_request_header_not_added_in_internal_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span("test", kind=SpanKind.SERVER):
            headers = {
                "Custom-Test-Header-1": "Test Value 1",
                "Custom-Test-Header-2": "TestValue2,TestValue3",
            }
            resp = self.client.get("/hello/123", headers=headers)
            self.assertEqual(200, resp.status_code)
            span = self.memory_exporter.get_finished_spans()[0]
            not_expected = {
                "http.request.header.custom_test_header_1": ("Test Value 1",),
                "http.request.header.custom_test_header_2": (
                    "TestValue2,TestValue3",
                ),
            }
            self.assertEqual(span.kind, SpanKind.INTERNAL)
            for key, _ in not_expected.items():
                self.assertNotIn(key, span.attributes)

    def test_custom_response_header_added_in_server_span(self):
        resp = self.client.get("/test_custom_response_headers")
        self.assertEqual(200, resp.status_code)
        span = self.memory_exporter.get_finished_spans()[0]
        expected = {
            "http.response.header.content_type": (
                "text/plain; charset=utf-8",
            ),
            "http.response.header.content_length": ("7",),
            "http.response.header.my_custom_header": (
                "my-custom-value-1,my-custom-header-2",
            ),
            "http.response.header.my_custom_regex_header_1": (
                "my-custom-regex-value-1,my-custom-regex-value-2",
            ),
            "http.response.header.my_custom_regex_header_2": (
                "my-custom-regex-value-3,my-custom-regex-value-4",
            ),
            "http.response.header.my_secret_header": ("[REDACTED]",),
        }
        not_expected = {
            "http.response.header.dont_capture_me": ("test-value",)
        }
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)
        for key, _ in not_expected.items():
            self.assertNotIn(key, span.attributes)

    def test_custom_response_header_not_added_in_internal_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span("test", kind=SpanKind.SERVER):
            resp = self.client.get("/test_custom_response_headers")
            self.assertEqual(200, resp.status_code)
            span = self.memory_exporter.get_finished_spans()[0]
            not_expected = {
                "http.response.header.content_type": (
                    "text/plain; charset=utf-8",
                ),
                "http.response.header.content_length": ("7",),
                "http.response.header.my_custom_header": (
                    "my-custom-value-1,my-custom-header-2",
                ),
            }
            self.assertEqual(span.kind, SpanKind.INTERNAL)
            for key, _ in not_expected.items():
                self.assertNotIn(key, span.attributes)


@patch.dict(
    "os.environ",
    {
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SANITIZE_FIELDS: ".*my-secret.*",
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,invalid-header,Regex-Test-Header-.*,Regex-Invalid-Test-Header-.*,.*my-secret.*",
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "content-type,content-length,my-custom-header,invalid-header,my-custom-regex-header-.*,invalid-regex-header-.*,.*my-secret.*",
    },
)
class TestCustomHeadersNonRecordingSpan(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()
        # This is done because set_tracer_provider cannot override the
        # current tracer provider.
        reset_trace_globals()
        tracer_provider = trace.NoOpTracerProvider()
        trace.set_tracer_provider(tracer_provider)
        PyramidInstrumentor().instrument()
        self.config = Configurator()
        self._common_initialization(self.config)

    def tearDown(self) -> None:
        super().tearDown()
        with self.disable_logging():
            PyramidInstrumentor().uninstrument()

    def test_custom_header_non_recording_span(self):
        try:
            resp = self.client.get("/hello/123")
            self.assertEqual(200, resp.status_code)
        except Exception as exc:  # pylint: disable=W0703
            self.fail(f"Exception raised with NonRecordingSpan {exc}")
