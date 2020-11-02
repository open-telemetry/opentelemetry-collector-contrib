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

from sys import modules
from unittest.mock import Mock, patch

from django import VERSION
from django.conf import settings
from django.conf.urls import url
from django.test import Client
from django.test.utils import setup_test_environment, teardown_test_environment

from opentelemetry.configuration import Configuration
from opentelemetry.instrumentation.django import DjangoInstrumentor
from opentelemetry.sdk.util import get_dict_as_key
from opentelemetry.test.test_base import TestBase
from opentelemetry.test.wsgitestutil import WsgiTestBase
from opentelemetry.trace import SpanKind
from opentelemetry.trace.status import StatusCode
from opentelemetry.util import ExcludeList

# pylint: disable=import-error
from .views import (
    error,
    excluded,
    excluded_noarg,
    excluded_noarg2,
    route_span_name,
    traced,
    traced_template,
)

DJANGO_2_2 = VERSION >= (2, 2)

urlpatterns = [
    url(r"^traced/", traced),
    url(r"^route/(?P<year>[0-9]{4})/template/$", traced_template),
    url(r"^error/", error),
    url(r"^excluded_arg/", excluded),
    url(r"^excluded_noarg/", excluded_noarg),
    url(r"^excluded_noarg2/", excluded_noarg2),
    url(r"^span_name/([0-9]{4})/$", route_span_name),
]
_django_instrumentor = DjangoInstrumentor()


class TestMiddleware(TestBase, WsgiTestBase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        settings.configure(ROOT_URLCONF=modules[__name__])

    def setUp(self):
        super().setUp()
        setup_test_environment()
        _django_instrumentor.instrument()
        Configuration._reset()  # pylint: disable=protected-access

    def tearDown(self):
        super().tearDown()
        teardown_test_environment()
        _django_instrumentor.uninstrument()

    def test_templated_route_get(self):
        Client().get("/route/2020/template/")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)

        span = spans[0]

        self.assertEqual(
            span.name,
            "^route/(?P<year>[0-9]{4})/template/$"
            if DJANGO_2_2
            else "tests.views.traced",
        )
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertEqual(span.status.status_code, StatusCode.UNSET)
        self.assertEqual(span.attributes["http.method"], "GET")
        self.assertEqual(
            span.attributes["http.url"],
            "http://testserver/route/2020/template/",
        )
        self.assertEqual(
            span.attributes["http.route"],
            "^route/(?P<year>[0-9]{4})/template/$",
        )
        self.assertEqual(span.attributes["http.scheme"], "http")
        self.assertEqual(span.attributes["http.status_code"], 200)
        self.assertEqual(span.attributes["http.status_text"], "OK")

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
        self.assertEqual(span.attributes["http.method"], "GET")
        self.assertEqual(
            span.attributes["http.url"], "http://testserver/traced/"
        )
        self.assertEqual(span.attributes["http.route"], "^traced/")
        self.assertEqual(span.attributes["http.scheme"], "http")
        self.assertEqual(span.attributes["http.status_code"], 200)
        self.assertEqual(span.attributes["http.status_text"], "OK")

        self.assertIsNotNone(_django_instrumentor.meter)
        self.assertEqual(len(_django_instrumentor.meter.metrics), 1)
        recorder = _django_instrumentor.meter.metrics.pop()
        match_key = get_dict_as_key(
            {
                "http.flavor": "1.1",
                "http.method": "GET",
                "http.status_code": "200",
                "http.url": "http://testserver/traced/",
            }
        )
        for key in recorder.bound_instruments.keys():
            self.assertEqual(key, match_key)
            # pylint: disable=protected-access
            bound = recorder.bound_instruments.get(key)
            for view_data in bound.view_datas:
                self.assertEqual(view_data.labels, key)
                self.assertEqual(view_data.aggregator.current.count, 1)
                self.assertGreaterEqual(view_data.aggregator.current.sum, 0)

    def test_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        mock_tracer.use_span.return_value.__enter__ = mock_span
        mock_tracer.use_span.return_value.__exit__ = True
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
        self.assertEqual(span.attributes["http.method"], "POST")
        self.assertEqual(
            span.attributes["http.url"], "http://testserver/traced/"
        )
        self.assertEqual(span.attributes["http.route"], "^traced/")
        self.assertEqual(span.attributes["http.scheme"], "http")
        self.assertEqual(span.attributes["http.status_code"], 200)
        self.assertEqual(span.attributes["http.status_text"], "OK")

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
        self.assertEqual(span.attributes["http.method"], "GET")
        self.assertEqual(
            span.attributes["http.url"], "http://testserver/error/"
        )
        self.assertEqual(span.attributes["http.route"], "^error/")
        self.assertEqual(span.attributes["http.scheme"], "http")
        self.assertEqual(span.attributes["http.status_code"], 500)
        self.assertIsNotNone(_django_instrumentor.meter)
        self.assertEqual(len(_django_instrumentor.meter.metrics), 1)

        self.assertEqual(len(span.events), 1)
        event = span.events[0]
        self.assertEqual(event.name, "exception")
        self.assertEqual(event.attributes["exception.type"], "ValueError")
        self.assertEqual(event.attributes["exception.message"], "error")

        recorder = _django_instrumentor.meter.metrics.pop()
        match_key = get_dict_as_key(
            {
                "http.flavor": "1.1",
                "http.method": "GET",
                "http.status_code": "500",
                "http.url": "http://testserver/error/",
            }
        )
        for key in recorder.bound_instruments.keys():
            self.assertEqual(key, match_key)
            # pylint: disable=protected-access
            bound = recorder.bound_instruments.get(key)
            for view_data in bound.view_datas:
                self.assertEqual(view_data.labels, key)
                self.assertEqual(view_data.aggregator.current.count, 1)

    @patch(
        "opentelemetry.instrumentation.django.middleware._DjangoMiddleware._excluded_urls",
        ExcludeList(["http://testserver/excluded_arg/123", "excluded_noarg"]),
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
        with patch(
            "opentelemetry.instrumentation.django.middleware._DjangoMiddleware._traced_request_attrs",
            [],
        ):
            Client().get("/span_name/1234/", CONTENT_TYPE="test/ct")
            span_list = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(span_list), 1)

            span = span_list[0]
            self.assertNotIn("path_info", span.attributes)
            self.assertNotIn("content_type", span.attributes)
            self.memory_exporter.clear()

        with patch(
            "opentelemetry.instrumentation.django.middleware._DjangoMiddleware._traced_request_attrs",
            ["path_info", "content_type", "non_existing_variable"],
        ):
            Client().get("/span_name/1234/", CONTENT_TYPE="test/ct")
            span_list = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(span_list), 1)

            span = span_list[0]
            self.assertEqual(span.attributes["path_info"], "/span_name/1234/")
            self.assertEqual(span.attributes["content_type"], "test/ct")
            self.assertNotIn("non_existing_variable", span.attributes)
