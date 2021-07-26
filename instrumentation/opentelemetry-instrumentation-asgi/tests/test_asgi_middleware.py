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

import sys
import unittest
from unittest import mock

import opentelemetry.instrumentation.asgi as otel_asgi
from opentelemetry import trace as trace_api
from opentelemetry.sdk import resources
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.asgitestutil import (
    AsgiTestBase,
    setup_testing_defaults,
)
from opentelemetry.test.test_base import TestBase


async def http_app(scope, receive, send):
    message = await receive()
    assert scope["type"] == "http"
    if message.get("type") == "http.request":
        await send(
            {
                "type": "http.response.start",
                "status": 200,
                "headers": [[b"Content-Type", b"text/plain"]],
            }
        )
        await send({"type": "http.response.body", "body": b"*"})


async def websocket_app(scope, receive, send):
    assert scope["type"] == "websocket"
    while True:
        message = await receive()
        if message.get("type") == "websocket.connect":
            await send({"type": "websocket.accept"})

        if message.get("type") == "websocket.receive":
            if message.get("text") == "ping":
                await send({"type": "websocket.send", "text": "pong"})

        if message.get("type") == "websocket.disconnect":
            break


async def simple_asgi(scope, receive, send):
    assert isinstance(scope, dict)
    if scope["type"] == "http":
        await http_app(scope, receive, send)
    elif scope["type"] == "websocket":
        await websocket_app(scope, receive, send)


async def error_asgi(scope, receive, send):
    assert isinstance(scope, dict)
    assert scope["type"] == "http"
    message = await receive()
    if message.get("type") == "http.request":
        try:
            raise ValueError
        except ValueError:
            scope["hack_exc_info"] = sys.exc_info()
        await send(
            {
                "type": "http.response.start",
                "status": 200,
                "headers": [[b"Content-Type", b"text/plain"]],
            }
        )
        await send({"type": "http.response.body", "body": b"*"})


