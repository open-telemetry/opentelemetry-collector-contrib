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

from unittest import mock

import httpretty
import requests

import opentelemetry.instrumentation.requests
from opentelemetry import context, propagators, trace
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.sdk import resources
from opentelemetry.test.mock_httptextformat import MockHTTPTextFormat
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace.status import StatusCanonicalCode


class TestRequestsIntegration(TestBase):
    URL = "http://httpbin.org/status/200"

    def setUp(self):
        super().setUp()
        RequestsInstrumentor().instrument()
        httpretty.enable()
        httpretty.register_uri(
            httpretty.GET, self.URL, body="Hello!",
        )

    def tearDown(self):
        super().tearDown()
        RequestsInstrumentor().uninstrument()
        httpretty.disable()

    def test_basic(self):
        result = requests.get(self.URL)
        self.assertEqual(result.text, "Hello!")

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        span = span_list[0]

        self.assertIs(span.kind, trace.SpanKind.CLIENT)
        self.assertEqual(span.name, "/status/200")

        self.assertEqual(
            span.attributes,
            {
                "component": "http",
                "http.method": "GET",
                "http.url": self.URL,
                "http.status_code": 200,
                "http.status_text": "OK",
            },
        )

        self.assertIs(
            span.status.canonical_code, trace.status.StatusCanonicalCode.OK
        )

        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.requests
        )

    def test_not_foundbasic(self):
        url_404 = "http://httpbin.org/status/404"
        httpretty.register_uri(
            httpretty.GET, url_404, status=404,
        )
        result = requests.get(url_404)
        self.assertEqual(result.status_code, 404)

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        span = span_list[0]

        self.assertEqual(span.attributes.get("http.status_code"), 404)
        self.assertEqual(span.attributes.get("http.status_text"), "Not Found")

        self.assertIs(
            span.status.canonical_code,
            trace.status.StatusCanonicalCode.NOT_FOUND,
        )

    def test_invalid_url(self):
        url = "http://[::1/nope"

        with self.assertRaises(ValueError):
            requests.post(url)

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        span = span_list[0]

        self.assertTrue(span.name.startswith("<Unparsable URL"))
        self.assertEqual(
            span.attributes,
            {"component": "http", "http.method": "POST", "http.url": url},
        )
        self.assertEqual(
            span.status.canonical_code, StatusCanonicalCode.INVALID_ARGUMENT
        )

    def test_uninstrument(self):
        RequestsInstrumentor().uninstrument()
        result = requests.get(self.URL)
        self.assertEqual(result.text, "Hello!")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)
        # instrument again to avoid annoying warning message
        RequestsInstrumentor().instrument()

    def test_uninstrument_session(self):
        session1 = requests.Session()
        RequestsInstrumentor().uninstrument_session(session1)

        result = session1.get(self.URL)
        self.assertEqual(result.text, "Hello!")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)

        # Test that other sessions as well as global requests is still
        # instrumented
        session2 = requests.Session()
        result = session2.get(self.URL)
        self.assertEqual(result.text, "Hello!")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        self.memory_exporter.clear()

        result = requests.get(self.URL)
        self.assertEqual(result.text, "Hello!")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_suppress_instrumentation(self):
        token = context.attach(
            context.set_value("suppress_instrumentation", True)
        )
        try:
            result = requests.get(self.URL)
            self.assertEqual(result.text, "Hello!")
        finally:
            context.detach(token)

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)

    def test_distributed_context(self):
        previous_propagator = propagators.get_global_httptextformat()
        try:
            propagators.set_global_httptextformat(MockHTTPTextFormat())
            result = requests.get(self.URL)
            self.assertEqual(result.text, "Hello!")

            span_list = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(span_list), 1)
            span = span_list[0]

            headers = dict(httpretty.last_request().headers)
            self.assertIn(MockHTTPTextFormat.TRACE_ID_KEY, headers)
            self.assertEqual(
                str(span.get_context().trace_id),
                headers[MockHTTPTextFormat.TRACE_ID_KEY],
            )
            self.assertIn(MockHTTPTextFormat.SPAN_ID_KEY, headers)
            self.assertEqual(
                str(span.get_context().span_id),
                headers[MockHTTPTextFormat.SPAN_ID_KEY],
            )

        finally:
            propagators.set_global_httptextformat(previous_propagator)

    def test_span_callback(self):
        RequestsInstrumentor().uninstrument()

        def span_callback(span, result: requests.Response):
            span.set_attribute(
                "http.response.body", result.content.decode("utf-8")
            )

        RequestsInstrumentor().instrument(
            tracer_provider=self.tracer_provider, span_callback=span_callback,
        )

        result = requests.get(self.URL)
        self.assertEqual(result.text, "Hello!")

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        span = span_list[0]

        self.assertEqual(
            span.attributes,
            {
                "component": "http",
                "http.method": "GET",
                "http.url": self.URL,
                "http.status_code": 200,
                "http.status_text": "OK",
                "http.response.body": "Hello!",
            },
        )

    def test_custom_tracer_provider(self):
        resource = resources.Resource.create({})
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result
        RequestsInstrumentor().uninstrument()
        RequestsInstrumentor().instrument(tracer_provider=tracer_provider)

        result = requests.get(self.URL)
        self.assertEqual(result.text, "Hello!")

        span_list = exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        span = span_list[0]

        self.assertIs(span.resource, resource)

    def test_if_headers_equals_none(self):
        result = requests.get(self.URL, headers=None)
        self.assertEqual(result.text, "Hello!")

    @mock.patch("requests.Session.send", side_effect=requests.RequestException)
    def test_requests_exception_without_response(self, *_, **__):

        with self.assertRaises(requests.RequestException):
            requests.get(self.URL)

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        span = span_list[0]
        self.assertEqual(
            span.attributes,
            {"component": "http", "http.method": "GET", "http.url": self.URL},
        )
        self.assertEqual(
            span.status.canonical_code, StatusCanonicalCode.UNKNOWN
        )

    mocked_response = requests.Response()
    mocked_response.status_code = 500
    mocked_response.reason = "Internal Server Error"

    @mock.patch(
        "requests.Session.send",
        side_effect=requests.RequestException(response=mocked_response),
    )
    def test_requests_exception_with_response(self, *_, **__):

        with self.assertRaises(requests.RequestException):
            requests.get(self.URL)

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        span = span_list[0]
        self.assertEqual(
            span.attributes,
            {
                "component": "http",
                "http.method": "GET",
                "http.url": self.URL,
                "http.status_code": 500,
                "http.status_text": "Internal Server Error",
            },
        )
        self.assertEqual(
            span.status.canonical_code, StatusCanonicalCode.INTERNAL
        )

    @mock.patch("requests.Session.send", side_effect=Exception)
    def test_requests_basic_exception(self, *_, **__):

        with self.assertRaises(Exception):
            requests.get(self.URL)

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(
            span_list[0].status.canonical_code, StatusCanonicalCode.UNKNOWN
        )

    @mock.patch("requests.Session.send", side_effect=requests.Timeout)
    def test_requests_timeout_exception(self, *_, **__):

        with self.assertRaises(Exception):
            requests.get(self.URL)

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(
            span_list[0].status.canonical_code,
            StatusCanonicalCode.DEADLINE_EXCEEDED,
        )
