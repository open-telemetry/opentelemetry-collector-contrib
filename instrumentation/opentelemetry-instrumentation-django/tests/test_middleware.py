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

# pylint: disable=E0611

from sys import modules
from unittest.mock import Mock, patch

from django import VERSION, conf
from django.http import HttpRequest, HttpResponse
from django.test.client import Client
from django.test.utils import setup_test_environment, teardown_test_environment

from opentelemetry import trace
from opentelemetry.instrumentation.django import (
    DjangoInstrumentor,
    _DjangoMiddleware,
)
from opentelemetry.instrumentation.propagators import (
    TraceResponsePropagator,
    set_global_response_propagator,
)
from opentelemetry.sdk import resources
from opentelemetry.sdk.trace import Span
from opentelemetry.sdk.trace.id_generator import RandomIdGenerator
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.wsgitestutil import WsgiTestBase
from opentelemetry.trace import (
    SpanKind,
    StatusCode,
    format_span_id,
    format_trace_id,
)
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
    get_excluded_urls,
    get_traced_request_attrs,
)

# pylint: disable=import-error
from .views import (
    error,
    excluded,
    excluded_noarg,
    excluded_noarg2,
    response_with_custom_header,
    route_span_name,
    traced,
    traced_template,
)

DJANGO_2_0 = VERSION >= (2, 0)
DJANGO_2_2 = VERSION >= (2, 2)
DJANGO_3_0 = VERSION >= (3, 0)

if DJANGO_2_0:
    from django.urls import re_path
else:
    from django.conf.urls import url as re_path

urlpatterns = [
    re_path(r"^traced/", traced),
    re_path(r"^traced_custom_header/", response_with_custom_header),
    re_path(r"^route/(?P<year>[0-9]{4})/template/$", traced_template),
    re_path(r"^error/", error),
    re_path(r"^excluded_arg/", excluded),
    re_path(r"^excluded_noarg/", excluded_noarg),
    re_path(r"^excluded_noarg2/", excluded_noarg2),
    re_path(r"^span_name/([0-9]{4})/$", route_span_name),
]
_django_instrumentor = DjangoInstrumentor()


