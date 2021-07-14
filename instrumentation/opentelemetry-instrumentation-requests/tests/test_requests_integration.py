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

import abc
from unittest import mock

import httpretty
import requests
from requests.adapters import BaseAdapter
from requests.models import Response

import opentelemetry.instrumentation.requests
from opentelemetry import context, trace
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.instrumentation.utils import _SUPPRESS_INSTRUMENTATION_KEY
from opentelemetry.propagate import get_global_textmap, set_global_textmap
from opentelemetry.sdk import resources
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.mock_textmap import MockTextMapPropagator
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import StatusCode


class TransportMock:
    def read(self, *args, **kwargs):
        pass


class MyAdapter(BaseAdapter):
    def __init__(self, response):
        super().__init__()
        self._response = response

    def send(self, *args, **kwargs):  # pylint:disable=signature-differs
        return self._response

    def close(self):
        pass


class InvalidResponseObjectException(Exception):
    def __init__(self):
        super().__init__()
        self.response = {}


class RequestsIntegrationTestBase(abc.ABC):
    # pylint: disable=no-member
    # pylint: disable=too-many-public-methods

    URL = "http://httpbin.org/status/200"

    # pylint: disable=invalid-name
    def setUp(self):
        super().setUp()
        RequestsInstrumentor().instrument()
        httpretty.enable()
        httpretty.register_uri(httpretty.GET, self.URL, body="Hello!")

    # pylint: disable=invalid-name
    def tearDown(self):
        super().tearDown()
        RequestsInstrumentor().uninstrument()
        httpretty.disable()

    def assert_span(self, exporter=None, num_spans=1):
        if exporter is None:
            exporter = self.memory_exporter
        span_list = exporter.get_finished_spans()
        self.assertEqual(num_spans, len(span_list))
        if num_spans == 0:
            return None
        if num_spans == 1:
            return span_list[0]
        return span_list

    @staticmethod
    @abc.abstractmethod
    def perform_request(url: str, session: requests.Session = None):
        pass

    def test_basic(self):
        result = self.perform_request(self.URL)
        self.assertEqual(result.text, "Hello!")
        span = self.assert_span()

        self.assertIs(span.kind, trace.SpanKind.CLIENT)
        self.assertEqual(span.name, "HTTP GET")

        self.assertEqual(
            span.attributes,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_URL: self.URL,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

        self.assertIs(span.status.status_code, trace.StatusCode.UNSET)

        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.requests
        )

    def test_name_callback(self):
        def name_callback(method, url):
            return "GET" + url

        RequestsInstrumentor().uninstrument()
        RequestsInstrumentor().instrument(name_callback=name_callback)
        result = self.perform_request(self.URL)
        self.assertEqual(result.text, "Hello!")
        span = self.assert_span()

        self.assertEqual(span.name, "GET" + self.URL)

    def test_name_callback_default(self):
        def name_callback(method, url):
            return 123

        RequestsInstrumentor().uninstrument()
        RequestsInstrumentor().instrument(name_callback=name_callback)
        result = self.perform_request(self.URL)
        self.assertEqual(result.text, "Hello!")
        span = self.assert_span()

        self.assertEqual(span.name, "HTTP GET")

    def test_not_foundbasic(self):
        url_404 = "http://httpbin.org/status/404"
        httpretty.register_uri(
            httpretty.GET, url_404, status=404,
        )
        result = self.perform_request(url_404)
        self.assertEqual(result.status_code, 404)

        span = self.assert_span()

        self.assertEqual(
            span.attributes.get(SpanAttributes.HTTP_STATUS_CODE), 404
        )

        self.assertIs(
            span.status.status_code, trace.StatusCode.ERROR,
        )

    def test_uninstrument(self):
        RequestsInstrumentor().uninstrument()
        result = self.perform_request(self.URL)
        self.assertEqual(result.text, "Hello!")
        self.assert_span(num_spans=0)
        # instrument again to avoid annoying warning message
        RequestsInstrumentor().instrument()

    def test_uninstrument_session(self):
        session1 = requests.Session()
        RequestsInstrumentor().uninstrument_session(session1)

        result = self.perform_request(self.URL, session1)
        self.assertEqual(result.text, "Hello!")
        self.assert_span(num_spans=0)

        # Test that other sessions as well as global requests is still
        # instrumented
        session2 = requests.Session()
        result = self.perform_request(self.URL, session2)
        self.assertEqual(result.text, "Hello!")
        self.assert_span()

        self.memory_exporter.clear()

        result = self.perform_request(self.URL)
        self.assertEqual(result.text, "Hello!")
        self.assert_span()

    def test_suppress_instrumentation(self):
        token = context.attach(
            context.set_value(_SUPPRESS_INSTRUMENTATION_KEY, True)
        )
        try:
            result = self.perform_request(self.URL)
            self.assertEqual(result.text, "Hello!")
        finally:
            context.detach(token)

        self.assert_span(num_spans=0)

    def test_not_recording(self):
        with mock.patch("opentelemetry.trace.INVALID_SPAN") as mock_span:
            RequestsInstrumentor().uninstrument()
            RequestsInstrumentor().instrument(
                tracer_provider=trace._DefaultTracerProvider()
            )
            mock_span.is_recording.return_value = False
            result = self.perform_request(self.URL)
            self.assertEqual(result.text, "Hello!")
            self.assert_span(None, 0)
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_distributed_context(self):
        previous_propagator = get_global_textmap()
        try:
            set_global_textmap(MockTextMapPropagator())
            result = self.perform_request(self.URL)
            self.assertEqual(result.text, "Hello!")

            span = self.assert_span()

            headers = dict(httpretty.last_request().headers)
            self.assertIn(MockTextMapPropagator.TRACE_ID_KEY, headers)
            self.assertEqual(
                str(span.get_span_context().trace_id),
                headers[MockTextMapPropagator.TRACE_ID_KEY],
            )
            self.assertIn(MockTextMapPropagator.SPAN_ID_KEY, headers)
            self.assertEqual(
                str(span.get_span_context().span_id),
                headers[MockTextMapPropagator.SPAN_ID_KEY],
            )

        finally:
            set_global_textmap(previous_propagator)

    def test_span_callback(self):
        RequestsInstrumentor().uninstrument()

        def span_callback(span, result: requests.Response):
            span.set_attribute(
                "http.response.body", result.content.decode("utf-8")
            )

        RequestsInstrumentor().instrument(
            tracer_provider=self.tracer_provider, span_callback=span_callback,
        )

        result = self.perform_request(self.URL)
        self.assertEqual(result.text, "Hello!")

        span = self.assert_span()
        self.assertEqual(
            span.attributes,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_URL: self.URL,
                SpanAttributes.HTTP_STATUS_CODE: 200,
                "http.response.body": "Hello!",
            },
        )

    def test_custom_tracer_provider(self):
        resource = resources.Resource.create({})
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result
        RequestsInstrumentor().uninstrument()
        RequestsInstrumentor().instrument(tracer_provider=tracer_provider)

        result = self.perform_request(self.URL)
        self.assertEqual(result.text, "Hello!")

        span = self.assert_span(exporter=exporter)
        self.assertIs(span.resource, resource)

    @mock.patch(
        "requests.adapters.HTTPAdapter.send",
        side_effect=requests.RequestException,
    )
    def test_requests_exception_without_response(self, *_, **__):
        with self.assertRaises(requests.RequestException):
            self.perform_request(self.URL)

        span = self.assert_span()
        self.assertEqual(
            span.attributes,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_URL: self.URL,
            },
        )
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

    mocked_response = requests.Response()
    mocked_response.status_code = 500
    mocked_response.reason = "Internal Server Error"

    @mock.patch(
        "requests.adapters.HTTPAdapter.send",
        side_effect=InvalidResponseObjectException,
    )
    def test_requests_exception_without_proper_response_type(self, *_, **__):
        with self.assertRaises(InvalidResponseObjectException):
            self.perform_request(self.URL)

        span = self.assert_span()
        self.assertEqual(
            span.attributes,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_URL: self.URL,
            },
        )
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

    mocked_response = requests.Response()
    mocked_response.status_code = 500
    mocked_response.reason = "Internal Server Error"

    @mock.patch(
        "requests.adapters.HTTPAdapter.send",
        side_effect=requests.RequestException(response=mocked_response),
    )
    def test_requests_exception_with_response(self, *_, **__):
        with self.assertRaises(requests.RequestException):
            self.perform_request(self.URL)

        span = self.assert_span()
        self.assertEqual(
            span.attributes,
            {
                SpanAttributes.HTTP_METHOD: "GET",
                SpanAttributes.HTTP_URL: self.URL,
                SpanAttributes.HTTP_STATUS_CODE: 500,
            },
        )
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

    @mock.patch("requests.adapters.HTTPAdapter.send", side_effect=Exception)
    def test_requests_basic_exception(self, *_, **__):
        with self.assertRaises(Exception):
            self.perform_request(self.URL)

        span = self.assert_span()
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

    @mock.patch(
        "requests.adapters.HTTPAdapter.send", side_effect=requests.Timeout
    )
    def test_requests_timeout_exception(self, *_, **__):
        with self.assertRaises(Exception):
            self.perform_request(self.URL)

        span = self.assert_span()
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

    def test_adapter_with_custom_response(self):
        response = Response()
        response.status_code = 210
        response.reason = "hello adapter"
        response.raw = TransportMock()

        session = requests.Session()
        session.mount(self.URL, MyAdapter(response))

        self.perform_request(self.URL, session)
        span = self.assert_span()
        self.assertEqual(
            span.attributes,
            {
                "http.method": "GET",
                "http.url": self.URL,
                "http.status_code": 210,
            },
        )


