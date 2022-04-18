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

from starlette import applications
from starlette.responses import PlainTextResponse
from starlette.routing import Route
from starlette.testclient import TestClient
from starlette.websockets import WebSocket

import opentelemetry.instrumentation.starlette as otel_starlette
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.globals_test import reset_trace_globals
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import (
    NoOpTracerProvider,
    SpanKind,
    get_tracer,
    set_tracer_provider,
)
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
    get_excluded_urls,
)


class TestStarletteManualInstrumentation(TestBase):
    def _create_app(self):
        app = self._create_starlette_app()
        self._instrumentor.instrument_app(
            app=app,
            server_request_hook=getattr(self, "server_request_hook", None),
            client_request_hook=getattr(self, "client_request_hook", None),
            client_response_hook=getattr(self, "client_response_hook", None),
        )
        return app

    def setUp(self):
        super().setUp()
        self.env_patch = patch.dict(
            "os.environ",
            {"OTEL_PYTHON_STARLETTE_EXCLUDED_URLS": "/exclude/123,healthzz"},
        )
        self.env_patch.start()
        self.exclude_patch = patch(
            "opentelemetry.instrumentation.starlette._excluded_urls",
            get_excluded_urls("STARLETTE"),
        )
        self.exclude_patch.start()
        self._instrumentor = otel_starlette.StarletteInstrumentor()
        self._app = self._create_app()
        self._client = TestClient(self._app)

    def tearDown(self):
        super().tearDown()
        self.env_patch.stop()
        self.exclude_patch.stop()

    def test_basic_starlette_call(self):
        self._client.get("/foobar")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        for span in spans:
            self.assertIn("/foobar", span.name)

    def test_starlette_route_attribute_added(self):
        """Ensure that starlette routes are used as the span name."""
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

    def test_starlette_excluded_urls(self):
        """Ensure that givem starlette routes are excluded."""
        self._client.get("/healthzz")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    @staticmethod
    def _create_starlette_app():
        def home(_):
            return PlainTextResponse("hi")

        def health(_):
            return PlainTextResponse("ok")

        app = applications.Starlette(
            routes=[
                Route("/foobar", home),
                Route("/user/{username}", home),
                Route("/healthzz", health),
            ]
        )
        return app


class TestStarletteManualInstrumentationHooks(
    TestStarletteManualInstrumentation
):
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


class TestAutoInstrumentation(TestStarletteManualInstrumentation):
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

        return self._create_starlette_app()

    def tearDown(self):
        self._instrumentor.uninstrument()
        super().tearDown()

    def test_request(self):
        self._client.get("/foobar")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        for span in spans:
            self.assertEqual(span.resource.attributes["key1"], "value1")
            self.assertEqual(span.resource.attributes["key2"], "value2")


class TestAutoInstrumentationHooks(TestStarletteManualInstrumentationHooks):
    """
    Test the auto-instrumented variant for request and response hooks
    """

    def _create_app(self):
        # instrumentation is handled by the instrument call
        self._instrumentor.instrument(
            server_request_hook=getattr(self, "server_request_hook", None),
            client_request_hook=getattr(self, "client_request_hook", None),
            client_response_hook=getattr(self, "client_response_hook", None),
        )

        return self._create_starlette_app()

    def tearDown(self):
        self._instrumentor.uninstrument()
        super().tearDown()


class TestAutoInstrumentationLogic(unittest.TestCase):
    def test_instrumentation(self):
        """Verify that instrumentation methods are instrumenting and
        removing as expected.
        """
        instrumentor = otel_starlette.StarletteInstrumentor()
        original = applications.Starlette
        instrumentor.instrument()
        try:
            instrumented = applications.Starlette
            self.assertIsNot(original, instrumented)
        finally:
            instrumentor.uninstrument()

        should_be_original = applications.Starlette
        self.assertIs(original, should_be_original)


class TestConditonalServerSpanCreation(TestStarletteManualInstrumentation):
    def test_mark_span_internal_in_presence_of_another_span(self):
        tracer = get_tracer(__name__)
        with tracer.start_as_current_span(
            "test", kind=SpanKind.SERVER
        ) as parent_span:
            self._client.get("/foobar")
            spans = self.sorted_spans(
                self.memory_exporter.get_finished_spans()
            )
            starlette_span = spans[2]
            self.assertEqual(SpanKind.INTERNAL, starlette_span.kind)
            self.assertEqual(SpanKind.SERVER, parent_span.kind)
            self.assertEqual(
                parent_span.context.span_id, starlette_span.parent.span_id
            )


