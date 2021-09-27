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

from http.client import HTTPConnection, HTTPResponse, HTTPSConnection

from opentelemetry import trace
from opentelemetry.test.httptest import HttpTestBase
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace.span import INVALID_SPAN
from opentelemetry.util.http.httplib import (
    HttpClientInstrumentor,
    set_ip_on_next_http_connection,
)

# pylint: disable=too-many-public-methods


class TestHttpBase(TestBase, HttpTestBase):
    def setUp(self):
        super().setUp()
        HttpClientInstrumentor().instrument()
        self.server_thread, self.server = self.run_server()

    def tearDown(self):
        HttpClientInstrumentor().uninstrument()
        self.server.shutdown()
        self.server_thread.join()
        super().tearDown()

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

    def test_basic(self):
        resp, body = self.perform_request()
        assert resp.status == 200
        assert body == b"Hello!"
        self.assert_span(num_spans=0)

    def test_basic_with_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span(
            "HTTP GET"
        ) as span, set_ip_on_next_http_connection(span):
            resp, body = self.perform_request()
        assert resp.status == 200
        assert body == b"Hello!"
        span = self.assert_span(num_spans=1)
        self.assertEqual(span.attributes, {"net.peer.ip": "127.0.0.1"})

    def test_with_nested_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span(
            "requests HTTP GET"
        ) as span, set_ip_on_next_http_connection(span):
            with tracer.start_as_current_span(
                "urllib3 HTTP GET"
            ) as span2, set_ip_on_next_http_connection(span2):
                resp, body = self.perform_request()
        assert resp.status == 200
        assert body == b"Hello!"
        for span in self.assert_span(num_spans=2):
            self.assertEqual(span.attributes, {"net.peer.ip": "127.0.0.1"})

    def test_with_nested_nonrecording_span(self):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span(
            "requests HTTP GET"
        ) as span, set_ip_on_next_http_connection(span):
            with trace.use_span(INVALID_SPAN), set_ip_on_next_http_connection(
                INVALID_SPAN
            ):
                resp, body = self.perform_request()
        assert resp.status == 200
        assert body == b"Hello!"
        span = self.assert_span(num_spans=1)
        self.assertEqual(span.attributes, {"net.peer.ip": "127.0.0.1"})

    def test_with_only_nonrecording_span(self):
        with trace.use_span(INVALID_SPAN), set_ip_on_next_http_connection(
            INVALID_SPAN
        ):
            resp, body = self.perform_request()
        assert resp.status == 200
        assert body == b"Hello!"
        self.assert_span(num_spans=0)

    def perform_request(self, secure=False) -> HTTPResponse:
        conn_cls = HTTPSConnection if secure else HTTPConnection
        conn = conn_cls(self.server.server_address[0], self.server.server_port)
        resp = None
        try:
            conn.request("GET", "/", headers={"Connection": "close"})
            resp = conn.getresponse()
            return resp, resp.read()
        finally:
            if resp:
                resp.close()
            conn.close()

    def test_uninstrument(self):
        HttpClientInstrumentor().uninstrument()

        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span(
            "HTTP GET"
        ) as span, set_ip_on_next_http_connection(span):
            body = self.perform_request()[1]
        self.assertEqual(b"Hello!", body)

        # We should have a span, but it should have no attributes
        self.assertFalse(self.assert_span(num_spans=1).attributes)

        # instrument again to avoid warning message on tearDown
        HttpClientInstrumentor().instrument()
