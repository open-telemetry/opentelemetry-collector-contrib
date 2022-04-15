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
#
from unittest.mock import Mock, patch

import pytest
from falcon import __version__ as _falcon_verison
from falcon import testing
from packaging import version as package_version

from opentelemetry import trace
from opentelemetry.instrumentation.falcon import FalconInstrumentor
from opentelemetry.instrumentation.propagators import (
    TraceResponsePropagator,
    get_global_response_propagator,
    set_global_response_propagator,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase
from opentelemetry.test.wsgitestutil import WsgiTestBase
from opentelemetry.trace import StatusCode
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
)

from .app import make_app


class TestFalconBase(TestBase):
    def setUp(self):
        super().setUp()
        self.env_patch = patch.dict(
            "os.environ",
            {
                "OTEL_PYTHON_FALCON_EXCLUDED_URLS": "ping",
                "OTEL_PYTHON_FALCON_TRACED_REQUEST_ATTRS": "query_string",
            },
        )
        self.env_patch.start()

        FalconInstrumentor().instrument(
            request_hook=getattr(self, "request_hook", None),
            response_hook=getattr(self, "response_hook", None),
        )
        self.app = make_app()

    def client(self):
        return testing.TestClient(self.app)

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FalconInstrumentor().uninstrument()
        self.env_patch.stop()


