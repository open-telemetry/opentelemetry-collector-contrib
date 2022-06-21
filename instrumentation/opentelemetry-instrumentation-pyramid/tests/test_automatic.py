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

from unittest.mock import patch

from pyramid.config import Configurator

from opentelemetry import trace
from opentelemetry.instrumentation.pyramid import PyramidInstrumentor
from opentelemetry.test.globals_test import reset_trace_globals
from opentelemetry.test.wsgitestutil import WsgiTestBase
from opentelemetry.trace import SpanKind
from opentelemetry.trace.status import StatusCode
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
)

# pylint: disable=import-error
from .pyramid_base_test import InstrumentationTest


class TestAutomatic(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()

        PyramidInstrumentor().instrument()

        self.config = Configurator()

        self._common_initialization(self.config)

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            PyramidInstrumentor().uninstrument()

    def test_uninstrument(self):
        # pylint: disable=access-member-before-definition
        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        PyramidInstrumentor().uninstrument()
        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_tween_list(self):
        tween_list = "pyramid.tweens.excview_tween_factory"
        config = Configurator(settings={"pyramid.tweens": tween_list})
        self._common_initialization(config)
        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        PyramidInstrumentor().uninstrument()

        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_registry_name_is_this_module(self):
        config = Configurator()
        self.assertEqual(
            config.registry.__name__, __name__.rsplit(".", maxsplit=1)[0]
        )

    def test_redirect_response_is_not_an_error(self):
        tween_list = "pyramid.tweens.excview_tween_factory"
        config = Configurator(settings={"pyramid.tweens": tween_list})
        self._common_initialization(config)
        resp = self.client.get("/hello/302")
        self.assertEqual(302, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].status.status_code, StatusCode.UNSET)

        PyramidInstrumentor().uninstrument()

        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/302")
        self.assertEqual(302, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_204_empty_response_is_not_an_error(self):
        tween_list = "pyramid.tweens.excview_tween_factory"
        config = Configurator(settings={"pyramid.tweens": tween_list})
        self._common_initialization(config)
        resp = self.client.get("/hello/204")
        self.assertEqual(204, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].status.status_code, StatusCode.UNSET)

        PyramidInstrumentor().uninstrument()

        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/204")
        self.assertEqual(204, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_400s_response_is_not_an_error(self):
        tween_list = "pyramid.tweens.excview_tween_factory"
        config = Configurator(settings={"pyramid.tweens": tween_list})
        self._common_initialization(config)
        resp = self.client.get("/hello/404")
        self.assertEqual(404, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].status.status_code, StatusCode.UNSET)

        PyramidInstrumentor().uninstrument()

        self.config = Configurator()

        self._common_initialization(self.config)

        resp = self.client.get("/hello/404")
        self.assertEqual(404, resp.status_code)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)