class TestMiddleware(WsgiTestBase):
    @classmethod
    def setUpClass(cls):
        conf.settings.configure(ROOT_URLCONF=modules[__name__])
        super().setUpClass()

    def setUp(self):
        super().setUp()
        setup_test_environment()
        _django_instrumentor.instrument()
        self.env_patch = patch.dict(
            "os.environ",
            {
                "OTEL_PYTHON_DJANGO_EXCLUDED_URLS": "http://testserver/excluded_arg/123,excluded_noarg",
                "OTEL_PYTHON_DJANGO_TRACED_REQUEST_ATTRS": "path_info,content_type,non_existing_variable",
            },
        )
        self.env_patch.start()
        self.exclude_patch = patch(
            "opentelemetry.instrumentation.django.middleware.otel_middleware._DjangoMiddleware._excluded_urls",
            get_excluded_urls("DJANGO"),
        )
        self.traced_patch = patch(
            "opentelemetry.instrumentation.django.middleware.otel_middleware._DjangoMiddleware._traced_request_attrs",
            get_traced_request_attrs("DJANGO"),
        )
        self.exclude_patch.start()
        self.traced_patch.start()

    def tearDown(self):
        super().tearDown()
        self.env_patch.stop()
        self.exclude_patch.stop()
        self.traced_patch.stop()
        teardown_test_environment()
        _django_instrumentor.uninstrument()

    @classmethod
    def tearDownClass(cls):
        super().tearDownClass()
        conf.settings = conf.LazySettings()

    def test_templated_route_get(self):
        Client().get("/route/2020/template/")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]

        self.assertEqual(
            span.name,
            "^route/(?P<year>[0-9]{4})/template/$"
            if DJANGO_2_2
            else "tests.views.traced_template",
        )
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertEqual(span.status.status_code, StatusCode.UNSET)
        self.assertEqual(span.attributes[SpanAttributes.HTTP_METHOD], "GET")
        self.assertEqual(
            span.attributes[SpanAttributes.HTTP_URL],
            "http://testserver/route/2020/template/",
        )
        if DJANGO_2_2:
            self.assertEqual(
                span.attributes[SpanAttributes.HTTP_ROUTE],
                "^route/(?P<year>[0-9]{4})/template/$",
            )
        self.assertEqual(span.attributes[SpanAttributes.HTTP_SCHEME], "http")
        self.assertEqual(span.attributes[SpanAttributes.HTTP_STATUS_CODE], 200)

    def test_traced_get(self):
        Client().get("/traced/")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]

        self.assertEqual(
            span.name, "^traced/" if DJANGO_2_2 else "tests.views.traced"
        )
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertEqual(span.status.status_code, StatusCode.UNSET)
        self.assertEqual(span.attributes[SpanAttributes.HTTP_METHOD], "GET")
        self.assertEqual(
            span.attributes[SpanAttributes.HTTP_URL],
            "http://testserver/traced/",
        )
        if DJANGO_2_2:
            self.assertEqual(
                span.attributes[SpanAttributes.HTTP_ROUTE], "^traced/"
            )
        self.assertEqual(span.attributes[SpanAttributes.HTTP_SCHEME], "http")
        self.assertEqual(span.attributes[SpanAttributes.HTTP_STATUS_CODE], 200)

    def test_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            Client().get("/traced/")
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_traced_post(self):
        Client().post("/traced/")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]

        self.assertEqual(
            span.name, "^traced/" if DJANGO_2_2 else "tests.views.traced"
        )
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertEqual(span.status.status_code, StatusCode.UNSET)
        self.assertEqual(span.attributes[SpanAttributes.HTTP_METHOD], "POST")
        self.assertEqual(
            span.attributes[SpanAttributes.HTTP_URL],
            "http://testserver/traced/",
        )
        if DJANGO_2_2:
            self.assertEqual(
                span.attributes[SpanAttributes.HTTP_ROUTE], "^traced/"
            )
        self.assertEqual(span.attributes[SpanAttributes.HTTP_SCHEME], "http")
        self.assertEqual(span.attributes[SpanAttributes.HTTP_STATUS_CODE], 200)

    def test_error(self):
        with self.assertRaises(ValueError):
            Client().get("/error/")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]

        self.assertEqual(
            span.name, "^error/" if DJANGO_2_2 else "tests.views.error"
        )
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertEqual(span.status.status_code, StatusCode.ERROR)
        self.assertEqual(span.attributes[SpanAttributes.HTTP_METHOD], "GET")
        self.assertEqual(
            span.attributes[SpanAttributes.HTTP_URL],
            "http://testserver/error/",
        )
        if DJANGO_2_2:
            self.assertEqual(
                span.attributes[SpanAttributes.HTTP_ROUTE], "^error/"
            )
        self.assertEqual(span.attributes[SpanAttributes.HTTP_SCHEME], "http")
        self.assertEqual(span.attributes[SpanAttributes.HTTP_STATUS_CODE], 500)

        self.assertEqual(len(span.events), 1)
        event = span.events[0]
        self.assertEqual(event.name, "exception")
        self.assertEqual(
            event.attributes[SpanAttributes.EXCEPTION_TYPE], "ValueError"
        )
        self.assertEqual(
            event.attributes[SpanAttributes.EXCEPTION_MESSAGE], "error"
        )

    def test_exclude_lists(self):
        client = Client()
        client.get("/excluded_arg/123")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)

        client.get("/excluded_arg/125")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        client.get("/excluded_noarg/")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        client.get("/excluded_noarg2/")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_span_name(self):
        # test no query_string
        Client().get("/span_name/1234/")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        span = span_list[0]
        self.assertEqual(
            span.name,
            "^span_name/([0-9]{4})/$"
            if DJANGO_2_2
            else "tests.views.route_span_name",
        )

    def test_span_name_for_query_string(self):
        """
        request not have query string
        """
        Client().get("/span_name/1234/?query=test")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        span = span_list[0]
        self.assertEqual(
            span.name,
            "^span_name/([0-9]{4})/$"
            if DJANGO_2_2
            else "tests.views.route_span_name",
        )

    def test_span_name_404(self):
        Client().get("/span_name/1234567890/")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        span = span_list[0]
        self.assertEqual(span.name, "HTTP GET")

    def test_traced_request_attrs(self):
        Client().get("/span_name/1234/", CONTENT_TYPE="test/ct")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        span = span_list[0]
        self.assertEqual(span.attributes["path_info"], "/span_name/1234/")
        self.assertEqual(span.attributes["content_type"], "test/ct")
        self.assertNotIn("non_existing_variable", span.attributes)

    def test_hooks(self):
        request_hook_args = ()
        response_hook_args = ()

        def request_hook(span, request):
            nonlocal request_hook_args
            request_hook_args = (span, request)

        def response_hook(span, request, response):
            nonlocal response_hook_args
            response_hook_args = (span, request, response)
            response["hook-header"] = "set by hook"

        _DjangoMiddleware._otel_request_hook = request_hook
        _DjangoMiddleware._otel_response_hook = response_hook

        response = Client().get("/span_name/1234/")
        _DjangoMiddleware._otel_request_hook = (
            _DjangoMiddleware._otel_response_hook
        ) = None

        self.assertEqual(response["hook-header"], "set by hook")

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        span = span_list[0]
        self.assertEqual(span.attributes["path_info"], "/span_name/1234/")

        self.assertEqual(len(request_hook_args), 2)
        self.assertEqual(request_hook_args[0].name, span.name)
        self.assertIsInstance(request_hook_args[0], Span)
        self.assertIsInstance(request_hook_args[1], HttpRequest)

        self.assertEqual(len(response_hook_args), 3)
        self.assertEqual(request_hook_args[0], response_hook_args[0])
        self.assertIsInstance(response_hook_args[1], HttpRequest)
        self.assertIsInstance(response_hook_args[2], HttpResponse)
        self.assertEqual(response_hook_args[2], response)

    async def test_trace_parent(self):
        id_generator = RandomIdGenerator()
        trace_id = format_trace_id(id_generator.generate_trace_id())
        span_id = format_span_id(id_generator.generate_span_id())
        traceparent_value = f"00-{trace_id}-{span_id}-01"

        Client().get(
            "/span_name/1234/",
            traceparent=traceparent_value,
        )
        span = self.memory_exporter.get_finished_spans()[0]

        self.assertEqual(
            trace_id,
            format_trace_id(span.get_span_context().trace_id),
        )
        self.assertIsNotNone(span.parent)
        self.assertEqual(
            trace_id,
            format_trace_id(span.parent.trace_id),
        )
        self.assertEqual(
            span_id,
            format_span_id(span.parent.span_id),
        )
        self.memory_exporter.clear()

    def test_trace_response_headers(self):
        response = Client().get("/span_name/1234/")

        self.assertFalse(response.has_header("Server-Timing"))
        self.memory_exporter.clear()

        set_global_response_propagator(TraceResponsePropagator())

        response = Client().get("/span_name/1234/")
        self.assertTraceResponseHeaderMatchesSpan(
            response,
            self.memory_exporter.get_finished_spans()[0],
        )
        self.memory_exporter.clear()


