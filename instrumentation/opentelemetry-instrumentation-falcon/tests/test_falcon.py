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

from unittest.mock import Mock, patch

from falcon import testing

from opentelemetry.instrumentation.falcon import FalconInstrumentor
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace.status import StatusCode
from opentelemetry.util import ExcludeList

from .app import make_app


class TestFalconInstrumentation(TestBase):
    def setUp(self):
        super().setUp()
        FalconInstrumentor().instrument()
        self.app = make_app()

    def client(self):
        return testing.TestClient(self.app)

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FalconInstrumentor().uninstrument()

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
        self.assertEqual(
            span.name, "HelloWorldResource.on_{0}".format(method.lower())
        )
        self.assertEqual(span.status.status_code, StatusCode.UNSET)
        self.assert_span_has_attributes(
            span,
            {
                "component": "http",
                "http.method": method,
                "http.server_name": "falconframework.org",
                "http.scheme": "http",
                "host.port": 80,
                "http.host": "falconframework.org",
                "http.target": "/",
                "net.peer.ip": "127.0.0.1",
                "net.peer.port": "65133",
                "http.flavor": "1.1",
                "falcon.resource": "HelloWorldResource",
                "http.status_text": "Created",
                "http.status_code": 201,
            },
        )
        self.memory_exporter.clear()

    def test_404(self):
        self.client().simulate_get("/does-not-exist")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.name, "HTTP GET")
        self.assertEqual(span.status.status_code, StatusCode.ERROR)
        self.assert_span_has_attributes(
            span,
            {
                "component": "http",
                "http.method": "GET",
                "http.server_name": "falconframework.org",
                "http.scheme": "http",
                "host.port": 80,
                "http.host": "falconframework.org",
                "http.target": "/",
                "net.peer.ip": "127.0.0.1",
                "net.peer.port": "65133",
                "http.flavor": "1.1",
                "http.status_text": "Not Found",
                "http.status_code": 404,
            },
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
        self.assert_span_has_attributes(
            span,
            {
                "component": "http",
                "http.method": "GET",
                "http.server_name": "falconframework.org",
                "http.scheme": "http",
                "host.port": 80,
                "http.host": "falconframework.org",
                "http.target": "/",
                "net.peer.ip": "127.0.0.1",
                "net.peer.port": "65133",
                "http.flavor": "1.1",
                "http.status_code": 500,
            },
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

    @patch(
        "opentelemetry.instrumentation.falcon._excluded_urls",
        ExcludeList(["ping"]),
    )
    def test_exclude_lists(self):
        self.client().simulate_get(path="/ping")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 0)

        self.client().simulate_get(path="/hello")
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_traced_request_attributes(self):
        self.client().simulate_get(path="/hello?q=abc")
        span = self.memory_exporter.get_finished_spans()[0]
        self.assertNotIn("query_string", span.attributes)
        self.memory_exporter.clear()

        middleware = self.app._middleware[0][  # pylint:disable=W0212
            0
        ].__self__
        with patch.object(
            middleware, "_traced_request_attrs", ["query_string"]
        ):
            self.client().simulate_get(path="/hello?q=abc")
            span = self.memory_exporter.get_finished_spans()[0]
            self.assertIn("query_string", span.attributes)
            self.assertEqual(span.attributes["query_string"], "q=abc")

    def test_traced_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        mock_tracer.use_span.return_value.__enter__ = mock_span
        mock_tracer.use_span.return_value.__exit__ = mock_span
        with patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            self.client().simulate_get(path="/hello?q=abc")
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)
