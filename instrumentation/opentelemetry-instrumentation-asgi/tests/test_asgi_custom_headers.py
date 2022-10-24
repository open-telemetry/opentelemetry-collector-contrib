from unittest import mock

import opentelemetry.instrumentation.asgi as otel_asgi
from opentelemetry.test.asgitestutil import AsgiTestBase
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import SpanKind
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SANITIZE_FIELDS,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
)

from .test_asgi_middleware import simple_asgi


async def http_app_with_custom_headers(scope, receive, send):
    message = await receive()
    assert scope["type"] == "http"
    if message.get("type") == "http.request":
        await send(
            {
                "type": "http.response.start",
                "status": 200,
                "headers": [
                    (b"Content-Type", b"text/plain"),
                    (b"custom-test-header-1", b"test-header-value-1"),
                    (b"custom-test-header-2", b"test-header-value-2"),
                    (
                        b"my-custom-regex-header-1",
                        b"my-custom-regex-value-1,my-custom-regex-value-2",
                    ),
                    (
                        b"My-Custom-Regex-Header-2",
                        b"my-custom-regex-value-3,my-custom-regex-value-4",
                    ),
                    (b"my-secret-header", b"my-secret-value"),
                ],
            }
        )
    await send({"type": "http.response.body", "body": b"*"})


async def websocket_app_with_custom_headers(scope, receive, send):
    assert scope["type"] == "websocket"
    while True:
        message = await receive()
        if message.get("type") == "websocket.connect":
            await send(
                {
                    "type": "websocket.accept",
                    "headers": [
                        (b"custom-test-header-1", b"test-header-value-1"),
                        (b"custom-test-header-2", b"test-header-value-2"),
                        (
                            b"my-custom-regex-header-1",
                            b"my-custom-regex-value-1,my-custom-regex-value-2",
                        ),
                        (
                            b"My-Custom-Regex-Header-2",
                            b"my-custom-regex-value-3,my-custom-regex-value-4",
                        ),
                        (b"my-secret-header", b"my-secret-value"),
                    ],
                }
            )

        if message.get("type") == "websocket.receive":
            if message.get("text") == "ping":
                await send({"type": "websocket.send", "text": "pong"})

        if message.get("type") == "websocket.disconnect":
            break


