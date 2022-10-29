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
from unittest.mock import Mock, patch

from flask import Flask, request

from opentelemetry import trace
from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.instrumentation.propagators import (
    TraceResponsePropagator,
    get_global_response_propagator,
    set_global_response_propagator,
)
from opentelemetry.instrumentation.wsgi import (
    OpenTelemetryMiddleware,
    _active_requests_count_attrs,
    _duration_attrs,
)
from opentelemetry.sdk.metrics.export import (
    HistogramDataPoint,
    NumberDataPoint,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.wsgitestutil import WsgiTestBase
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SANITIZE_FIELDS,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
    get_excluded_urls,
)

# pylint: disable=import-error
from .base_test import InstrumentationTest


def expected_attributes(override_attributes):
    default_attributes = {
        SpanAttributes.HTTP_METHOD: "GET",
        SpanAttributes.HTTP_SERVER_NAME: "localhost",
        SpanAttributes.HTTP_SCHEME: "http",
        SpanAttributes.NET_HOST_PORT: 80,
        SpanAttributes.HTTP_HOST: "localhost",
        SpanAttributes.HTTP_TARGET: "/",
        SpanAttributes.HTTP_FLAVOR: "1.1",
        SpanAttributes.HTTP_STATUS_CODE: 200,
    }
    for key, val in override_attributes.items():
        default_attributes[key] = val
    return default_attributes


_expected_metric_names = [
    "http.server.active_requests",
    "http.server.duration",
]
_recommended_attrs = {
    "http.server.active_requests": _active_requests_count_attrs,
    "http.server.duration": _duration_attrs,
}