class TestMiddlewareWithTracerProvider(WsgiTestBase):
    @classmethod
    def setUpClass(cls):
        conf.settings.configure(ROOT_URLCONF=modules[__name__])
        super().setUpClass()

    def setUp(self):
        super().setUp()
        setup_test_environment()
        resource = resources.Resource.create(
            {"resource-key": "resource-value"}
        )
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result
        self.exporter = exporter
        self.tracer_provider = tracer_provider
        _django_instrumentor.instrument(tracer_provider=tracer_provider)

    def tearDown(self):
        super().tearDown()
        teardown_test_environment()
        _django_instrumentor.uninstrument()

    @classmethod
    def tearDownClass(cls):
        super().tearDownClass()
        conf.settings = conf.LazySettings()

    def test_tracer_provider_traced(self):
        Client().post("/traced/")

        spans = self.exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]

        self.assertEqual(
            span.resource.attributes["resource-key"], "resource-value"
        )

    def test_django_with_wsgi_instrumented(self):
        tracer = self.tracer_provider.get_tracer(__name__)
        with tracer.start_as_current_span(
            "test", kind=SpanKind.SERVER
        ) as parent_span:
            Client().get("/span_name/1234/")
            span_list = self.exporter.get_finished_spans()
            print(span_list)
            self.assertEqual(
                span_list[0].attributes[SpanAttributes.HTTP_STATUS_CODE], 200
            )
            self.assertEqual(trace.SpanKind.INTERNAL, span_list[0].kind)
            self.assertEqual(
                parent_span.get_span_context().span_id,
                span_list[0].parent.span_id,
            )


