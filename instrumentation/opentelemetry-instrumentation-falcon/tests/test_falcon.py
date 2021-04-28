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

from falcon import testing

from opentelemetry.instrumentation.falcon import FalconInstrumentor
from opentelemetry.instrumentation.propagators import (
    TraceResponsePropagator,
    get_global_response_propagator,
    set_global_response_propagator,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import StatusCode, format_span_id, format_trace_id
from opentelemetry.util.http import get_excluded_urls, get_traced_request_attrs

from .app import make_app


class TestFalconBase(TestBase):
    def setUp(self):
        super().setUp()
        FalconInstrumentor().instrument(
            request_hook=getattr(self, "request_hook", None),
            response_hook=getattr(self, "response_hook", None),
        )
        self.app = make_app()
        # pylint: disable=protected-access
        self.env_patch = patch.dict(
            "os.environ",
            {
                "OTEL_PYTHON_FALCON_EXCLUDED_URLS": "ping",
                "OTEL_PYTHON_FALCON_TRACED_REQUEST_ATTRS": "query_string",
            },
        )
        self.env_patch.start()
        self.exclude_patch = patch(
            "opentelemetry.instrumentation.falcon._excluded_urls",
            get_excluded_urls("FALCON"),
        )
        middleware = self.app._middleware[0][  # pylint:disable=W0212
            0
        ].__self__
        self.traced_patch = patch.object(
            middleware,
            "_traced_request_attrs",
            get_traced_request_attrs("FALCON"),
        )
        self.exclude_patch.start()
        self.traced_patch.start()

    def client(self):
        return testing.TestClient(self.app)

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FalconInstrumentor().uninstrument()
        self.env_patch.stop()
        self.exclude_patch.stop()
        self.traced_patch.stop()


class TestFalconInstrumentation(TestFalconBase):
    def test_get(self):
        self._test_method("GET")

    def test_post(self):
        self._test_method("POST")

    def test_patch(self):
        self._test_method("PATCH")

    def test_put(self):
        self._test_method("PUT")

    def test_delete(self):
        self._test_method("DELETE")

    def test_head(self):
        self._test_method("HEAD")

    def _test_method(self, method):
        self.client().simulate_request(method=method, path="/hello")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(
            span.name, "HelloWorldResource.on_{0}".format(method.lower())
        )
        self.assertEqual(span.status.status_code, StatusCode.UNSET)
        self.assert_span_has_attributes(
            span,
            {
                SpanAttributes.HTTP_METHOD: method,
                SpanAttributes.HTTP_SERVER_NAME: "falconframework.org",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.NET_HOST_PORT: 80,
                SpanAttributes.HTTP_HOST: "falconframework.org",
                SpanAttributes.HTTP_TARGET: "/",
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.NET_PEER_PORT: "65133",
                SpanAttributes.HTTP_FLAVOR: "1.1",
                "falcon.resource": "HelloWorldResource",
                SpanAttributes.HTTP_STATUS_CODE: 201,
            },
        )
        self.memory_exporter.clear()

    def test_404(self):
        self.client().simulate_get("/does-not-exist")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.name, "HTTP GET")
        self.assertEqual(span.status.status_code, StatusCode.ERROR)
        self.assert_span_has_attributes(
            span,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_SERVER_NAME: "falconframework.org",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.NET_HOST_PORT: 80,
                SpanAttributes.HTTP_HOST: "falconframework.org",
                SpanAttributes.HTTP_TARGET: "/",
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.NET_PEER_PORT: "65133",
                SpanAttributes.HTTP_FLAVOR: "1.1",
                SpanAttributes.HTTP_STATUS_CODE: 404,
            },
        )

    def test_500(self):
        try:
            self.client().simulate_get("/error")
        except NameError:
            pass
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.name, "ErrorResource.on_get")
        self.assertFalse(span.status.is_ok)
        self.assertEqual(span.status.status_code, StatusCode.ERROR)
        self.assertEqual(
            span.status.description,
            "NameError: name 'non_existent_var' is not defined",
        )
        self.assert_span_has_attributes(
            span,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_SERVER_NAME: "falconframework.org",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.NET_HOST_PORT: 80,
                SpanAttributes.HTTP_HOST: "falconframework.org",
                SpanAttributes.HTTP_TARGET: "/",
                SpanAttributes.NET_PEER_IP: "127.0.0.1",
                SpanAttributes.NET_PEER_PORT: "65133",
                SpanAttributes.HTTP_FLAVOR: "1.1",
                SpanAttributes.HTTP_STATUS_CODE: 500,
            },
        )

    def test_uninstrument(self):
        self.client().simulate_get(path="/hello")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        self.memory_exporter.clear()

        FalconInstrumentor().uninstrument()
        self.app = make_app()
        self.client().simulate_get(path="/hello")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_exclude_lists(self):
        self.client().simulate_get(path="/ping")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)

        self.client().simulate_get(path="/hello")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_traced_request_attributes(self):
        self.client().simulate_get(path="/hello?q=abc")
        span = self.memory_exporter.get_finished_spans()[0]
        self.assertIn("query_string", span.attributes)
        self.assertEqual(span.attributes["query_string"], "q=abc")
        self.assertNotIn("not_available_attr", span.attributes)

    def test_trace_response(self):
        orig = get_global_response_propagator()
        set_global_response_propagator(TraceResponsePropagator())

        response = self.client().simulate_get(path="/hello?q=abc")
        headers = response.headers
        span = self.memory_exporter.get_finished_spans()[0]

        self.assertIn("traceresponse", headers)
        self.assertEqual(
            headers["access-control-expose-headers"], "traceresponse",
        )
        self.assertEqual(
            headers["traceresponse"],
            "00-{0}-{1}-01".format(
                format_trace_id(span.get_span_context().trace_id),
                format_span_id(span.get_span_context().span_id),
            ),
        )

        set_global_response_propagator(orig)

    def test_traced_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            self.client().simulate_get(path="/hello?q=abc")
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)


class TestFalconInstrumentationWithTracerProvider(TestBase):
    def setUp(self):
        super().setUp()
        resource = Resource.create({"resource-key": "resource-value"})
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result
        self.exporter = exporter

        FalconInstrumentor().instrument(tracer_provider=tracer_provider)
        self.app = make_app()

    def client(self):
        return testing.TestClient(self.app)

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FalconInstrumentor().uninstrument()

    def test_traced_request(self):
        self.client().simulate_request(method="GET", path="/hello")
        spans = self.exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(
            span.resource.attributes["resource-key"], "resource-value"
        )
        self.exporter.clear()


class TestFalconInstrumentationHooks(TestFalconBase):
    # pylint: disable=no-self-use
    def request_hook(self, span, req):
        span.set_attribute("request_hook_attr", "value from hook")

    def response_hook(self, span, req, resp):
        span.update_name("set from hook")

    def test_hooks(self):
        self.client().simulate_get(path="/hello?q=abc")
        span = self.memory_exporter.get_finished_spans()[0]

        self.assertEqual(span.name, "set from hook")
        self.assertIn("request_hook_attr", span.attributes)
        self.assertEqual(
            span.attributes["request_hook_attr"], "value from hook"
        )
