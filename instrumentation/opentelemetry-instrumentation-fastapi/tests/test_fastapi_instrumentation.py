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

import fastapi
from fastapi.responses import JSONResponse
from fastapi.testclient import TestClient

import opentelemetry.instrumentation.fastapi as otel_fastapi
from opentelemetry import trace
from opentelemetry.instrumentation.asgi import OpenTelemetryMiddleware
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.globals_test import reset_trace_globals
from opentelemetry.test.test_base import TestBase
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
    get_excluded_urls,
)


class TestFastAPIManualInstrumentation(TestBase):
    def _create_app(self):
        app = self._create_fastapi_app()
        self._instrumentor.instrument_app(
            app=app,
            server_request_hook=getattr(self, "server_request_hook", None),
            client_request_hook=getattr(self, "client_request_hook", None),
            client_response_hook=getattr(self, "client_response_hook", None),
        )
        return app

    def _create_app_explicit_excluded_urls(self):
        app = self._create_fastapi_app()
        to_exclude = "/user/123,/foobar"
        self._instrumentor.instrument_app(
            app,
            excluded_urls=to_exclude,
            server_request_hook=getattr(self, "server_request_hook", None),
            client_request_hook=getattr(self, "client_request_hook", None),
            client_response_hook=getattr(self, "client_response_hook", None),
        )
        return app

    def setUp(self):
        super().setUp()
        self.env_patch = patch.dict(
            "os.environ",
            {"OTEL_PYTHON_FASTAPI_EXCLUDED_URLS": "/exclude/123,healthzz"},
        )
        self.env_patch.start()
        self.exclude_patch = patch(
            "opentelemetry.instrumentation.fastapi._excluded_urls_from_env",
            get_excluded_urls("FASTAPI"),
        )
        self.exclude_patch.start()
        self._instrumentor = otel_fastapi.FastAPIInstrumentor()
        self._app = self._create_app()
        self._client = TestClient(self._app)

    def tearDown(self):
        super().tearDown()
        self.env_patch.stop()
        self.exclude_patch.stop()
        with self.disable_logging():
            self._instrumentor.uninstrument()
            self._instrumentor.uninstrument_app(self._app)

    def test_instrument_app_with_instrument(self):
        if not isinstance(self, TestAutoInstrumentation):
            self._instrumentor.instrument()
        self._client.get("/foobar")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        for span in spans:
            self.assertIn("/foobar", span.name)

    def test_uninstrument_app(self):
        self._client.get("/foobar")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        # pylint: disable=import-outside-toplevel
        from fastapi.middleware.httpsredirect import HTTPSRedirectMiddleware

        self._app.add_middleware(HTTPSRedirectMiddleware)
        self._instrumentor.uninstrument_app(self._app)
        print(self._app.user_middleware[0].cls)
        self.assertFalse(
            isinstance(
                self._app.user_middleware[0].cls, OpenTelemetryMiddleware
            )
        )
        self._client = TestClient(self._app)
        resp = self._client.get("/foobar")
        self.assertEqual(200, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 3)

    def test_uninstrument_app_after_instrument(self):
        if not isinstance(self, TestAutoInstrumentation):
            self._instrumentor.instrument()
        self._instrumentor.uninstrument_app(self._app)
        self._client.get("/foobar")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_basic_fastapi_call(self):
        self._client.get("/foobar")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        for span in spans:
            self.assertIn("/foobar", span.name)

    def test_fastapi_route_attribute_added(self):
        """Ensure that fastapi routes are used as the span name."""
        self._client.get("/user/123")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        for span in spans:
            self.assertIn("/user/{username}", span.name)
        self.assertEqual(
            spans[-1].attributes[SpanAttributes.HTTP_ROUTE], "/user/{username}"
        )
        # ensure that at least one attribute that is populated by
        # the asgi instrumentation is successfully feeding though.
        self.assertEqual(
            spans[-1].attributes[SpanAttributes.HTTP_FLAVOR], "1.1"
        )

    def test_fastapi_excluded_urls(self):
        """Ensure that given fastapi routes are excluded."""
        self._client.get("/exclude/123")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)
        self._client.get("/healthzz")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_fastapi_excluded_urls_not_env(self):
        """Ensure that given fastapi routes are excluded when passed explicitly (not in the environment)"""
        app = self._create_app_explicit_excluded_urls()
        client = TestClient(app)
        client.get("/user/123")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)
        client.get("/foobar")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    @staticmethod
    def _create_fastapi_app():
        app = fastapi.FastAPI()

        @app.get("/foobar")
        async def _():
            return {"message": "hello world"}

        @app.get("/user/{username}")
        async def _(username: str):
            return {"message": username}

        @app.get("/exclude/{param}")
        async def _(param: str):
            return {"message": param}

        @app.get("/healthzz")
        async def _():
            return {"message": "ok"}

        return app