class TestAsgiApplication(AsgiTestBase):
    def validate_outputs(self, outputs, error=None, modifiers=None):
        # Ensure modifiers is a list
        modifiers = modifiers or []
        # Check for expected outputs
        self.assertEqual(len(outputs), 2)
        response_start = outputs[0]
        response_body = outputs[1]
        self.assertEqual(response_start["type"], "http.response.start")
        self.assertEqual(response_body["type"], "http.response.body")

        # Check http response body
        self.assertEqual(response_body["body"], b"*")

        # Check http response start
        self.assertEqual(response_start["status"], 200)
        self.assertEqual(
            response_start["headers"], [[b"Content-Type", b"text/plain"]]
        )

        exc_info = self.scope.get("hack_exc_info")
        if error:
            self.assertIs(exc_info[0], error)
            self.assertIsInstance(exc_info[1], error)
            self.assertIsNotNone(exc_info[2])
        else:
            self.assertIsNone(exc_info)

        # Check spans
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 4)
        expected = [
            {
                "name": "/ http receive",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {"type": "http.request"},
            },
            {
                "name": "/ http send",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {
                    SpanAttributes.HTTP_STATUS_CODE: 200,
                    "type": "http.response.start",
                },
            },
            {
                "name": "/ http send",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {"type": "http.response.body"},
            },
            {
                "name": "/",
                "kind": trace_api.SpanKind.SERVER,
                "attributes": {
                    SpanAttributes.HTTP_METHOD: "GET",
                    SpanAttributes.HTTP_SCHEME: "http",
                    SpanAttributes.NET_HOST_PORT: 80,
                    SpanAttributes.HTTP_HOST: "127.0.0.1",
                    SpanAttributes.HTTP_FLAVOR: "1.0",
                    SpanAttributes.HTTP_TARGET: "/",
                    SpanAttributes.HTTP_URL: "http://127.0.0.1/",
                    SpanAttributes.NET_PEER_IP: "127.0.0.1",
                    SpanAttributes.NET_PEER_PORT: 32767,
                    SpanAttributes.HTTP_STATUS_CODE: 200,
                },
            },
        ]
        # Run our expected modifiers
        for modifier in modifiers:
            expected = modifier(expected)
        # Check that output matches
        for span, expected in zip(span_list, expected):
            self.assertEqual(span.name, expected["name"])
            self.assertEqual(span.kind, expected["kind"])
            self.assertDictEqual(dict(span.attributes), expected["attributes"])

    def test_basic_asgi_call(self):
        """Test that spans are emitted as expected."""
        app = otel_asgi.OpenTelemetryMiddleware(simple_asgi)
        self.seed_app(app)
        self.send_default_request()
        outputs = self.get_all_output()
        self.validate_outputs(outputs)

    def test_asgi_not_recording(self):
        mock_tracer = mock.Mock()
        mock_span = mock.Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_as_current_span.return_value = mock_span
        mock_tracer.start_as_current_span.return_value.__enter__ = mock_span
        mock_tracer.start_as_current_span.return_value.__exit__ = mock_span
        with mock.patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            app = otel_asgi.OpenTelemetryMiddleware(simple_asgi)
            self.seed_app(app)
            self.send_default_request()
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_asgi_exc_info(self):
        """Test that exception information is emitted as expected."""
        app = otel_asgi.OpenTelemetryMiddleware(error_asgi)
        self.seed_app(app)
        self.send_default_request()
        outputs = self.get_all_output()
        self.validate_outputs(outputs, error=ValueError)

    def test_override_span_name(self):
        """Test that default span_names can be overwritten by our callback function."""
        span_name = "Dymaxion"

        def get_predefined_span_details(_):
            return span_name, {}

        def update_expected_span_name(expected):
            for entry in expected:
                if entry["kind"] == trace_api.SpanKind.SERVER:
                    entry["name"] = span_name
                else:
                    entry["name"] = " ".join(
                        [span_name] + entry["name"].split(" ")[1:]
                    )
            return expected

        app = otel_asgi.OpenTelemetryMiddleware(
            simple_asgi, default_span_details=get_predefined_span_details
        )
        self.seed_app(app)
        self.send_default_request()
        outputs = self.get_all_output()
        self.validate_outputs(outputs, modifiers=[update_expected_span_name])

    def test_custom_tracer_provider_otel_asgi(self):
        resource = resources.Resource.create({"service-test-key": "value"})
        result = TestBase.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result

        app = otel_asgi.OpenTelemetryMiddleware(
            simple_asgi, tracer_provider=tracer_provider
        )
        self.seed_app(app)
        self.send_default_request()
        span_list = exporter.get_finished_spans()
        for span in span_list:
            self.assertEqual(
                span.resource.attributes["service-test-key"], "value"
            )

    def test_behavior_with_scope_server_as_none(self):
        """Test that middleware is ok when server is none in scope."""

        def update_expected_server(expected):
            expected[3]["attributes"].update(
                {
                    SpanAttributes.HTTP_HOST: "0.0.0.0",
                    SpanAttributes.NET_HOST_PORT: 80,
                    SpanAttributes.HTTP_URL: "http://0.0.0.0/",
                }
            )
            return expected

        self.scope["server"] = None
        app = otel_asgi.OpenTelemetryMiddleware(simple_asgi)
        self.seed_app(app)
        self.send_default_request()
        outputs = self.get_all_output()
        self.validate_outputs(outputs, modifiers=[update_expected_server])

    def test_host_header(self):
        """Test that host header is converted to http.server_name."""
        hostname = b"server_name_1"

        def update_expected_server(expected):
            expected[3]["attributes"].update(
                {SpanAttributes.HTTP_SERVER_NAME: hostname.decode("utf8")}
            )
            return expected

        self.scope["headers"].append([b"host", hostname])
        app = otel_asgi.OpenTelemetryMiddleware(simple_asgi)
        self.seed_app(app)
        self.send_default_request()
        outputs = self.get_all_output()
        self.validate_outputs(outputs, modifiers=[update_expected_server])

    def test_user_agent(self):
        """Test that host header is converted to http.server_name."""
        user_agent = b"test-agent"

        def update_expected_user_agent(expected):
            expected[3]["attributes"].update(
                {SpanAttributes.HTTP_USER_AGENT: user_agent.decode("utf8")}
            )
            return expected

        self.scope["headers"].append([b"user-agent", user_agent])
        app = otel_asgi.OpenTelemetryMiddleware(simple_asgi)
        self.seed_app(app)
        self.send_default_request()
        outputs = self.get_all_output()
        self.validate_outputs(outputs, modifiers=[update_expected_user_agent])

    def test_websocket(self):
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
        app = otel_asgi.OpenTelemetryMiddleware(simple_asgi)
        self.seed_app(app)
        self.send_input({"type": "websocket.connect"})
        self.send_input({"type": "websocket.receive", "text": "ping"})
        self.send_input({"type": "websocket.disconnect"})
        self.get_all_output()
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 6)
        expected = [
            {
                "name": "/ websocket receive",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {"type": "websocket.connect"},
            },
            {
                "name": "/ websocket send",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {"type": "websocket.accept"},
            },
            {
                "name": "/ websocket receive",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {
                    "type": "websocket.receive",
                    SpanAttributes.HTTP_STATUS_CODE: 200,
                },
            },
            {
                "name": "/ websocket send",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {
                    "type": "websocket.send",
                    SpanAttributes.HTTP_STATUS_CODE: 200,
                },
            },
            {
                "name": "/ websocket receive",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {"type": "websocket.disconnect"},
            },
            {
                "name": "/",
                "kind": trace_api.SpanKind.SERVER,
                "attributes": {
                    SpanAttributes.HTTP_SCHEME: self.scope["scheme"],
                    SpanAttributes.NET_HOST_PORT: self.scope["server"][1],
                    SpanAttributes.HTTP_HOST: self.scope["server"][0],
                    SpanAttributes.HTTP_FLAVOR: self.scope["http_version"],
                    SpanAttributes.HTTP_TARGET: self.scope["path"],
                    SpanAttributes.HTTP_URL: f'{self.scope["scheme"]}://{self.scope["server"][0]}{self.scope["path"]}',
                    SpanAttributes.NET_PEER_IP: self.scope["client"][0],
                    SpanAttributes.NET_PEER_PORT: self.scope["client"][1],
                    SpanAttributes.HTTP_STATUS_CODE: 200,
                },
            },
        ]
        for span, expected in zip(span_list, expected):
            self.assertEqual(span.name, expected["name"])
            self.assertEqual(span.kind, expected["kind"])
            self.assertDictEqual(dict(span.attributes), expected["attributes"])

    def test_lifespan(self):
        self.scope["type"] = "lifespan"
        app = otel_asgi.OpenTelemetryMiddleware(simple_asgi)
        self.seed_app(app)
        self.send_default_request()
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)

    def test_hooks(self):
        def server_request_hook(span, scope):
            span.update_name("name from server hook")

        def client_request_hook(recieve_span, request):
            recieve_span.update_name("name from client request hook")

        def client_response_hook(send_span, response):
            send_span.set_attribute("attr-from-hook", "value")

        def update_expected_hook_results(expected):
            for entry in expected:
                if entry["kind"] == trace_api.SpanKind.SERVER:
                    entry["name"] = "name from server hook"
                elif entry["name"] == "/ http receive":
                    entry["name"] = "name from client request hook"
                elif entry["name"] == "/ http send":
                    entry["attributes"].update({"attr-from-hook": "value"})
            return expected

        app = otel_asgi.OpenTelemetryMiddleware(
            simple_asgi,
            server_request_hook=server_request_hook,
            client_request_hook=client_request_hook,
            client_response_hook=client_response_hook,
        )
        self.seed_app(app)
        self.send_default_request()
        outputs = self.get_all_output()
        self.validate_outputs(
            outputs, modifiers=[update_expected_hook_results]
        )