@mock.patch.dict(
    "os.environ",
    {
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SANITIZE_FIELDS: ".*my-secret.*",
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3,Regex-Test-Header-.*,Regex-Invalid-Test-Header-.*,.*my-secret.*",
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3,my-custom-regex-header-.*,invalid-regex-header-.*,.*my-secret.*",
    },
)
class TestCustomHeaders(AsgiTestBase, TestBase):
    def setUp(self):
        super().setUp()
        self.tracer_provider, self.exporter = TestBase.create_tracer_provider()
        self.tracer = self.tracer_provider.get_tracer(__name__)
        self.app = otel_asgi.OpenTelemetryMiddleware(
            simple_asgi, tracer_provider=self.tracer_provider
        )

    def test_http_custom_request_headers_in_span_attributes(self):
        self.scope["headers"].extend(
            [
                (b"custom-test-header-1", b"test-header-value-1"),
                (b"custom-test-header-2", b"test-header-value-2"),
                (b"Regex-Test-Header-1", b"Regex Test Value 1"),
                (b"regex-test-header-2", b"RegexTestValue2,RegexTestValue3"),
                (b"My-Secret-Header", b"My Secret Value"),
            ]
        )
        self.seed_app(self.app)
        self.send_default_request()
        self.get_all_output()
        span_list = self.exporter.get_finished_spans()
        expected = {
            "http.request.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.request.header.custom_test_header_2": (
                "test-header-value-2",
            ),
            "http.request.header.regex_test_header_1": ("Regex Test Value 1",),
            "http.request.header.regex_test_header_2": (
                "RegexTestValue2,RegexTestValue3",
            ),
            "http.request.header.my_secret_header": ("[REDACTED]",),
        }
        for span in span_list:
            if span.kind == SpanKind.SERVER:
                self.assertSpanHasAttributes(span, expected)

    def test_http_custom_request_headers_not_in_span_attributes(self):
        self.scope["headers"].extend(
            [
                (b"custom-test-header-1", b"test-header-value-1"),
            ]
        )
        self.seed_app(self.app)
        self.send_default_request()
        self.get_all_output()
        span_list = self.exporter.get_finished_spans()
        expected = {
            "http.request.header.custom_test_header_1": (
                "test-header-value-1",
            ),
        }
        not_expected = {
            "http.request.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        for span in span_list:
            if span.kind == SpanKind.SERVER:
                self.assertSpanHasAttributes(span, expected)
                for key, _ in not_expected.items():
                    self.assertNotIn(key, span.attributes)

    def test_http_custom_response_headers_in_span_attributes(self):
        self.app = otel_asgi.OpenTelemetryMiddleware(
            http_app_with_custom_headers, tracer_provider=self.tracer_provider
        )
        self.seed_app(self.app)
        self.send_default_request()
        self.get_all_output()
        span_list = self.exporter.get_finished_spans()
        expected = {
            "http.response.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.response.header.custom_test_header_2": (
                "test-header-value-2",
            ),
            "http.response.header.my_custom_regex_header_1": (
                "my-custom-regex-value-1,my-custom-regex-value-2",
            ),
            "http.response.header.my_custom_regex_header_2": (
                "my-custom-regex-value-3,my-custom-regex-value-4",
            ),
            "http.response.header.my_secret_header": ("[REDACTED]",),
        }
        for span in span_list:
            if span.kind == SpanKind.SERVER:
                self.assertSpanHasAttributes(span, expected)

    def test_http_custom_response_headers_not_in_span_attributes(self):
        self.app = otel_asgi.OpenTelemetryMiddleware(
            http_app_with_custom_headers, tracer_provider=self.tracer_provider
        )
        self.seed_app(self.app)
        self.send_default_request()
        self.get_all_output()
        span_list = self.exporter.get_finished_spans()
        not_expected = {
            "http.response.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        for span in span_list:
            if span.kind == SpanKind.SERVER:
                for key, _ in not_expected.items():
                    self.assertNotIn(key, span.attributes)

    def test_websocket_custom_request_headers_in_span_attributes(self):
        self.scope = {
            "type": "websocket",
            "http_version": "1.1",
            "scheme": "ws",
            "path": "/",
            "query_string": b"",
            "headers": [
                (b"custom-test-header-1", b"test-header-value-1"),
                (b"custom-test-header-2", b"test-header-value-2"),
                (b"Regex-Test-Header-1", b"Regex Test Value 1"),
                (b"regex-test-header-2", b"RegexTestValue2,RegexTestValue3"),
                (b"My-Secret-Header", b"My Secret Value"),
            ],
            "client": ("127.0.0.1", 32767),
            "server": ("127.0.0.1", 80),
        }
        self.seed_app(self.app)
        self.send_input({"type": "websocket.connect"})
        self.send_input({"type": "websocket.receive", "text": "ping"})
        self.send_input({"type": "websocket.disconnect"})

        self.get_all_output()
        span_list = self.exporter.get_finished_spans()
        expected = {
            "http.request.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.request.header.custom_test_header_2": (
                "test-header-value-2",
            ),
            "http.request.header.regex_test_header_1": ("Regex Test Value 1",),
            "http.request.header.regex_test_header_2": (
                "RegexTestValue2,RegexTestValue3",
            ),
            "http.request.header.my_secret_header": ("[REDACTED]",),
        }
        for span in span_list:
            if span.kind == SpanKind.SERVER:
                self.assertSpanHasAttributes(span, expected)

    def test_websocket_custom_request_headers_not_in_span_attributes(self):
        self.scope = {
            "type": "websocket",
            "http_version": "1.1",
            "scheme": "ws",
            "path": "/",
            "query_string": b"",
            "headers": [
                (b"Custom-Test-Header-1", b"test-header-value-1"),
                (b"Custom-Test-Header-2", b"test-header-value-2"),
            ],
            "client": ("127.0.0.1", 32767),
            "server": ("127.0.0.1", 80),
        }
        self.seed_app(self.app)
        self.send_input({"type": "websocket.connect"})
        self.send_input({"type": "websocket.receive", "text": "ping"})
        self.send_input({"type": "websocket.disconnect"})

        self.get_all_output()
        span_list = self.exporter.get_finished_spans()
        not_expected = {
            "http.request.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        for span in span_list:
            if span.kind == SpanKind.SERVER:
                for key, _ in not_expected.items():
                    self.assertNotIn(key, span.attributes)

    def test_websocket_custom_response_headers_in_span_attributes(self):
        self.scope = {
            "type": "websocket",
            "http_version": "1.1",
            "scheme": "ws",
            "path": "/",
            "query_string": b"",
            "headers": [],
            "client": ("127.0.0.1", 32767),
            "server": ("127.0.0.1", 80),
        }
        self.app = otel_asgi.OpenTelemetryMiddleware(
            websocket_app_with_custom_headers,
            tracer_provider=self.tracer_provider,
        )
        self.seed_app(self.app)
        self.send_input({"type": "websocket.connect"})
        self.send_input({"type": "websocket.receive", "text": "ping"})
        self.send_input({"type": "websocket.disconnect"})
        self.get_all_output()
        span_list = self.exporter.get_finished_spans()
        expected = {
            "http.response.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.response.header.custom_test_header_2": (
                "test-header-value-2",
            ),
            "http.response.header.my_custom_regex_header_1": (
                "my-custom-regex-value-1,my-custom-regex-value-2",
            ),
            "http.response.header.my_custom_regex_header_2": (
                "my-custom-regex-value-3,my-custom-regex-value-4",
            ),
            "http.response.header.my_secret_header": ("[REDACTED]",),
        }
        for span in span_list:
            if span.kind == SpanKind.SERVER:
                self.assertSpanHasAttributes(span, expected)

    def test_websocket_custom_response_headers_not_in_span_attributes(self):
        self.scope = {
            "type": "websocket",
            "http_version": "1.1",
            "scheme": "ws",
            "path": "/",
            "query_string": b"",
            "headers": [],
            "client": ("127.0.0.1", 32767),
            "server": ("127.0.0.1", 80),
        }
        self.app = otel_asgi.OpenTelemetryMiddleware(
            websocket_app_with_custom_headers,
            tracer_provider=self.tracer_provider,
        )
        self.seed_app(self.app)
        self.send_input({"type": "websocket.connect"})
        self.send_input({"type": "websocket.receive", "text": "ping"})
        self.send_input({"type": "websocket.disconnect"})
        self.get_all_output()
        span_list = self.exporter.get_finished_spans()
        not_expected = {
            "http.response.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        for span in span_list:
            if span.kind == SpanKind.SERVER:
                for key, _ in not_expected.items():
                    self.assertNotIn(key, span.attributes)
