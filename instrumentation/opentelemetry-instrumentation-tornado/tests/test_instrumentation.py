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

from tornado.testing import AsyncHTTPTestCase

from opentelemetry import trace
from opentelemetry.instrumentation.propagators import (
    TraceResponsePropagator,
    get_global_response_propagator,
    set_global_response_propagator,
)
from opentelemetry.instrumentation.tornado import (
    TornadoInstrumentor,
    patch_handler_class,
    unpatch_handler_class,
)
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import SpanKind
from opentelemetry.util.http import get_excluded_urls, get_traced_request_attrs

from .tornado_test_app import (
    AsyncHandler,
    DynamicHandler,
    MainHandler,
    make_app,
)


class TornadoTest(AsyncHTTPTestCase, TestBase):
    # pylint:disable=no-self-use
    def get_app(self):
        tracer = trace.get_tracer(__name__)
        app = make_app(tracer)
        return app

    def setUp(self):
        TornadoInstrumentor().instrument(
            server_request_hook=getattr(self, "server_request_hook", None),
            client_request_hook=getattr(self, "client_request_hook", None),
            client_response_hook=getattr(self, "client_response_hook", None),
        )
        super().setUp()
        # pylint: disable=protected-access
        self.env_patch = patch.dict(
            "os.environ",
            {
                "OTEL_PYTHON_TORNADO_EXCLUDED_URLS": "healthz,ping",
                "OTEL_PYTHON_TORNADO_TRACED_REQUEST_ATTRS": "uri,full_url,query",
            },
        )
        self.env_patch.start()
        self.exclude_patch = patch(
            "opentelemetry.instrumentation.tornado._excluded_urls",
            get_excluded_urls("TORNADO"),
        )
        self.traced_patch = patch(
            "opentelemetry.instrumentation.tornado._traced_request_attrs",
            get_traced_request_attrs("TORNADO"),
        )
        self.exclude_patch.start()
        self.traced_patch.start()

    def tearDown(self):
        TornadoInstrumentor().uninstrument()
        self.env_patch.stop()
        self.exclude_patch.stop()
        self.traced_patch.stop()
        super().tearDown()


class TestTornadoInstrumentor(TornadoTest):
    def test_patch_references(self):
        self.assertEqual(len(TornadoInstrumentor().patched_handlers), 0)

        self.fetch("/")
        self.fetch("/async")
        self.assertEqual(
            TornadoInstrumentor().patched_handlers, [MainHandler, AsyncHandler]
        )

        self.fetch("/async")
        self.fetch("/")
        self.assertEqual(
            TornadoInstrumentor().patched_handlers, [MainHandler, AsyncHandler]
        )

        TornadoInstrumentor().uninstrument()
        self.assertEqual(TornadoInstrumentor().patched_handlers, [])

    def test_patch_applied_only_once(self):
        tracer = trace.get_tracer(__name__)
        self.assertTrue(patch_handler_class(tracer, AsyncHandler))
        self.assertFalse(patch_handler_class(tracer, AsyncHandler))
        self.assertFalse(patch_handler_class(tracer, AsyncHandler))
        unpatch_handler_class(AsyncHandler)