class TestAsgiAttributes(unittest.TestCase):
    def setUp(self):
        self.scope = {}
        setup_testing_defaults(self.scope)
        self.span = mock.create_autospec(trace_api.Span, spec_set=True)

    def test_request_attributes(self):
        self.scope["query_string"] = b"foo=bar"
        headers = []
        headers.append(("host".encode("utf8"), "test".encode("utf8")))
        self.scope["headers"] = headers

        attrs = otel_asgi.collect_request_attributes(self.scope)

        self.assertDictEqual(
            attrs,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_HOST: "127.0.0.1",
                SpanAttributes.HTTP_TARGET: "/",
                SpanAttributes.HTTP_URL: "http://127.0.0.1/?foo=bar",
                SpanAttributes.NET_HOST_PORT: 80,
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.HTTP_SERVER_NAME: "test",
                SpanAttributes.HTTP_FLAVOR: "1.0",
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.NET_PEER_PORT: 32767,
            },
        )

    def test_query_string(self):
        self.scope["query_string"] = b"foo=bar"
        attrs = otel_asgi.collect_request_attributes(self.scope)
        self.assertEqual(
            attrs[SpanAttributes.HTTP_URL], "http://127.0.0.1/?foo=bar"
        )

    def test_query_string_percent_bytes(self):
        self.scope["query_string"] = b"foo%3Dbar"
        attrs = otel_asgi.collect_request_attributes(self.scope)
        self.assertEqual(
            attrs[SpanAttributes.HTTP_URL], "http://127.0.0.1/?foo=bar"
        )

    def test_query_string_percent_str(self):
        self.scope["query_string"] = "foo%3Dbar"
        attrs = otel_asgi.collect_request_attributes(self.scope)
        self.assertEqual(
            attrs[SpanAttributes.HTTP_URL], "http://127.0.0.1/?foo=bar"
        )

    def test_response_attributes(self):
        otel_asgi.set_status_code(self.span, 404)
        expected = (mock.call(SpanAttributes.HTTP_STATUS_CODE, 404),)
        self.assertEqual(self.span.set_attribute.call_count, 1)
        self.assertEqual(self.span.set_attribute.call_count, 1)
        self.span.set_attribute.assert_has_calls(expected, any_order=True)

    def test_response_attributes_invalid_status_code(self):
        otel_asgi.set_status_code(self.span, "Invalid Status Code")
        self.assertEqual(self.span.set_status.call_count, 1)

    def test_credential_removal(self):
        self.scope["server"] = ("username:password@httpbin.org", 80)
        self.scope["path"] = "/status/200"
        attrs = otel_asgi.collect_request_attributes(self.scope)
        self.assertEqual(
            attrs[SpanAttributes.HTTP_URL], "http://httpbin.org/status/200"
        )


if __name__ == "__main__":
    unittest.main()