class TestProgrammatic(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()

        self.env_patch = patch.dict(
            "os.environ",
            {
                "OTEL_PYTHON_FLASK_EXCLUDED_URLS": "http://localhost/env_excluded_arg/123,env_excluded_noarg"
            },
        )
        self.env_patch.start()

        self.exclude_patch = patch(
            "opentelemetry.instrumentation.flask._excluded_urls_from_env",
            get_excluded_urls("FLASK"),
        )
        self.exclude_patch.start()

        self.app = Flask(__name__)
        FlaskInstrumentor().instrument_app(self.app)

        self._common_initialization()

    def tearDown(self):
        super().tearDown()
        self.env_patch.stop()
        self.exclude_patch.stop()
        with self.disable_logging():
            FlaskInstrumentor().uninstrument_app(self.app)

    def test_instrument_app_and_instrument(self):
        FlaskInstrumentor().instrument()
        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        FlaskInstrumentor().uninstrument()

    def test_uninstrument_app(self):
        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        FlaskInstrumentor().uninstrument_app(self.app)

        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_uninstrument_app_after_instrument(self):
        FlaskInstrumentor().instrument()
        FlaskInstrumentor().uninstrument_app(self.app)
        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)
        FlaskInstrumentor().uninstrument()

    # pylint: disable=no-member
    def test_only_strings_in_environ(self):
        """
        Some WSGI servers (such as Gunicorn) expect keys in the environ object
        to be strings

        OpenTelemetry should adhere to this convention.
        """
        nonstring_keys = set()

        def assert_environ():
            for key in request.environ:
                if not isinstance(key, str):
                    nonstring_keys.add(key)
            return "hi"

        self.app.route("/assert_environ")(assert_environ)
        self.client.get("/assert_environ")
        self.assertEqual(nonstring_keys, set())

    def test_simple(self):
        expected_attrs = expected_attributes(
            {
                SpanAttributes.HTTP_TARGET: "/hello/123",
                SpanAttributes.HTTP_ROUTE: "/hello/<int:helloid>",
            }
        )
        self.client.get("/hello/123")

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].name, "/hello/<int:helloid>")
        self.assertEqual(span_list[0].kind, trace.SpanKind.SERVER)
        self.assertEqual(span_list[0].attributes, expected_attrs)

    def test_trace_response(self):
        orig = get_global_response_propagator()

        set_global_response_propagator(TraceResponsePropagator())
        response = self.client.get("/hello/123")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        self.assertTraceResponseHeaderMatchesSpan(
            response.headers,
            span_list[0],
        )

        set_global_response_propagator(orig)

    def test_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            self.client.get("/hello/123")
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_404(self):
        expected_attrs = expected_attributes(
            {
                SpanAttributes.HTTP_METHOD: "POST",
                SpanAttributes.HTTP_TARGET: "/bye",
                SpanAttributes.HTTP_STATUS_CODE: 404,
            }
        )

        resp = self.client.post("/bye")
        self.assertEqual(404, resp.status_code)
        resp.close()
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].name, "HTTP POST")
        self.assertEqual(span_list[0].kind, trace.SpanKind.SERVER)
        self.assertEqual(span_list[0].attributes, expected_attrs)

    def test_internal_error(self):
        expected_attrs = expected_attributes(
            {
                SpanAttributes.HTTP_TARGET: "/hello/500",
                SpanAttributes.HTTP_ROUTE: "/hello/<int:helloid>",
                SpanAttributes.HTTP_STATUS_CODE: 500,
            }
        )
        resp = self.client.get("/hello/500")
        self.assertEqual(500, resp.status_code)
        resp.close()
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].name, "/hello/<int:helloid>")
        self.assertEqual(span_list[0].kind, trace.SpanKind.SERVER)
        self.assertEqual(span_list[0].attributes, expected_attrs)

    def test_exclude_lists_from_env(self):
        self.client.get("/env_excluded_arg/123")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)

        self.client.get("/env_excluded_arg/125")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        self.client.get("/env_excluded_noarg")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        self.client.get("/env_excluded_noarg2")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_exclude_lists_from_explicit(self):
        excluded_urls = "http://localhost/explicit_excluded_arg/123,explicit_excluded_noarg"
        app = Flask(__name__)
        FlaskInstrumentor().instrument_app(app, excluded_urls=excluded_urls)
        client = app.test_client()

        client.get("/explicit_excluded_arg/123")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)

        client.get("/explicit_excluded_arg/125")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        client.get("/explicit_excluded_noarg")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        client.get("/explicit_excluded_noarg2")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_flask_metrics(self):
        start = default_timer()
        self.client.get("/hello/123")
        self.client.get("/hello/321")
        self.client.get("/hello/756")
        duration = max(round((default_timer() - start) * 1000), 0)
        metrics_list = self.memory_metrics_reader.get_metrics_data()
        number_data_point_seen = False
        histogram_data_point_seen = False
        self.assertTrue(len(metrics_list.resource_metrics) != 0)
        for resource_metric in metrics_list.resource_metrics:
            self.assertTrue(len(resource_metric.scope_metrics) != 0)
            for scope_metric in resource_metric.scope_metrics:
                self.assertTrue(len(scope_metric.metrics) != 0)
                for metric in scope_metric.metrics:
                    self.assertIn(metric.name, _expected_metric_names)
                    data_points = list(metric.data.data_points)
                    self.assertEqual(len(data_points), 1)
                    for point in data_points:
                        if isinstance(point, HistogramDataPoint):
                            self.assertEqual(point.count, 3)
                            self.assertAlmostEqual(
                                duration, point.sum, delta=10
                            )
                            histogram_data_point_seen = True
                        if isinstance(point, NumberDataPoint):
                            number_data_point_seen = True
                        for attr in point.attributes:
                            self.assertIn(
                                attr, _recommended_attrs[metric.name]
                            )
        self.assertTrue(number_data_point_seen and histogram_data_point_seen)

    def test_flask_metric_values(self):
        start = default_timer()
        self.client.post("/hello/756")
        self.client.post("/hello/756")
        self.client.post("/hello/756")
        duration = max(round((default_timer() - start) * 1000), 0)
        metrics_list = self.memory_metrics_reader.get_metrics_data()
        for resource_metric in metrics_list.resource_metrics:
            for scope_metric in resource_metric.scope_metrics:
                for metric in scope_metric.metrics:
                    for point in list(metric.data.data_points):
                        if isinstance(point, HistogramDataPoint):
                            self.assertEqual(point.count, 3)
                            self.assertAlmostEqual(
                                duration, point.sum, delta=10
                            )
                        if isinstance(point, NumberDataPoint):
                            self.assertEqual(point.value, 0)

    def test_basic_metric_success(self):
        self.client.get("/hello/756")
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
        for resource_metric in metrics_list.resource_metrics:
            for scope_metrics in resource_metric.scope_metrics:
                for metric in scope_metrics.metrics:
                    for point in list(metric.data.data_points):
                        if isinstance(point, HistogramDataPoint):
                            self.assertDictEqual(
                                expected_duration_attributes,
                                dict(point.attributes),
                            )
                            self.assertEqual(point.count, 1)
                        elif isinstance(point, NumberDataPoint):
                            self.assertDictEqual(
                                expected_requests_count_attributes,
                                dict(point.attributes),
                            )
                            self.assertEqual(point.value, 0)

    def test_metric_uninstrument(self):
        self.client.delete("/hello/756")
        FlaskInstrumentor().uninstrument_app(self.app)
        self.client.delete("/hello/756")
        metrics_list = self.memory_metrics_reader.get_metrics_data()
        for resource_metric in metrics_list.resource_metrics:
            for scope_metric in resource_metric.scope_metrics:
                for metric in scope_metric.metrics:
                    for point in list(metric.data.data_points):
                        if isinstance(point, HistogramDataPoint):
                            self.assertEqual(point.count, 1)