class TestBaseWithCustomHeaders(TestBase):
    def create_app(self):
        app = self.create_starlette_app()
        self._instrumentor.instrument_app(app=app)
        return app

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
        self._instrumentor = otel_starlette.StarletteInstrumentor()
        self._app = self.create_app()
        self._client = TestClient(self._app)

    def tearDown(self) -> None:
        super().tearDown()
        self.env_patch.stop()
        with self.disable_logging():
            self._instrumentor.uninstrument()

    @staticmethod
    def create_starlette_app():
        app = applications.Starlette()

        @app.route("/foobar")
        def _(request):
            return PlainTextResponse(
                content="hi",
                headers={
                    "custom-test-header-1": "test-header-value-1",
                    "custom-test-header-2": "test-header-value-2",
                },
            )

        @app.websocket_route("/foobar_web")
        async def _(websocket: WebSocket) -> None:
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


class TestHTTPAppWithCustomHeaders(TestBaseWithCustomHeaders):
    def test_custom_request_headers_in_span_attributes(self):
        expected = {
            "http.request.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.request.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        resp = self._client.get(
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
            span for span in span_list if span.kind == SpanKind.SERVER
        ][0]

        self.assertSpanHasAttributes(server_span, expected)

    def test_custom_request_headers_not_in_span_attributes(self):
        not_expected = {
            "http.request.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        resp = self._client.get(
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
            span for span in span_list if span.kind == SpanKind.SERVER
        ][0]

        for key in not_expected:
            self.assertNotIn(key, server_span.attributes)

    def test_custom_response_headers_in_span_attributes(self):
        expected = {
            "http.response.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.response.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        resp = self._client.get("/foobar")
        self.assertEqual(200, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 3)

        server_span = [
            span for span in span_list if span.kind == SpanKind.SERVER
        ][0]

        self.assertSpanHasAttributes(server_span, expected)

    def test_custom_response_headers_not_in_span_attributes(self):
        not_expected = {
            "http.response.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        resp = self._client.get("/foobar")
        self.assertEqual(200, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 3)

        server_span = [
            span for span in span_list if span.kind == SpanKind.SERVER
        ][0]

        for key in not_expected:
            self.assertNotIn(key, server_span.attributes)


class TestWebSocketAppWithCustomHeaders(TestBaseWithCustomHeaders):
    def test_custom_request_headers_in_span_attributes(self):
        expected = {
            "http.request.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.request.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        with self._client.websocket_connect(
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
            span for span in span_list if span.kind == SpanKind.SERVER
        ][0]
        self.assertSpanHasAttributes(server_span, expected)

    def test_custom_request_headers_not_in_span_attributes(self):
        not_expected = {
            "http.request.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        with self._client.websocket_connect(
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
            span for span in span_list if span.kind == SpanKind.SERVER
        ][0]

        for key, _ in not_expected.items():
            self.assertNotIn(key, server_span.attributes)

    def test_custom_response_headers_in_span_attributes(self):
        expected = {
            "http.response.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.response.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        with self._client.websocket_connect("/foobar_web") as websocket:
            data = websocket.receive_json()
            self.assertEqual(data, {"message": "hello world"})

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 5)

        server_span = [
            span for span in span_list if span.kind == SpanKind.SERVER
        ][0]

        self.assertSpanHasAttributes(server_span, expected)

    def test_custom_response_headers_not_in_span_attributes(self):
        not_expected = {
            "http.response.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        with self._client.websocket_connect("/foobar_web") as websocket:
            data = websocket.receive_json()
            self.assertEqual(data, {"message": "hello world"})

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 5)

        server_span = [
            span for span in span_list if span.kind == SpanKind.SERVER
        ][0]

        for key, _ in not_expected.items():
            self.assertNotIn(key, server_span.attributes)


class TestNonRecordingSpanWithCustomHeaders(TestBaseWithCustomHeaders):
    def setUp(self):
        super().setUp()
        reset_trace_globals()
        set_tracer_provider(tracer_provider=NoOpTracerProvider())

        self._app = self.create_app()
        self._client = TestClient(self._app)

    def test_custom_header_not_present_in_non_recording_span(self):
        resp = self._client.get(
            "/foobar",
            headers={
                "custom-test-header-1": "test-header-value-1",
            },
        )
        self.assertEqual(200, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)