class TestRequestsIntegration(RequestsIntegrationTestBase, TestBase):
    @staticmethod
    def perform_request(url: str, session: requests.Session = None):
        if session is None:
            return requests.get(url)
        return session.get(url)

    def test_invalid_url(self):
        url = "http://[::1/nope"

        with self.assertRaises(ValueError):
            requests.post(url)

        span = self.assert_span()

        self.assertEqual(span.name, "HTTP POST")
        self.assertEqual(
            span.attributes,
            {SpanAttributes.HTTP_METHOD: "POST", SpanAttributes.HTTP_URL: url},
        )
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

    def test_credential_removal(self):
        new_url = "http://username:password@httpbin.org/status/200"
        self.perform_request(new_url)
        span = self.assert_span()

        self.assertEqual(span.attributes[SpanAttributes.HTTP_URL], self.URL)

    def test_if_headers_equals_none(self):
        result = requests.get(self.URL, headers=None)
        self.assertEqual(result.text, "Hello!")
        self.assert_span()


class TestRequestsIntegrationPreparedRequest(
    RequestsIntegrationTestBase, TestBase
):
    @staticmethod
    def perform_request(url: str, session: requests.Session = None):
        if session is None:
            session = requests.Session()
        request = requests.Request("GET", url)
        prepared_request = session.prepare_request(request)
        return session.send(prepared_request)
