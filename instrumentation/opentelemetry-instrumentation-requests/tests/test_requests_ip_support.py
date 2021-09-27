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

import requests

from opentelemetry import trace
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.test.httptest import HttpTestBase
from opentelemetry.test.test_base import TestBase
from opentelemetry.util.http.httplib import HttpClientInstrumentor


class TestURLLib3InstrumentorWithRealSocket(HttpTestBase, TestBase):
    def setUp(self):
        super().setUp()
        self.assert_ip = self.server.server_address[0]
        self.http_host = ":".join(map(str, self.server.server_address[:2]))
        self.http_url_base = "http://" + self.http_host
        self.http_url = self.http_url_base + "/status/200"
        HttpClientInstrumentor().instrument()
        RequestsInstrumentor().instrument()

    def tearDown(self):
        super().tearDown()
        HttpClientInstrumentor().uninstrument()
        RequestsInstrumentor().uninstrument()

    @staticmethod
    def perform_request(url: str) -> requests.Response:
        return requests.get(url)

    def test_basic_http_success(self):
        response = self.perform_request(self.http_url)
        self.assert_success_span(response)

    def test_basic_http_success_using_connection_pool(self):
        with requests.Session() as session:
            response = session.get(self.http_url)

            self.assert_success_span(response)

            # Test that when re-using an existing connection, everything still works.
            # Especially relevant for IP capturing.
            response = session.get(self.http_url)

            self.assert_success_span(response)

    def assert_span(self, num_spans=1):  # TODO: Move this to TestBase
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(num_spans, len(span_list))
        if num_spans == 0:
            return None
        self.memory_exporter.clear()
        if num_spans == 1:
            return span_list[0]
        return span_list

    def assert_success_span(self, response: requests.Response):
        self.assertEqual("Hello!", response.text)

        span = self.assert_span()
        self.assertIs(trace.SpanKind.CLIENT, span.kind)
        self.assertEqual("HTTP GET", span.name)

        attributes = {
            "http.status_code": 200,
            "net.peer.ip": self.assert_ip,
        }
        self.assertGreaterEqual(span.attributes.items(), attributes.items())
