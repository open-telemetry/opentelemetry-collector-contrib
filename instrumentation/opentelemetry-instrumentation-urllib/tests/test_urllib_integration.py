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
import socket
import urllib
from http.client import HTTPResponse
from unittest import mock
from urllib import request
from urllib.error import HTTPError
from urllib.request import OpenerDirector

import httpretty

import opentelemetry.instrumentation.urllib  # pylint: disable=no-name-in-module,import-error
from opentelemetry import context, propagators, trace
from opentelemetry.instrumentation.urllib import (  # pylint: disable=no-name-in-module,import-error
    URLLibInstrumentor,
)
from opentelemetry.sdk import resources
from opentelemetry.sdk.util import get_dict_as_key
from opentelemetry.test.mock_textmap import MockTextMapPropagator
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace.status import StatusCode


class RequestsIntegrationTestBase(abc.ABC):
    # pylint: disable=no-member

    URL = "http://httpbin.org/status/200"
    URL_TIMEOUT = "http://httpbin.org/timeout/0"
    URL_EXCEPTION = "http://httpbin.org/exception/0"

    # pylint: disable=invalid-name
    def setUp(self):
        super().setUp()
        URLLibInstrumentor().instrument()
        httpretty.enable()
        httpretty.register_uri(httpretty.GET, self.URL, body=b"Hello!")
        httpretty.register_uri(
            httpretty.GET,
            self.URL_TIMEOUT,
            body=self.timeout_exception_callback,
        )
        httpretty.register_uri(
            httpretty.GET,
            self.URL_EXCEPTION,
            body=self.base_exception_callback,
        )
        httpretty.register_uri(
            httpretty.GET, "http://httpbin.org/status/500", status=500,
        )

    # pylint: disable=invalid-name
    def tearDown(self):
        super().tearDown()
        URLLibInstrumentor().uninstrument()
        httpretty.disable()

    @staticmethod
    def timeout_exception_callback(*_, **__):
        raise socket.timeout

    @staticmethod
    def base_exception_callback(*_, **__):
        raise Exception("test")

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
    def perform_request(url: str, opener: OpenerDirector = None):
        pass

    def test_basic(self):
        result = self.perform_request(self.URL)

        self.assertEqual(result.read(), b"Hello!")
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
            span, opentelemetry.instrumentation.urllib
        )

        self.assertIsNotNone(URLLibInstrumentor().meter)
        self.assertEqual(len(URLLibInstrumentor().meter.instruments), 1)
        recorder = list(URLLibInstrumentor().meter.instruments.values())[0]
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

    def test_name_callback(self):
        def name_callback(method, url):
            return "GET" + url

        URLLibInstrumentor().uninstrument()
        URLLibInstrumentor().instrument(name_callback=name_callback)
        result = self.perform_request(self.URL)

        self.assertEqual(result.read(), b"Hello!")
        span = self.assert_span()

        self.assertEqual(span.name, "GET" + self.URL)

    def test_name_callback_default(self):
        def name_callback(method, url):
            return 123

        URLLibInstrumentor().uninstrument()
        URLLibInstrumentor().instrument(name_callback=name_callback)
        result = self.perform_request(self.URL)
        self.assertEqual(result.read(), b"Hello!")
        span = self.assert_span()

        self.assertEqual(span.name, "HTTP GET")

    def test_not_foundbasic(self):
        url_404 = "http://httpbin.org/status/404/"
        httpretty.register_uri(
            httpretty.GET, url_404, status=404,
        )
        exception = None
        try:
            self.perform_request(url_404)
        except Exception as err:  # pylint: disable=broad-except
            exception = err
        code = exception.code
        self.assertEqual(code, 404)

        span = self.assert_span()

        self.assertEqual(span.attributes.get("http.status_code"), 404)
        self.assertEqual(span.attributes.get("http.status_text"), "Not Found")

        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
        )

    def test_uninstrument(self):
        URLLibInstrumentor().uninstrument()
        result = self.perform_request(self.URL)
        self.assertEqual(result.read(), b"Hello!")
        self.assert_span(num_spans=0)
        # instrument again to avoid annoying warning message
        URLLibInstrumentor().instrument()

    def test_uninstrument_session(self):
        clienr1 = urllib.request.build_opener()
        URLLibInstrumentor().uninstrument_opener(clienr1)

        result = self.perform_request(self.URL, clienr1)
        self.assertEqual(result.read(), b"Hello!")
        self.assert_span(num_spans=0)

        # Test that other sessions as well as global requests is still
        # instrumented
        opener2 = urllib.request.build_opener()
        result = self.perform_request(self.URL, opener2)
        self.assertEqual(result.read(), b"Hello!")
        self.assert_span()

        self.memory_exporter.clear()

        result = self.perform_request(self.URL)
        self.assertEqual(result.read(), b"Hello!")
        self.assert_span()

    def test_suppress_instrumentation(self):
        token = context.attach(
            context.set_value("suppress_instrumentation", True)
        )
        try:
            result = self.perform_request(self.URL)
            self.assertEqual(result.read(), b"Hello!")
        finally:
            context.detach(token)

        self.assert_span(num_spans=0)

    def test_not_recording(self):
        with mock.patch("opentelemetry.trace.INVALID_SPAN") as mock_span:
            URLLibInstrumentor().uninstrument()
            # original_tracer_provider returns a default tracer provider, which
            # in turn will return an INVALID_SPAN, which is always not recording
            URLLibInstrumentor().instrument(
                tracer_provider=self.original_tracer_provider
            )
            mock_span.is_recording.return_value = False
            result = self.perform_request(self.URL)
            self.assertEqual(result.read(), b"Hello!")
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
            self.assertEqual(result.read(), b"Hello!")

            span = self.assert_span()

            headers_ = dict(httpretty.last_request().headers)
            headers = {}
            for k, v in headers_.items():
                headers[k.lower()] = v

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
        URLLibInstrumentor().uninstrument()

        def span_callback(span, result: HTTPResponse):
            span.set_attribute("http.response.body", result.read())

        URLLibInstrumentor().instrument(
            tracer_provider=self.tracer_provider, span_callback=span_callback,
        )

        result = self.perform_request(self.URL)

        self.assertEqual(result.read(), b"")

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
        URLLibInstrumentor().uninstrument()
        URLLibInstrumentor().instrument(tracer_provider=tracer_provider)

        result = self.perform_request(self.URL)
        self.assertEqual(result.read(), b"Hello!")

        span = self.assert_span(exporter=exporter)
        self.assertIs(span.resource, resource)

    def test_requests_exception_with_response(self, *_, **__):

        with self.assertRaises(HTTPError):
            self.perform_request("http://httpbin.org/status/500")

        span = self.assert_span()
        self.assertEqual(
            dict(span.attributes),
            {
                "component": "http",
                "http.method": "GET",
                "http.url": "http://httpbin.org/status/500",
                "http.status_code": 500,
                "http.status_text": "Internal Server Error",
            },
        )
        self.assertEqual(span.status.status_code, StatusCode.ERROR)
        self.assertIsNotNone(URLLibInstrumentor().meter)
        self.assertEqual(len(URLLibInstrumentor().meter.instruments), 1)
        recorder = list(URLLibInstrumentor().meter.instruments.values())[0]
        match_key = get_dict_as_key(
            {
                "http.method": "GET",
                "http.status_code": "500",
                "http.url": "http://httpbin.org/status/500",
                "http.flavor": "1.1",
            }
        )
        for key in recorder.bound_instruments.keys():
            self.assertEqual(key, match_key)
            # pylint: disable=protected-access
            bound = recorder.bound_instruments.get(key)
            for view_data in bound.view_datas:
                self.assertEqual(view_data.labels, key)
                self.assertEqual(view_data.aggregator.current.count, 1)

    def test_requests_basic_exception(self, *_, **__):
        with self.assertRaises(Exception):
            self.perform_request(self.URL_EXCEPTION)

        span = self.assert_span()
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

    def test_requests_timeout_exception(self, *_, **__):
        with self.assertRaises(Exception):
            opener = urllib.request.build_opener()
            opener.open(self.URL_TIMEOUT, timeout=0.0001)

        span = self.assert_span()
        self.assertEqual(span.status.status_code, StatusCode.ERROR)


class TestRequestsIntegration(RequestsIntegrationTestBase, TestBase):
    @staticmethod
    def perform_request(url: str, opener: OpenerDirector = None):
        if not opener:
            opener = urllib.request.build_opener()
        return opener.open(fullurl=url)

    def test_invalid_url(self):
        url = "http://[::1/nope"

        with self.assertRaises(ValueError):
            request.Request(url, method="POST")

        self.assert_span(num_spans=0)

    def test_if_headers_equals_none(self):
        result = self.perform_request(self.URL)
        self.assertEqual(result.read(), b"Hello!")
        self.assert_span()