class TestFastAPIManualInstrumentationHooks(TestFastAPIManualInstrumentation):
    _server_request_hook = None
    _client_request_hook = None
    _client_response_hook = None

    def server_request_hook(self, span, scope):
        if self._server_request_hook is not None:
            self._server_request_hook(span, scope)

    def client_request_hook(self, receive_span, request):
        if self._client_request_hook is not None:
            self._client_request_hook(receive_span, request)

    def client_response_hook(self, send_span, response):
        if self._client_response_hook is not None:
            self._client_response_hook(send_span, response)

    def test_hooks(self):
        def server_request_hook(span, scope):
            span.update_name("name from server hook")

        def client_request_hook(receive_span, request):
            receive_span.update_name("name from client hook")
            receive_span.set_attribute("attr-from-request-hook", "set")

        def client_response_hook(send_span, response):
            send_span.update_name("name from response hook")
            send_span.set_attribute("attr-from-response-hook", "value")

        self._server_request_hook = server_request_hook
        self._client_request_hook = client_request_hook
        self._client_response_hook = client_response_hook

        self._client.get("/foobar")
        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(
            len(spans), 3
        )  # 1 server span and 2 response spans (response start and body)

        server_span = spans[2]
        self.assertEqual(server_span.name, "name from server hook")

        response_spans = spans[:2]
        for span in response_spans:
            self.assertEqual(span.name, "name from response hook")
            self.assertSpanHasAttributes(
                span, {"attr-from-response-hook": "value"}
            )


class TestAutoInstrumentation(TestFastAPIManualInstrumentation):
    """Test the auto-instrumented variant

    Extending the manual instrumentation as most test cases apply
    to both.
    """

    def _create_app(self):
        # instrumentation is handled by the instrument call
        resource = Resource.create({"key1": "value1", "key2": "value2"})
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result
        self.memory_exporter = exporter

        self._instrumentor.instrument(tracer_provider=tracer_provider)
        return self._create_fastapi_app()

    def _create_app_explicit_excluded_urls(self):
        resource = Resource.create({"key1": "value1", "key2": "value2"})
        tracer_provider, exporter = self.create_tracer_provider(
            resource=resource
        )
        self.memory_exporter = exporter

        to_exclude = "/user/123,/foobar"
        self._instrumentor.uninstrument()  # Disable previous instrumentation (setUp)
        self._instrumentor.instrument(
            tracer_provider=tracer_provider,
            excluded_urls=to_exclude,
        )
        return self._create_fastapi_app()

    def test_request(self):
        self._client.get("/foobar")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        for span in spans:
            self.assertEqual(span.resource.attributes["key1"], "value1")
            self.assertEqual(span.resource.attributes["key2"], "value2")

    def tearDown(self):
        self._instrumentor.uninstrument()
        super().tearDown()


class TestAutoInstrumentationHooks(TestFastAPIManualInstrumentationHooks):
    """
    Test the auto-instrumented variant for request and response hooks

    Extending the manual instrumentation to inherit defined hooks and since most test cases apply
    to both.
    """

    def _create_app(self):
        # instrumentation is handled by the instrument call
        self._instrumentor.instrument(
            server_request_hook=getattr(self, "server_request_hook", None),
            client_request_hook=getattr(self, "client_request_hook", None),
            client_response_hook=getattr(self, "client_response_hook", None),
        )

        return self._create_fastapi_app()

    def _create_app_explicit_excluded_urls(self):
        resource = Resource.create({"key1": "value1", "key2": "value2"})
        tracer_provider, exporter = self.create_tracer_provider(
            resource=resource
        )
        self.memory_exporter = exporter

        to_exclude = "/user/123,/foobar"
        self._instrumentor.uninstrument()  # Disable previous instrumentation (setUp)
        self._instrumentor.instrument(
            tracer_provider=tracer_provider,
            excluded_urls=to_exclude,
            server_request_hook=getattr(self, "server_request_hook", None),
            client_request_hook=getattr(self, "client_request_hook", None),
            client_response_hook=getattr(self, "client_response_hook", None),
        )
        return self._create_fastapi_app()

    def tearDown(self):
        self._instrumentor.uninstrument()
        super().tearDown()