class TestFalconInstrumentation(TestFalconBase, WsgiTestBase):
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
        self.assertEqual(span.name, f"HelloWorldResource.on_{method.lower()}")
        self.assertEqual(span.status.status_code, StatusCode.UNSET)
        self.assertEqual(
            span.status.description,
            None,
        )
        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.HTTP_METHOD: method,
                SpanAttributes.HTTP_SERVER_NAME: "falconframework.org",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.NET_HOST_PORT: 80,
                SpanAttributes.HTTP_HOST: "falconframework.org",
                SpanAttributes.HTTP_TARGET: "/",
                SpanAttributes.NET_PEER_PORT: "65133",
                SpanAttributes.HTTP_FLAVOR: "1.1",
                "falcon.resource": "HelloWorldResource",
                SpanAttributes.HTTP_STATUS_CODE: 201,
            },
        )
        # In falcon<3, NET_PEER_IP is always set by default to 127.0.0.1
        # In falcon>3, NET_PEER_IP is not set to anything by default to
        # https://github.com/falconry/falcon/blob/5233d0abed977d9dab78ebadf305f5abe2eef07c/falcon/testing/helpers.py#L1168-L1172 # noqa
        if SpanAttributes.NET_PEER_IP in span.attributes:
            self.assertEqual(
                span.attributes[SpanAttributes.NET_PEER_IP], "127.0.0.1"
            )
        self.memory_exporter.clear()

    def test_404(self):
        self.client().simulate_get("/does-not-exist")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.name, "HTTP GET")
        self.assertEqual(span.status.status_code, StatusCode.UNSET)
        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_SERVER_NAME: "falconframework.org",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.NET_HOST_PORT: 80,
                SpanAttributes.HTTP_HOST: "falconframework.org",
                SpanAttributes.HTTP_TARGET: "/",
                SpanAttributes.NET_PEER_PORT: "65133",
                SpanAttributes.HTTP_FLAVOR: "1.1",
                SpanAttributes.HTTP_STATUS_CODE: 404,
            },
        )
        # In falcon<3, NET_PEER_IP is always set by default to 127.0.0.1
        # In falcon>3, NET_PEER_IP is not set to anything by default to
        # https://github.com/falconry/falcon/blob/5233d0abed977d9dab78ebadf305f5abe2eef07c/falcon/testing/helpers.py#L1168-L1172 # noqa
        if SpanAttributes.NET_PEER_IP in span.attributes:
            self.assertEqual(
                span.attributes[SpanAttributes.NET_PEER_IP], "127.0.0.1"
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
        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_SERVER_NAME: "falconframework.org",
                SpanAttributes.HTTP_SCHEME: "http",
                SpanAttributes.NET_HOST_PORT: 80,
                SpanAttributes.HTTP_HOST: "falconframework.org",
                SpanAttributes.HTTP_TARGET: "/",
                SpanAttributes.NET_PEER_PORT: "65133",
                SpanAttributes.HTTP_FLAVOR: "1.1",
                SpanAttributes.HTTP_STATUS_CODE: 500,
            },
        )
        # In falcon<3, NET_PEER_IP is always set by default to 127.0.0.1
        # In falcon>3, NET_PEER_IP is not set to anything by default to
        # https://github.com/falconry/falcon/blob/5233d0abed977d9dab78ebadf305f5abe2eef07c/falcon/testing/helpers.py#L1168-L1172 # noqa
        if SpanAttributes.NET_PEER_IP in span.attributes:
            self.assertEqual(
                span.attributes[SpanAttributes.NET_PEER_IP], "127.0.0.1"
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
        self.client().simulate_get(path="/hello", query_string="q=abc")
        span = self.memory_exporter.get_finished_spans()[0]
        self.assertIn("query_string", span.attributes)
        self.assertEqual(span.attributes["query_string"], "q=abc")
        self.assertNotIn("not_available_attr", span.attributes)

    def test_trace_response(self):
        orig = get_global_response_propagator()
        set_global_response_propagator(TraceResponsePropagator())

        response = self.client().simulate_get(
            path="/hello", query_string="q=abc"
        )
        self.assertTraceResponseHeaderMatchesSpan(
            response.headers, self.memory_exporter.get_finished_spans()[0]
        )

        set_global_response_propagator(orig)

    def test_traced_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            self.client().simulate_get(path="/hello", query_string="q=abc")
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
        self.client().simulate_get(path="/hello", query_string="q=abc")
        span = self.memory_exporter.get_finished_spans()[0]

        self.assertEqual(span.name, "set from hook")
        self.assertIn("request_hook_attr", span.attributes)
        self.assertEqual(
            span.attributes["request_hook_attr"], "value from hook"
        )


class TestFalconInstrumentationWrappedWithOtherFramework(TestFalconBase):
    def test_mark_span_internal_in_presence_of_span_from_other_framework(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span(
            "test", kind=trace.SpanKind.SERVER
        ) as parent_span:
            self.client().simulate_request(method="GET", path="/hello")
            span = self.memory_exporter.get_finished_spans()[0]
            assert span.status.is_ok
            self.assertEqual(trace.SpanKind.INTERNAL, span.kind)
            self.assertEqual(
                span.parent.span_id, parent_span.get_span_context().span_id
            )


@patch.dict(
    "os.environ",
    {
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,invalid-header",
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "content-type,content-length,my-custom-header,invalid-header",
    },
)
class TestCustomRequestResponseHeaders(TestFalconBase):
    def test_custom_request_header_added_in_server_span(self):
        headers = {
            "Custom-Test-Header-1": "Test Value 1",
            "Custom-Test-Header-2": "TestValue2,TestValue3",
            "Custom-Test-Header-3": "TestValue4",
        }
        self.client().simulate_request(
            method="GET", path="/hello", headers=headers
        )
        span = self.memory_exporter.get_finished_spans()[0]
        assert span.status.is_ok

        expected = {
            "http.request.header.custom_test_header_1": ("Test Value 1",),
            "http.request.header.custom_test_header_2": (
                "TestValue2,TestValue3",
            ),
        }
        not_expected = {
            "http.request.header.custom_test_header_3": ("TestValue4",),
        }

        self.assertEqual(span.kind, trace.SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)
        for key, _ in not_expected.items():
            self.assertNotIn(key, span.attributes)

    def test_custom_request_header_not_added_in_internal_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span("test", kind=trace.SpanKind.SERVER):
            headers = {
                "Custom-Test-Header-1": "Test Value 1",
                "Custom-Test-Header-2": "TestValue2,TestValue3",
            }
            self.client().simulate_request(
                method="GET", path="/hello", headers=headers
            )
            span = self.memory_exporter.get_finished_spans()[0]
            assert span.status.is_ok
            not_expected = {
                "http.request.header.custom_test_header_1": ("Test Value 1",),
                "http.request.header.custom_test_header_2": (
                    "TestValue2,TestValue3",
                ),
            }
            self.assertEqual(span.kind, trace.SpanKind.INTERNAL)
            for key, _ in not_expected.items():
                self.assertNotIn(key, span.attributes)

    @pytest.mark.skipif(
        condition=package_version.parse(_falcon_verison)
        < package_version.parse("2.0.0"),
        reason="falcon<2 does not implement custom response headers",
    )
    def test_custom_response_header_added_in_server_span(self):
        self.client().simulate_request(
            method="GET", path="/test_custom_response_headers"
        )
        span = self.memory_exporter.get_finished_spans()[0]
        assert span.status.is_ok
        expected = {
            "http.response.header.content_type": (
                "text/plain; charset=utf-8",
            ),
            "http.response.header.content_length": ("0",),
            "http.response.header.my_custom_header": (
                "my-custom-value-1,my-custom-header-2",
            ),
        }
        not_expected = {
            "http.response.header.dont_capture_me": ("test-value",)
        }
        self.assertEqual(span.kind, trace.SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)
        for key, _ in not_expected.items():
            self.assertNotIn(key, span.attributes)

    @pytest.mark.skipif(
        condition=package_version.parse(_falcon_verison)
        < package_version.parse("2.0.0"),
        reason="falcon<2 does not implement custom response headers",
    )
    def test_custom_response_header_not_added_in_internal_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span("test", kind=trace.SpanKind.SERVER):
            self.client().simulate_request(
                method="GET", path="/test_custom_response_headers"
            )
            span = self.memory_exporter.get_finished_spans()[0]
            assert span.status.is_ok
            not_expected = {
                "http.response.header.content_type": (
                    "text/plain; charset=utf-8",
                ),
                "http.response.header.content_length": ("0",),
                "http.response.header.my_custom_header": (
                    "my-custom-value-1,my-custom-header-2",
                ),
            }
            self.assertEqual(span.kind, trace.SpanKind.INTERNAL)
            for key, _ in not_expected.items():
                self.assertNotIn(key, span.attributes)