class TestTornadoInstrumentation(TornadoTest):
    def test_http_calls(self):
        methods = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"]
        for method in methods:
            self._test_http_method_call(method)

    def _test_http_method_call(self, method):
        body = "" if method in ["POST", "PUT", "PATCH"] else None
        response = self.fetch("/", method=method, body=body)
        self.assertEqual(response.code, 201)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)

        manual, server, client = self.sorted_spans(spans)

        self.assertEqual(manual.name, "manual")
        self.assertEqual(manual.parent, server.context)
        self.assertEqual(manual.context.trace_id, client.context.trace_id)

        self.assertEqual(server.name, "MainHandler." + method.lower())
        self.assertTrue(server.parent.is_remote)
        self.assertNotEqual(server.parent, client.context)
        self.assertEqual(server.parent.span_id, client.context.span_id)
        self.assertEqual(server.context.trace_id, client.context.trace_id)
        self.assertEqual(server.kind, SpanKind.SERVER)
        self.assert_span_has_attributes(
            server,
            {
                SpanAttributes.HTTP_METHOD: method,
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.HTTP_HOST: "127.0.0.1:"
                + str(self.get_http_port()),
                SpanAttributes.HTTP_TARGET: "/",
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.HTTP_STATUS_CODE: 201,
            },
        )

        self.assertEqual(client.name, method)
        self.assertFalse(client.context.is_remote)
        self.assertIsNone(client.parent)
        self.assertEqual(client.kind, SpanKind.CLIENT)
        self.assert_span_has_attributes(
            client,
            {
                SpanAttributes.HTTP_URL: self.get_url("/"),
                SpanAttributes.HTTP_METHOD: method,
                SpanAttributes.HTTP_STATUS_CODE: 201,
            },
        )

        self.memory_exporter.clear()

    def test_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            self.fetch("/")
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_async_handler(self):
        self._test_async_handler("/async", "AsyncHandler")

    def test_coroutine_handler(self):
        self._test_async_handler("/cor", "CoroutineHandler")

    def _test_async_handler(self, url, handler_name):
        response = self.fetch(url)
        self.assertEqual(response.code, 201)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 5)

        sub2, sub1, sub_wrapper, server, client = self.sorted_spans(spans)

        self.assertEqual(sub2.name, "sub-task-2")
        self.assertEqual(sub2.parent, sub_wrapper.context)
        self.assertEqual(sub2.context.trace_id, client.context.trace_id)

        self.assertEqual(sub1.name, "sub-task-1")
        self.assertEqual(sub1.parent, sub_wrapper.context)
        self.assertEqual(sub1.context.trace_id, client.context.trace_id)

        self.assertEqual(sub_wrapper.name, "sub-task-wrapper")
        self.assertEqual(sub_wrapper.parent, server.context)
        self.assertEqual(sub_wrapper.context.trace_id, client.context.trace_id)

        self.assertEqual(server.name, handler_name + ".get")
        self.assertTrue(server.parent.is_remote)
        self.assertNotEqual(server.parent, client.context)
        self.assertEqual(server.parent.span_id, client.context.span_id)
        self.assertEqual(server.context.trace_id, client.context.trace_id)
        self.assertEqual(server.kind, SpanKind.SERVER)
        self.assert_span_has_attributes(
            server,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.HTTP_HOST: "127.0.0.1:"
                + str(self.get_http_port()),
                SpanAttributes.HTTP_TARGET: url,
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.HTTP_STATUS_CODE: 201,
            },
        )

        self.assertEqual(client.name, "GET")
        self.assertFalse(client.context.is_remote)
        self.assertIsNone(client.parent)
        self.assertEqual(client.kind, SpanKind.CLIENT)
        self.assert_span_has_attributes(
            client,
            {
                SpanAttributes.HTTP_URL: self.get_url(url),
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_STATUS_CODE: 201,
            },
        )

    def test_500(self):
        response = self.fetch("/error")
        self.assertEqual(response.code, 500)

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 2)
        server, client = spans

        self.assertEqual(server.name, "BadHandler.get")
        self.assertEqual(server.kind, SpanKind.SERVER)
        self.assert_span_has_attributes(
            server,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.HTTP_HOST: "127.0.0.1:"
                + str(self.get_http_port()),
                SpanAttributes.HTTP_TARGET: "/error",
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.HTTP_STATUS_CODE: 500,
            },
        )

        self.assertEqual(client.name, "GET")
        self.assertEqual(client.kind, SpanKind.CLIENT)
        self.assert_span_has_attributes(
            client,
            {
                SpanAttributes.HTTP_URL: self.get_url("/error"),
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_STATUS_CODE: 500,
            },
        )

    def test_404(self):
        response = self.fetch("/missing-url")
        self.assertEqual(response.code, 404)

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 2)
        server, client = spans

        self.assertEqual(server.name, "ErrorHandler.get")
        self.assertEqual(server.kind, SpanKind.SERVER)
        self.assert_span_has_attributes(
            server,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.HTTP_HOST: "127.0.0.1:"
                + str(self.get_http_port()),
                SpanAttributes.HTTP_TARGET: "/missing-url",
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.HTTP_STATUS_CODE: 404,
            },
        )

        self.assertEqual(client.name, "GET")
        self.assertEqual(client.kind, SpanKind.CLIENT)
        self.assert_span_has_attributes(
            client,
            {
                SpanAttributes.HTTP_URL: self.get_url("/missing-url"),
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_STATUS_CODE: 404,
            },
        )

    def test_dynamic_handler(self):
        response = self.fetch("/dyna")
        self.assertEqual(response.code, 404)
        self.memory_exporter.clear()

        self._app.add_handlers(r".+", [(r"/dyna", DynamicHandler)])

        response = self.fetch("/dyna")
        self.assertEqual(response.code, 202)

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 2)
        server, client = spans

        self.assertEqual(server.name, "DynamicHandler.get")
        self.assertTrue(server.parent.is_remote)
        self.assertNotEqual(server.parent, client.context)
        self.assertEqual(server.parent.span_id, client.context.span_id)
        self.assertEqual(server.context.trace_id, client.context.trace_id)
        self.assertEqual(server.kind, SpanKind.SERVER)
        self.assert_span_has_attributes(
            server,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.HTTP_HOST: "127.0.0.1:"
                + str(self.get_http_port()),
                SpanAttributes.HTTP_TARGET: "/dyna",
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.HTTP_STATUS_CODE: 202,
            },
        )

        self.assertEqual(client.name, "GET")
        self.assertFalse(client.context.is_remote)
        self.assertIsNone(client.parent)
        self.assertEqual(client.kind, SpanKind.CLIENT)
        self.assert_span_has_attributes(
            client,
            {
                SpanAttributes.HTTP_URL: self.get_url("/dyna"),
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_STATUS_CODE: 202,
            },
        )

    def test_handler_on_finish(self):

        response = self.fetch("/on_finish")
        self.assertEqual(response.code, 200)

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 3)
        auditor, server, client = spans

        self.assertEqual(server.name, "FinishedHandler.get")
        self.assertTrue(server.parent.is_remote)
        self.assertNotEqual(server.parent, client.context)
        self.assertEqual(server.parent.span_id, client.context.span_id)
        self.assertEqual(server.context.trace_id, client.context.trace_id)
        self.assertEqual(server.kind, SpanKind.SERVER)
        self.assert_span_has_attributes(
            server,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.HTTP_HOST: "127.0.0.1:"
                + str(self.get_http_port()),
                SpanAttributes.HTTP_TARGET: "/on_finish",
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

        self.assertEqual(client.name, "GET")
        self.assertFalse(client.context.is_remote)
        self.assertIsNone(client.parent)
        self.assertEqual(client.kind, SpanKind.CLIENT)
        self.assert_span_has_attributes(
            client,
            {
                SpanAttributes.HTTP_URL: self.get_url("/on_finish"),
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

        self.assertEqual(auditor.name, "audit_task")
        self.assertFalse(auditor.context.is_remote)
        self.assertEqual(auditor.parent.span_id, server.context.span_id)
        self.assertEqual(auditor.context.trace_id, client.context.trace_id)

        self.assertEqual(auditor.kind, SpanKind.INTERNAL)

    def test_exclude_lists(self):
        def test_excluded(path):
            self.fetch(path)
            spans = self.sorted_spans(
                self.memory_exporter.get_finished_spans()
            )
            self.assertEqual(len(spans), 1)
            client = spans[0]
            self.assertEqual(client.name, "GET")
            self.assertEqual(client.kind, SpanKind.CLIENT)
            self.assert_span_has_attributes(
                client,
                {
                    SpanAttributes.HTTP_URL: self.get_url(path),
                    SpanAttributes.HTTP_METHOD: "GET",
                    SpanAttributes.HTTP_STATUS_CODE: 200,
                },
            )
            self.memory_exporter.clear()

        test_excluded("/healthz")
        test_excluded("/ping")

    def test_traced_attrs(self):
        self.fetch("/pong?q=abc&b=123")
        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 2)
        server_span = spans[0]
        self.assertEqual(server_span.kind, SpanKind.SERVER)
        self.assert_span_has_attributes(
            server_span, {"uri": "/pong?q=abc&b=123", "query": "q=abc&b=123"}
        )
        self.memory_exporter.clear()

    def test_response_headers(self):
        orig = get_global_response_propagator()
        set_global_response_propagator(TraceResponsePropagator())

        response = self.fetch("/")
        headers = response.headers

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 3)
        server_span = spans[1]

        self.assertIn("traceresponse", headers)
        self.assertEqual(
            headers["access-control-expose-headers"], "traceresponse",
        )
        self.assertEqual(
            headers["traceresponse"],
            "00-{0}-{1}-01".format(
                trace.format_trace_id(server_span.get_span_context().trace_id),
                trace.format_span_id(server_span.get_span_context().span_id),
            ),
        )

        self.memory_exporter.clear()
        set_global_response_propagator(orig)

    def test_credential_removal(self):
        response = self.fetch(
            "http://username:password@httpbin.org/status/200"
        )
        self.assertEqual(response.code, 200)

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 1)
        client = spans[0]

        self.assertEqual(client.name, "GET")
        self.assertEqual(client.kind, SpanKind.CLIENT)
        self.assert_span_has_attributes(
            client,
            {
                SpanAttributes.HTTP_URL: "http://httpbin.org/status/200",
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

        self.memory_exporter.clear()


class TornadoHookTest(TornadoTest):
    _client_request_hook = None
    _client_response_hook = None
    _server_request_hook = None

    def client_request_hook(self, span, handler):
        if self._client_request_hook is not None:
            self._client_request_hook(span, handler)

    def client_response_hook(self, span, handler):
        if self._client_response_hook is not None:
            self._client_response_hook(span, handler)

    def server_request_hook(self, span, handler):
        if self._server_request_hook is not None:
            self._server_request_hook(span, handler)

    def test_hooks(self):
        def server_request_hook(span, handler):
            span.update_name("name from server hook")
            handler.set_header("hello", "world")

        def client_request_hook(span, request):
            span.update_name("name from client hook")

        def client_response_hook(span, response):
            span.set_attribute("attr-from-hook", "value")

        self._server_request_hook = server_request_hook
        self._client_request_hook = client_request_hook
        self._client_response_hook = client_response_hook

        response = self.fetch("/")
        self.assertEqual(response.headers.get("hello"), "world")

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 3)
        server_span = spans[1]
        self.assertEqual(server_span.kind, SpanKind.SERVER)
        self.assertEqual(server_span.name, "name from server hook")
        self.assert_span_has_attributes(server_span, {"uri": "/"})
        self.memory_exporter.clear()

        client_span = spans[2]
        self.assertEqual(client_span.kind, SpanKind.CLIENT)
        self.assertEqual(client_span.name, "name from client hook")
        self.assert_span_has_attributes(
            client_span, {"attr-from-hook": "value"}
        )

        self.memory_exporter.clear()


class TestTornadoUninstrument(TornadoTest):
    def test_uninstrument(self):
        response = self.fetch("/")
        self.assertEqual(response.code, 201)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        manual, server, client = self.sorted_spans(spans)
        self.assertEqual(manual.name, "manual")
        self.assertEqual(server.name, "MainHandler.get")
        self.assertEqual(client.name, "GET")
        self.memory_exporter.clear()

        TornadoInstrumentor().uninstrument()

        response = self.fetch("/")
        self.assertEqual(response.code, 201)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        manual = spans[0]
        self.assertEqual(manual.name, "manual")