class TestAutoInstrumentationLogic(unittest.TestCase):
    def test_instrumentation(self):
        """Verify that instrumentation methods are instrumenting and
        removing as expected.
        """
        instrumentor = otel_fastapi.FastAPIInstrumentor()
        original = fastapi.FastAPI
        instrumentor.instrument()
        try:
            instrumented = fastapi.FastAPI
            self.assertIsNot(original, instrumented)
        finally:
            instrumentor.uninstrument()

        should_be_original = fastapi.FastAPI
        self.assertIs(original, should_be_original)


class TestWrappedApplication(TestBase):
    def setUp(self):
        super().setUp()

        self.app = fastapi.FastAPI()

        @self.app.get("/foobar")
        async def _():
            return {"message": "hello world"}

        otel_fastapi.FastAPIInstrumentor().instrument_app(self.app)
        self.client = TestClient(self.app)
        self.tracer = self.tracer_provider.get_tracer(__name__)

    def tearDown(self) -> None:
        super().tearDown()
        with self.disable_logging():
            otel_fastapi.FastAPIInstrumentor().uninstrument_app(self.app)

    def test_mark_span_internal_in_presence_of_span_from_other_framework(self):
        with self.tracer.start_as_current_span(
            "test", kind=trace.SpanKind.SERVER
        ) as parent_span:
            resp = self.client.get("/foobar")
            self.assertEqual(200, resp.status_code)

        span_list = self.memory_exporter.get_finished_spans()
        for span in span_list:
            print(str(span.__class__) + ": " + str(span.__dict__))

        # there should be 4 spans - single SERVER "test" and three INTERNAL "FastAPI"
        self.assertEqual(trace.SpanKind.INTERNAL, span_list[0].kind)
        self.assertEqual(trace.SpanKind.INTERNAL, span_list[1].kind)
        # main INTERNAL span - child of test
        self.assertEqual(trace.SpanKind.INTERNAL, span_list[2].kind)
        self.assertEqual(
            parent_span.context.span_id, span_list[2].parent.span_id
        )
        # SERVER "test"
        self.assertEqual(trace.SpanKind.SERVER, span_list[3].kind)
        self.assertEqual(
            parent_span.context.span_id, span_list[3].context.span_id
        )