class TestWrappedWithOtherFramework(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()
        PyramidInstrumentor().instrument()
        self.config = Configurator()
        self._common_initialization(self.config)

    def tearDown(self) -> None:
        super().tearDown()
        with self.disable_logging():
            PyramidInstrumentor().uninstrument()

    def test_with_existing_span(self):
        tracer_provider, _ = self.create_tracer_provider()
        tracer = tracer_provider.get_tracer(__name__)

        with tracer.start_as_current_span(
            "test", kind=SpanKind.SERVER
        ) as parent_span:
            resp = self.client.get("/hello/123")
            self.assertEqual(200, resp.status_code)
            span_list = self.memory_exporter.get_finished_spans()
            self.assertEqual(SpanKind.INTERNAL, span_list[0].kind)
            self.assertEqual(
                parent_span.get_span_context().span_id,
                span_list[0].parent.span_id,
            )


class TestCustomRequestResponseHeaders(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()
        PyramidInstrumentor().instrument()
        self.config = Configurator()
        self._common_initialization(self.config)
        self.env_patch = patch.dict(
            "os.environ",
            {
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,invalid-header",
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "content-type,content-length,my-custom-header,invalid-header",
            },
        )
        self.env_patch.start()

    def tearDown(self) -> None:
        super().tearDown()
        self.env_patch.stop()
        with self.disable_logging():
            PyramidInstrumentor().uninstrument()

    def test_custom_request_header_added_in_server_span(self):
        headers = {
            "Custom-Test-Header-1": "Test Value 1",
            "Custom-Test-Header-2": "TestValue2,TestValue3",
            "Custom-Test-Header-3": "TestValue4",
        }
        resp = self.client.get("/hello/123", headers=headers)
        self.assertEqual(200, resp.status_code)
        span = self.memory_exporter.get_finished_spans()[0]
        expected = {
            "http.request.header.custom_test_header_1": ("Test Value 1",),
            "http.request.header.custom_test_header_2": (
                "TestValue2,TestValue3",
            ),
        }
        not_expected = {
            "http.request.header.custom_test_header_3": ("TestValue4",),
        }
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)
        for key, _ in not_expected.items():
            self.assertNotIn(key, span.attributes)

    def test_custom_request_header_not_added_in_internal_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span("test", kind=SpanKind.SERVER):
            headers = {
                "Custom-Test-Header-1": "Test Value 1",
                "Custom-Test-Header-2": "TestValue2,TestValue3",
            }
            resp = self.client.get("/hello/123", headers=headers)
            self.assertEqual(200, resp.status_code)
            span = self.memory_exporter.get_finished_spans()[0]
            not_expected = {
                "http.request.header.custom_test_header_1": ("Test Value 1",),
                "http.request.header.custom_test_header_2": (
                    "TestValue2,TestValue3",
                ),
            }
            self.assertEqual(span.kind, SpanKind.INTERNAL)
            for key, _ in not_expected.items():
                self.assertNotIn(key, span.attributes)

    def test_custom_response_header_added_in_server_span(self):
        resp = self.client.get("/test_custom_response_headers")
        self.assertEqual(200, resp.status_code)
        span = self.memory_exporter.get_finished_spans()[0]
        expected = {
            "http.response.header.content_type": (
                "text/plain; charset=utf-8",
            ),
            "http.response.header.content_length": ("7",),
            "http.response.header.my_custom_header": (
                "my-custom-value-1,my-custom-header-2",
            ),
        }
        not_expected = {
            "http.response.header.dont_capture_me": ("test-value",)
        }
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertSpanHasAttributes(span, expected)
        for key, _ in not_expected.items():
            self.assertNotIn(key, span.attributes)

    def test_custom_response_header_not_added_in_internal_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span("test", kind=SpanKind.SERVER):
            resp = self.client.get("/test_custom_response_headers")
            self.assertEqual(200, resp.status_code)
            span = self.memory_exporter.get_finished_spans()[0]
            not_expected = {
                "http.response.header.content_type": (
                    "text/plain; charset=utf-8",
                ),
                "http.response.header.content_length": ("7",),
                "http.response.header.my_custom_header": (
                    "my-custom-value-1,my-custom-header-2",
                ),
            }
            self.assertEqual(span.kind, SpanKind.INTERNAL)
            for key, _ in not_expected.items():
                self.assertNotIn(key, span.attributes)


class TestCustomHeadersNonRecordingSpan(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()
        # This is done because set_tracer_provider cannot override the
        # current tracer provider.
        reset_trace_globals()
        tracer_provider = trace.NoOpTracerProvider()
        trace.set_tracer_provider(tracer_provider)
        PyramidInstrumentor().instrument()
        self.config = Configurator()
        self._common_initialization(self.config)
        self.env_patch = patch.dict(
            "os.environ",
            {
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST: "Custom-Test-Header-1,Custom-Test-Header-2,invalid-header",
                OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE: "content-type,content-length,my-custom-header,invalid-header",
            },
        )
        self.env_patch.start()

    def tearDown(self) -> None:
        super().tearDown()
        self.env_patch.stop()
        with self.disable_logging():
            PyramidInstrumentor().uninstrument()

    def test_custom_header_non_recording_span(self):
        try:
            resp = self.client.get("/hello/123")
            self.assertEqual(200, resp.status_code)
        except Exception as exc:  # pylint: disable=W0703
            self.fail(f"Exception raised with NonRecordingSpan {exc}")
