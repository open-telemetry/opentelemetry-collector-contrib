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

from unittest.mock import Mock, patch

from flask import Flask, request

from opentelemetry import trace
from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.instrumentation.propagators import (
    TraceResponsePropagator,
    get_global_response_propagator,
    set_global_response_propagator,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase
from opentelemetry.test.wsgitestutil import WsgiTestBase
from opentelemetry.util.http import get_excluded_urls

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


class TestProgrammatic(InstrumentationTest, TestBase, WsgiTestBase):
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
        headers = response.headers

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        span = span_list[0]

        self.assertIn("traceresponse", headers)
        self.assertEqual(
            headers["access-control-expose-headers"], "traceresponse",
        )
        self.assertEqual(
            headers["traceresponse"],
            "00-{0}-{1}-01".format(
                trace.format_trace_id(span.get_span_context().trace_id),
                trace.format_span_id(span.get_span_context().span_id),
            ),
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


class TestProgrammaticHooks(InstrumentationTest, TestBase, WsgiTestBase):
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


class TestProgrammaticHooksWithoutApp(
    InstrumentationTest, TestBase, WsgiTestBase
):
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


class TestProgrammaticCustomTracerProvider(
    InstrumentationTest, TestBase, WsgiTestBase
):
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
    InstrumentationTest, TestBase, WsgiTestBase
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