class TestHTTPAppWithCustomHeaders(TestBase):
    def setUp(self):
        super().setUp()
        self.env_patch = patch.dict(
            "os.environ",
            {
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3",
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3",
            },
        )
        self.env_patch.start()
        self.app = self._create_app()
        otel_fastapi.FastAPIInstrumentor().instrument_app(self.app)
        self.client = TestClient(self.app)

    def tearDown(self) -> None:
        super().tearDown()
        self.env_patch.stop()
        with self.disable_logging():
            otel_fastapi.FastAPIInstrumentor().uninstrument_app(self.app)

    @staticmethod
    def _create_app():
        app = fastapi.FastAPI()

        @app.get("/foobar")
        async def _():
            headers = {
                "custom-test-header-1": "test-header-value-1",
                "custom-test-header-2": "test-header-value-2",
            }
            content = {"message": "hello world"}
            return JSONResponse(content=content, headers=headers)

        return app

    def test_http_custom_request_headers_in_span_attributes(self):
        expected = {
            "http.request.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.request.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        resp = self.client.get(
            "/foobar",
            headers={
                "custom-test-header-1": "test-header-value-1",
                "custom-test-header-2": "test-header-value-2",
            },
        )
        self.assertEqual(200, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 3)

        server_span = [
            span for span in span_list if span.kind == trace.SpanKind.SERVER
        ][0]

        self.assertSpanHasAttributes(server_span, expected)

    def test_http_custom_request_headers_not_in_span_attributes(self):
        not_expected = {
            "http.request.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        resp = self.client.get(
            "/foobar",
            headers={
                "custom-test-header-1": "test-header-value-1",
                "custom-test-header-2": "test-header-value-2",
            },
        )
        self.assertEqual(200, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 3)

        server_span = [
            span for span in span_list if span.kind == trace.SpanKind.SERVER
        ][0]

        for key, _ in not_expected.items():
            self.assertNotIn(key, server_span.attributes)

    def test_http_custom_response_headers_in_span_attributes(self):
        expected = {
            "http.response.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.response.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        resp = self.client.get("/foobar")
        self.assertEqual(200, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 3)

        server_span = [
            span for span in span_list if span.kind == trace.SpanKind.SERVER
        ][0]
        self.assertSpanHasAttributes(server_span, expected)

    def test_http_custom_response_headers_not_in_span_attributes(self):
        not_expected = {
            "http.reponse.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        resp = self.client.get("/foobar")
        self.assertEqual(200, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 3)

        server_span = [
            span for span in span_list if span.kind == trace.SpanKind.SERVER
        ][0]

        for key, _ in not_expected.items():
            self.assertNotIn(key, server_span.attributes)


class TestWebSocketAppWithCustomHeaders(TestBase):
    def setUp(self):
        super().setUp()
        self.env_patch = patch.dict(
            "os.environ",
            {
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3",
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3",
            },
        )
        self.env_patch.start()
        self.app = self._create_app()
        otel_fastapi.FastAPIInstrumentor().instrument_app(self.app)
        self.client = TestClient(self.app)

    def tearDown(self) -> None:
        super().tearDown()
        self.env_patch.stop()
        with self.disable_logging():
            otel_fastapi.FastAPIInstrumentor().uninstrument_app(self.app)

    @staticmethod
    def _create_app():
        app = fastapi.FastAPI()

        @app.websocket("/foobar_web")
        async def _(websocket: fastapi.WebSocket):
            message = await websocket.receive()
            if message.get("type") == "websocket.connect":
                await websocket.send(
                    {
                        "type": "websocket.accept",
                        "headers": [
                            (b"custom-test-header-1", b"test-header-value-1"),
                            (b"custom-test-header-2", b"test-header-value-2"),
                        ],
                    }
                )
                await websocket.send_json({"message": "hello world"})
                await websocket.close()
            if message.get("type") == "websocket.disconnect":
                pass

        return app

    def test_web_socket_custom_request_headers_in_span_attributes(self):
        expected = {
            "http.request.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.request.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }

        with self.client.websocket_connect(
            "/foobar_web",
            headers={
                "custom-test-header-1": "test-header-value-1",
                "custom-test-header-2": "test-header-value-2",
            },
        ) as websocket:
            data = websocket.receive_json()
            self.assertEqual(data, {"message": "hello world"})

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 5)

        server_span = [
            span for span in span_list if span.kind == trace.SpanKind.SERVER
        ][0]

        self.assertSpanHasAttributes(server_span, expected)

    def test_web_socket_custom_request_headers_not_in_span_attributes(self):
        not_expected = {
            "http.request.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }

        with self.client.websocket_connect(
            "/foobar_web",
            headers={
                "custom-test-header-1": "test-header-value-1",
                "custom-test-header-2": "test-header-value-2",
            },
        ) as websocket:
            data = websocket.receive_json()
            self.assertEqual(data, {"message": "hello world"})

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 5)

        server_span = [
            span for span in span_list if span.kind == trace.SpanKind.SERVER
        ][0]

        for key, _ in not_expected.items():
            self.assertNotIn(key, server_span.attributes)

    def test_web_socket_custom_response_headers_in_span_attributes(self):
        expected = {
            "http.response.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.response.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }

        with self.client.websocket_connect("/foobar_web") as websocket:
            data = websocket.receive_json()
            self.assertEqual(data, {"message": "hello world"})

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 5)

        server_span = [
            span for span in span_list if span.kind == trace.SpanKind.SERVER
        ][0]

        self.assertSpanHasAttributes(server_span, expected)

    def test_web_socket_custom_response_headers_not_in_span_attributes(self):
        not_expected = {
            "http.reponse.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }

        with self.client.websocket_connect("/foobar_web") as websocket:
            data = websocket.receive_json()
            self.assertEqual(data, {"message": "hello world"})

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 5)

        server_span = [
            span for span in span_list if span.kind == trace.SpanKind.SERVER
        ][0]

        for key, _ in not_expected.items():
            self.assertNotIn(key, server_span.attributes)


class TestNonRecordingSpanWithCustomHeaders(TestBase):
    def setUp(self):
        super().setUp()
        self.env_patch = patch.dict(
            "os.environ",
            {
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3",
            },
        )
        self.env_patch.start()
        self.app = fastapi.FastAPI()

        @self.app.get("/foobar")
        async def _():
            return {"message": "hello world"}

        reset_trace_globals()
        tracer_provider = trace.NoOpTracerProvider()
        trace.set_tracer_provider(tracer_provider=tracer_provider)

        self._instrumentor = otel_fastapi.FastAPIInstrumentor()
        self._instrumentor.instrument_app(self.app)
        self.client = TestClient(self.app)

    def tearDown(self) -> None:
        super().tearDown()
        with self.disable_logging():
            self._instrumentor.uninstrument_app(self.app)

    def test_custom_header_not_present_in_non_recording_span(self):
        resp = self.client.get(
            "/foobar",
            headers={
                "custom-test-header-1": "test-header-value-1",
            },
        )
        self.assertEqual(200, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)
