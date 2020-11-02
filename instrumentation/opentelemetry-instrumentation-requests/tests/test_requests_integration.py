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

import opentelemetry.instrumentation.requests
from opentelemetry import context, propagators, trace
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.sdk import resources
from opentelemetry.sdk.util import get_dict_as_key
from opentelemetry.test.mock_textmap import MockTextMapPropagator
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace.status import StatusCode


class RequestsIntegrationTestBase(abc.ABC):
    # pylint: disable=no-member

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
                "component": "http",
                "http.method": "GET",
                "http.url": self.URL,
                "http.status_code": 200,
                "http.status_text": "OK",
            },
        )

        self.assertIs(span.status.status_code, trace.status.StatusCode.UNSET)

        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.requests
        )

        self.assertIsNotNone(RequestsInstrumentor().meter)
        self.assertEqual(len(RequestsInstrumentor().meter.metrics), 1)
        recorder = RequestsInstrumentor().meter.metrics.pop()
        match_key = get_dict_as_key(
            {
                "http.flavor": "1.1",
                "http.method": "GET",
                "http.status_code": "200",
                "http.url": "http://httpbin.org/status/200",
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

    def test_not_foundbasic(self):
        url_404 = "http://httpbin.org/status/404"
        httpretty.register_uri(
            httpretty.GET, url_404, status=404,
        )
        result = self.perform_request(url_404)
        self.assertEqual(result.status_code, 404)

        span = self.assert_span()

        self.assertEqual(span.attributes.get("http.status_code"), 404)
        self.assertEqual(span.attributes.get("http.status_text"), "Not Found")

        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
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
            context.set_value("suppress_instrumentation", True)
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
            # original_tracer_provider returns a default tracer provider, which
            # in turn will return an INVALID_SPAN, which is always not recording
            RequestsInstrumentor().instrument(
                tracer_provider=self.original_tracer_provider
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
        previous_propagator = propagators.get_global_textmap()
        try:
            propagators.set_global_textmap(MockTextMapPropagator())
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
            propagators.set_global_textmap(previous_propagator)

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
            {"component": "http", "http.method": "GET", "http.url": self.URL},
        )
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

        self.assertIsNotNone(RequestsInstrumentor().meter)
        self.assertEqual(len(RequestsInstrumentor().meter.metrics), 1)
        recorder = RequestsInstrumentor().meter.metrics.pop()
        match_key = get_dict_as_key(
            {
                "http.method": "GET",
                "http.url": "http://httpbin.org/status/200",
            }
        )
        for key in recorder.bound_instruments.keys():
            self.assertEqual(key, match_key)
            # pylint: disable=protected-access
            bound = recorder.bound_instruments.get(key)
            for view_data in bound.view_datas:
                self.assertEqual(view_data.labels, key)
                self.assertEqual(view_data.aggregator.current.count, 1)

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
                "component": "http",
                "http.method": "GET",
                "http.url": self.URL,
                "http.status_code": 500,
                "http.status_text": "Internal Server Error",
            },
        )
        self.assertEqual(span.status.status_code, StatusCode.ERROR)
        self.assertIsNotNone(RequestsInstrumentor().meter)
        self.assertEqual(len(RequestsInstrumentor().meter.metrics), 1)
        recorder = RequestsInstrumentor().meter.metrics.pop()
        match_key = get_dict_as_key(
            {
                "http.method": "GET",
                "http.status_code": "500",
                "http.url": "http://httpbin.org/status/200",
            }
        )
        for key in recorder.bound_instruments.keys():
            self.assertEqual(key, match_key)
            # pylint: disable=protected-access
            bound = recorder.bound_instruments.get(key)
            for view_data in bound.view_datas:
                self.assertEqual(view_data.labels, key)
                self.assertEqual(view_data.aggregator.current.count, 1)

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
            {"component": "http", "http.method": "POST", "http.url": url},
        )
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

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