class TestProgrammaticHooks(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()

        hook_headers = (
            "hook_attr",
            "hello otel",
        )

        def request_hook_test(span, environ):
            span.update_name("name from hook")

        def response_hook_test(span, environ, response_headers):
            span.set_attribute("hook_attr", "hello world")
            response_headers.append(hook_headers)

        self.app = Flask(__name__)

        FlaskInstrumentor().instrument_app(
            self.app,
            request_hook=request_hook_test,
            response_hook=response_hook_test,
        )

        self._common_initialization()

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FlaskInstrumentor().uninstrument_app(self.app)

    def test_hooks(self):
        expected_attrs = expected_attributes(
            {
                "http.target": "/hello/123",
                "http.route": "/hello/<int:helloid>",
                "hook_attr": "hello world",
            }
        )

        resp = self.client.get("/hello/123")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].name, "name from hook")
        self.assertEqual(span_list[0].attributes, expected_attrs)
        self.assertEqual(resp.headers["hook_attr"], "hello otel")


class TestProgrammaticHooksWithoutApp(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()

        hook_headers = (
            "hook_attr",
            "hello otel without app",
        )

        def request_hook_test(span, environ):
            span.update_name("without app")

        def response_hook_test(span, environ, response_headers):
            span.set_attribute("hook_attr", "hello world without app")
            response_headers.append(hook_headers)

        FlaskInstrumentor().instrument(
            request_hook=request_hook_test, response_hook=response_hook_test
        )
        # pylint: disable=import-outside-toplevel,reimported,redefined-outer-name
        from flask import Flask

        self.app = Flask(__name__)

        self._common_initialization()

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FlaskInstrumentor().uninstrument()

    def test_no_app_hooks(self):
        expected_attrs = expected_attributes(
            {
                "http.target": "/hello/123",
                "http.route": "/hello/<int:helloid>",
                "hook_attr": "hello world without app",
            }
        )
        resp = self.client.get("/hello/123")

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].name, "without app")
        self.assertEqual(span_list[0].attributes, expected_attrs)
        self.assertEqual(resp.headers["hook_attr"], "hello otel without app")


