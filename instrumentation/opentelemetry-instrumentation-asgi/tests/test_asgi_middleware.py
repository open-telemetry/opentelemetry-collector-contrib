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
import unittest.mock as mock

import opentelemetry.instrumentation.asgi as otel_asgi
from opentelemetry import trace as trace_api
from opentelemetry.test.asgitestutil import (
    AsgiTestBase,
    setup_testing_defaults,
)


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
                "name": "GET asgi.http.receive",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {"type": "http.request"},
            },
            {
                "name": "GET asgi.http.send",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {
                    "http.status_code": 200,
                    "type": "http.response.start",
                },
            },
            {
                "name": "GET asgi.http.send",
                "kind": trace_api.SpanKind.INTERNAL,
                "attributes": {"type": "http.response.body"},
            },
            {
                "name": "GET asgi",
                "kind": trace_api.SpanKind.SERVER,
                "attributes": {
                    "component": "http",
                    "http.method": "GET",
                    "http.scheme": "http",
                    "host.port": 80,
                    "http.host": "127.0.0.1",
                    "http.flavor": "1.0",
                    "http.target": "/",
                    "http.url": "http://127.0.0.1/",
                    "net.peer.ip": "127.0.0.1",
                    "net.peer.port": 32767,
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

    def test_wsgi_not_recording(self):
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
        """Test that span_names can be overwritten by our callback function."""
        span_name = "Dymaxion"

        def get_predefined_span_details(_):
            return span_name, {}

        def update_expected_span_name(expected):
            for entry in expected:
                entry["name"] = " ".join(
                    [span_name] + entry["name"].split(" ")[-1:]
                )
            return expected

        app = otel_asgi.OpenTelemetryMiddleware(
            simple_asgi, span_details_callback=get_predefined_span_details
        )
        self.seed_app(app)
        self.send_default_request()
        outputs = self.get_all_output()
        self.validate_outputs(outputs, modifiers=[update_expected_span_name])

    def test_behavior_with_scope_server_as_none(self):
        """Test that middleware is ok when server is none in scope."""

        def update_expected_server(expected):
            expected[3]["attributes"].update(
                {
                    "http.host": "0.0.0.0",
                    "host.port": 80,
                    "http.url": "http://0.0.0.0/",
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
                {"http.server_name": hostname.decode("utf8")}
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
                {"http.user_agent": user_agent.decode("utf8")}
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
            "/ asgi.websocket.receive",
            "/ asgi.websocket.send",
            "/ asgi.websocket.receive",
            "/ asgi.websocket.send",
            "/ asgi.websocket.receive",
            "/ asgi",
        ]
        actual = [span.name for span in span_list]
        self.assertListEqual(actual, expected)

    def test_lifespan(self):
        self.scope["type"] = "lifespan"
        app = otel_asgi.OpenTelemetryMiddleware(simple_asgi)
        self.seed_app(app)
        self.send_default_request()
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)


class TestAsgiAttributes(unittest.TestCase):
    def setUp(self):
        self.scope = {}
        setup_testing_defaults(self.scope)
        self.span = mock.create_autospec(trace_api.Span, spec_set=True)

    def test_request_attributes(self):
        self.scope["query_string"] = b"foo=bar"

        attrs = otel_asgi.collect_request_attributes(self.scope)
        self.assertDictEqual(
            attrs,
            {
                "component": "http",
                "http.method": "GET",
                "http.host": "127.0.0.1",
                "http.target": "/",
                "http.url": "http://127.0.0.1/?foo=bar",
                "host.port": 80,
                "http.scheme": "http",
                "http.flavor": "1.0",
                "net.peer.ip": "127.0.0.1",
                "net.peer.port": 32767,
            },
        )

    def test_query_string(self):
        self.scope["query_string"] = b"foo=bar"
        attrs = otel_asgi.collect_request_attributes(self.scope)
        self.assertEqual(attrs["http.url"], "http://127.0.0.1/?foo=bar")

    def test_query_string_percent_bytes(self):
        self.scope["query_string"] = b"foo%3Dbar"
        attrs = otel_asgi.collect_request_attributes(self.scope)
        self.assertEqual(attrs["http.url"], "http://127.0.0.1/?foo=bar")

    def test_query_string_percent_str(self):
        self.scope["query_string"] = "foo%3Dbar"
        attrs = otel_asgi.collect_request_attributes(self.scope)
        self.assertEqual(attrs["http.url"], "http://127.0.0.1/?foo=bar")

    def test_response_attributes(self):
        otel_asgi.set_status_code(self.span, 404)
        expected = (mock.call("http.status_code", 404),)
        self.assertEqual(self.span.set_attribute.call_count, 1)
        self.assertEqual(self.span.set_attribute.call_count, 1)
        self.span.set_attribute.assert_has_calls(expected, any_order=True)

    def test_response_attributes_invalid_status_code(self):
        otel_asgi.set_status_code(self.span, "Invalid Status Code")
        self.assertEqual(self.span.set_status.call_count, 1)


if __name__ == "__main__":
    unittest.main()