class TestMiddlewareWsgiWithCustomHeaders(WsgiTestBase):
    @classmethod
    def setUpClass(cls):
        conf.settings.configure(ROOT_URLCONF=modules[__name__])
        super().setUpClass()

    def setUp(self):
        super().setUp()
        setup_test_environment()
        tracer_provider, exporter = self.create_tracer_provider()
        self.exporter = exporter
        _django_instrumentor.instrument(tracer_provider=tracer_provider)
        self.env_patch = patch.dict(
            "os.environ",
            {
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3",
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "Custom-Test-Header-1,Custom-Test-Header-2,Custom-Test-Header-3",
            },
        )
        self.env_patch.start()

    def tearDown(self):
        super().tearDown()
        self.env_patch.stop()
        teardown_test_environment()
        _django_instrumentor.uninstrument()

    @classmethod
    def tearDownClass(cls):
        super().tearDownClass()
        conf.settings = conf.LazySettings()

    def test_http_custom_request_headers_in_span_attributes(self):
        expected = {
            "http.request.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.request.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        Client(
            HTTP_CUSTOM_TEST_HEADER_1="test-header-value-1",
            HTTP_CUSTOM_TEST_HEADER_2="test-header-value-2",
        ).get("/traced/")
        spans = self.exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)
        self.memory_exporter.clear()

    def test_http_custom_request_headers_not_in_span_attributes(self):
        not_expected = {
            "http.request.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        Client(HTTP_CUSTOM_TEST_HEADER_1="test-header-value-1").get("/traced/")
        spans = self.exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]
        self.assertEqual(span.kind, SpanKind.SERVER)
        for key, _ in not_expected.items():
            self.assertNotIn(key, span.attributes)
        self.memory_exporter.clear()

    def test_http_custom_response_headers_in_span_attributes(self):
        expected = {
            "http.response.header.custom_test_header_1": (
                "test-header-value-1",
            ),
            "http.response.header.custom_test_header_2": (
                "test-header-value-2",
            ),
        }
        Client().get("/traced_custom_header/")
        spans = self.exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)
        self.memory_exporter.clear()

    def test_http_custom_response_headers_not_in_span_attributes(self):
        not_expected = {
            "http.response.header.custom_test_header_3": (
                "test-header-value-3",
            ),
        }
        Client().get("/traced_custom_header/")
        spans = self.exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]
        self.assertEqual(span.kind, SpanKind.SERVER)
        for key, _ in not_expected.items():
            self.assertNotIn(key, span.attributes)
        self.memory_exporter.clear()