class TestProgrammaticCustomTracerProvider(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()
        resource = Resource.create({"service.name": "flask-api"})
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result
        self.memory_exporter = exporter

        self.app = Flask(__name__)

        FlaskInstrumentor().instrument_app(
            self.app, tracer_provider=tracer_provider
        )
        self._common_initialization()

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FlaskInstrumentor().uninstrument_app(self.app)

    def test_custom_span_name(self):
        self.client.get("/hello/123")

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(
            span_list[0].resource.attributes["service.name"], "flask-api"
        )


class TestProgrammaticCustomTracerProviderWithoutApp(
    InstrumentationTest, WsgiTestBase
):
    def setUp(self):
        super().setUp()
        resource = Resource.create({"service.name": "flask-api-no-app"})
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result
        self.memory_exporter = exporter

        FlaskInstrumentor().instrument(tracer_provider=tracer_provider)
        # pylint: disable=import-outside-toplevel,reimported,redefined-outer-name
        from flask import Flask

        self.app = Flask(__name__)

        self._common_initialization()

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FlaskInstrumentor().uninstrument()

    def test_custom_span_name(self):
        self.client.get("/hello/123")

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(
            span_list[0].resource.attributes["service.name"],
            "flask-api-no-app",
        )


class TestProgrammaticWrappedWithOtherFramework(
    InstrumentationTest, WsgiTestBase
):
    def setUp(self):
        super().setUp()

        self.app = Flask(__name__)
        self.app.wsgi_app = OpenTelemetryMiddleware(self.app.wsgi_app)
        FlaskInstrumentor().instrument_app(self.app)
        self._common_initialization()

    def tearDown(self) -> None:
        super().tearDown()
        with self.disable_logging():
            FlaskInstrumentor().uninstrument_app(self.app)

    def test_mark_span_internal_in_presence_of_span_from_other_framework(self):
        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        resp.get_data()
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(trace.SpanKind.INTERNAL, span_list[0].kind)
        self.assertEqual(trace.SpanKind.SERVER, span_list[1].kind)
        self.assertEqual(
            span_list[0].parent.span_id, span_list[1].context.span_id
        )


@patch.dict(
    "os.environ",
    {
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SANITIZE_FIELDS: ".*my-secret.*",
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3,Regex-Test-Header-.*,Regex-Invalid-Test-Header-.*,.*my-secret.*",
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "content-type,content-length,my-custom-header,invalid-header,my-custom-regex-header-.*,invalid-regex-header-.*,.*my-secret.*",
    },
)
class TestCustomRequestResponseHeaders(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()

        self.app = Flask(__name__)
        FlaskInstrumentor().instrument_app(self.app)

        self._common_initialization()

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FlaskInstrumentor().uninstrument_app(self.app)

    def test_custom_request_header_added_in_server_span(self):
        headers = {
            "Custom-Test-Header-1": "Test Value 1",
            "Custom-Test-Header-2": "TestValue2,TestValue3",
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
        self.assertEqual(span.kind, trace.SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)

    def test_custom_request_header_not_added_in_internal_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span("test", kind=trace.SpanKind.SERVER):
            headers = {
                "Custom-Test-Header-1": "Test Value 1",
                "Custom-Test-Header-2": "TestValue2,TestValue3",
                "Regex-Test-Header-1": "Regex Test Value 1",
                "regex-test-header-2": "RegexTestValue2,RegexTestValue3",
                "My-Secret-Header": "My Secret Value",
            }
            resp = self.client.get("/hello/123", headers=headers)
            self.assertEqual(200, resp.status_code)
            span = self.memory_exporter.get_finished_spans()[0]
            not_expected = {
                "http.request.header.custom_test_header_1": ("Test Value 1",),
                "http.request.header.custom_test_header_2": (
                    "TestValue2,TestValue3",
                ),
                "http.request.header.regex_test_header_1": (
                    "Regex Test Value 1",
                ),
                "http.request.header.regex_test_header_2": (
                    "RegexTestValue2,RegexTestValue3",
                ),
                "http.request.header.my_secret_header": ("[REDACTED]",),
            }
            self.assertEqual(span.kind, trace.SpanKind.INTERNAL)
            for key, _ in not_expected.items():
                self.assertNotIn(key, span.attributes)

    def test_custom_response_header_added_in_server_span(self):
        resp = self.client.get("/test_custom_response_headers")
        self.assertEqual(resp.status_code, 200)
        span = self.memory_exporter.get_finished_spans()[0]
        expected = {
            "http.response.header.content_type": (
                "text/plain; charset=utf-8",
            ),
            "http.response.header.content_length": ("13",),
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
        self.assertEqual(span.kind, trace.SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)

    def test_custom_response_header_not_added_in_internal_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span("test", kind=trace.SpanKind.SERVER):
            resp = self.client.get("/test_custom_response_headers")
            self.assertEqual(resp.status_code, 200)
            span = self.memory_exporter.get_finished_spans()[0]
            not_expected = {
                "http.response.header.content_type": (
                    "text/plain; charset=utf-8",
                ),
                "http.response.header.content_length": ("13",),
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
            self.assertEqual(span.kind, trace.SpanKind.INTERNAL)
            for key, _ in not_expected.items():
                self.assertNotIn(key, span.attributes)
