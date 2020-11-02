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

import sys
import unittest
import unittest.mock as mock
import wsgiref.util as wsgiref_util
from urllib.parse import urlsplit

import opentelemetry.instrumentation.wsgi as otel_wsgi
from opentelemetry import trace as trace_api
from opentelemetry.test.wsgitestutil import WsgiTestBase
from opentelemetry.trace.status import StatusCode


class Response:
    def __init__(self):
        self.iter = iter([b"*"])
        self.close_calls = 0

    def __iter__(self):
        return self

    def __next__(self):
        return next(self.iter)

    def close(self):
        self.close_calls += 1


def simple_wsgi(environ, start_response):
    assert isinstance(environ, dict)
    start_response("200 OK", [("Content-Type", "text/plain")])
    return [b"*"]


def create_iter_wsgi(response):
    def iter_wsgi(environ, start_response):
        assert isinstance(environ, dict)
        start_response("200 OK", [("Content-Type", "text/plain")])
        return response

    return iter_wsgi


def create_gen_wsgi(response):
    def gen_wsgi(environ, start_response):
        result = create_iter_wsgi(response)(environ, start_response)
        yield from result
        getattr(result, "close", lambda: None)()

    return gen_wsgi


def error_wsgi(environ, start_response):
    assert isinstance(environ, dict)
    try:
        raise ValueError
    except ValueError:
        exc_info = sys.exc_info()
    start_response("200 OK", [("Content-Type", "text/plain")], exc_info)
    exc_info = None
    return [b"*"]


def error_wsgi_unhandled(environ, start_response):
    assert isinstance(environ, dict)
    raise ValueError


class TestWsgiApplication(WsgiTestBase):
    def validate_response(
        self, response, error=None, span_name="HTTP GET", http_method="GET"
    ):
        while True:
            try:
                value = next(response)
                self.assertEqual(value, b"*")
            except StopIteration:
                break

        self.assertEqual(self.status, "200 OK")
        self.assertEqual(
            self.response_headers, [("Content-Type", "text/plain")]
        )
        if error:
            self.assertIs(self.exc_info[0], error)
            self.assertIsInstance(self.exc_info[1], error)
            self.assertIsNotNone(self.exc_info[2])
        else:
            self.assertIsNone(self.exc_info)

        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(span_list[0].name, span_name)
        self.assertEqual(span_list[0].kind, trace_api.SpanKind.SERVER)
        expected_attributes = {
            "component": "http",
            "http.server_name": "127.0.0.1",
            "http.scheme": "http",
            "host.port": 80,
            "http.host": "127.0.0.1",
            "http.flavor": "1.0",
            "http.url": "http://127.0.0.1/",
            "http.status_text": "OK",
            "http.status_code": 200,
        }
        if http_method is not None:
            expected_attributes["http.method"] = http_method
        self.assertEqual(span_list[0].attributes, expected_attributes)

    def test_basic_wsgi_call(self):
        app = otel_wsgi.OpenTelemetryMiddleware(simple_wsgi)
        response = app(self.environ, self.start_response)
        self.validate_response(response)

    def test_wsgi_not_recording(self):
        mock_tracer = mock.Mock()
        mock_span = mock.Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        mock_tracer.use_span.return_value.__enter__ = mock_span
        mock_tracer.use_span.return_value.__exit__ = mock_span
        with mock.patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            app = otel_wsgi.OpenTelemetryMiddleware(simple_wsgi)
            # pylint: disable=W0612
            response = app(self.environ, self.start_response)  # noqa: F841
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_wsgi_iterable(self):
        original_response = Response()
        iter_wsgi = create_iter_wsgi(original_response)
        app = otel_wsgi.OpenTelemetryMiddleware(iter_wsgi)
        response = app(self.environ, self.start_response)
        # Verify that start_response has been called
        self.assertTrue(self.status)
        self.validate_response(response)

        # Verify that close has been called exactly once
        self.assertEqual(1, original_response.close_calls)

    def test_wsgi_generator(self):
        original_response = Response()
        gen_wsgi = create_gen_wsgi(original_response)
        app = otel_wsgi.OpenTelemetryMiddleware(gen_wsgi)
        response = app(self.environ, self.start_response)
        # Verify that start_response has not been called
        self.assertIsNone(self.status)
        self.validate_response(response)

        # Verify that close has been called exactly once
        self.assertEqual(original_response.close_calls, 1)

    def test_wsgi_exc_info(self):
        app = otel_wsgi.OpenTelemetryMiddleware(error_wsgi)
        response = app(self.environ, self.start_response)
        self.validate_response(response, error=ValueError)

    def test_wsgi_internal_error(self):
        app = otel_wsgi.OpenTelemetryMiddleware(error_wsgi_unhandled)
        self.assertRaises(ValueError, app, self.environ, self.start_response)
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
        self.assertEqual(
            span_list[0].status.status_code, StatusCode.ERROR,
        )

    def test_override_span_name(self):
        """Test that span_names can be overwritten by our callback function."""
        span_name = "Dymaxion"

        def get_predefined_span_name(scope):
            # pylint: disable=unused-argument
            return span_name

        app = otel_wsgi.OpenTelemetryMiddleware(
            simple_wsgi, name_callback=get_predefined_span_name
        )
        response = app(self.environ, self.start_response)
        self.validate_response(response, span_name=span_name)

    def test_default_span_name_missing_request_method(self):
        """Test that default span_names with missing request method."""
        self.environ.pop("REQUEST_METHOD")
        app = otel_wsgi.OpenTelemetryMiddleware(simple_wsgi)
        response = app(self.environ, self.start_response)
        self.validate_response(response, span_name="HTTP", http_method=None)


class TestWsgiAttributes(unittest.TestCase):
    def setUp(self):
        self.environ = {}
        wsgiref_util.setup_testing_defaults(self.environ)
        self.span = mock.create_autospec(trace_api.Span, spec_set=True)

    def test_request_attributes(self):
        self.environ["QUERY_STRING"] = "foo=bar"

        attrs = otel_wsgi.collect_request_attributes(self.environ)
        self.assertDictEqual(
            attrs,
            {
                "component": "http",
                "http.method": "GET",
                "http.host": "127.0.0.1",
                "http.url": "http://127.0.0.1/?foo=bar",
                "host.port": 80,
                "http.scheme": "http",
                "http.server_name": "127.0.0.1",
                "http.flavor": "1.0",
            },
        )

    def validate_url(self, expected_url, raw=False, has_host=True):
        parts = urlsplit(expected_url)
        expected = {
            "http.scheme": parts.scheme,
            "host.port": parts.port or (80 if parts.scheme == "http" else 443),
            "http.server_name": parts.hostname,  # Not true in the general case, but for all tests.
        }
        if raw:
            expected["http.target"] = expected_url.split(parts.netloc, 1)[1]
        else:
            expected["http.url"] = expected_url
        if has_host:
            expected["http.host"] = parts.hostname

        attrs = otel_wsgi.collect_request_attributes(self.environ)
        self.assertGreaterEqual(
            attrs.items(), expected.items(), expected_url + " expected."
        )

    def test_request_attributes_with_partial_raw_uri(self):
        self.environ["RAW_URI"] = "/#top"
        self.validate_url("http://127.0.0.1/#top", raw=True)

    def test_request_attributes_with_partial_raw_uri_and_nonstandard_port(
        self,
    ):
        self.environ["RAW_URI"] = "/?"
        del self.environ["HTTP_HOST"]
        self.environ["SERVER_PORT"] = "8080"
        self.validate_url("http://127.0.0.1:8080/?", raw=True, has_host=False)

    def test_https_uri_port(self):
        del self.environ["HTTP_HOST"]
        self.environ["SERVER_PORT"] = "443"
        self.environ["wsgi.url_scheme"] = "https"
        self.validate_url("https://127.0.0.1/", has_host=False)

        self.environ["SERVER_PORT"] = "8080"
        self.validate_url("https://127.0.0.1:8080/", has_host=False)

        self.environ["SERVER_PORT"] = "80"
        self.validate_url("https://127.0.0.1:80/", has_host=False)

    def test_http_uri_port(self):
        del self.environ["HTTP_HOST"]
        self.environ["SERVER_PORT"] = "80"
        self.environ["wsgi.url_scheme"] = "http"
        self.validate_url("http://127.0.0.1/", has_host=False)

        self.environ["SERVER_PORT"] = "8080"
        self.validate_url("http://127.0.0.1:8080/", has_host=False)

        self.environ["SERVER_PORT"] = "443"
        self.validate_url("http://127.0.0.1:443/", has_host=False)

    def test_request_attributes_with_nonstandard_port_and_no_host(self):
        del self.environ["HTTP_HOST"]
        self.environ["SERVER_PORT"] = "8080"
        self.validate_url("http://127.0.0.1:8080/", has_host=False)

        self.environ["SERVER_PORT"] = "443"
        self.validate_url("http://127.0.0.1:443/", has_host=False)

    def test_request_attributes_with_conflicting_nonstandard_port(self):
        self.environ[
            "HTTP_HOST"
        ] += ":8080"  # Note that we do not correct SERVER_PORT
        expected = {
            "http.host": "127.0.0.1:8080",
            "http.url": "http://127.0.0.1:8080/",
            "host.port": 80,
        }
        self.assertGreaterEqual(
            otel_wsgi.collect_request_attributes(self.environ).items(),
            expected.items(),
        )

    def test_request_attributes_with_faux_scheme_relative_raw_uri(self):
        self.environ["RAW_URI"] = "//127.0.0.1/?"
        self.validate_url("http://127.0.0.1//127.0.0.1/?", raw=True)

    def test_request_attributes_pathless(self):
        self.environ["RAW_URI"] = ""
        expected = {"http.target": ""}
        self.assertGreaterEqual(
            otel_wsgi.collect_request_attributes(self.environ).items(),
            expected.items(),
        )

    def test_request_attributes_with_full_request_uri(self):
        self.environ["HTTP_HOST"] = "127.0.0.1:8080"
        self.environ["REQUEST_METHOD"] = "CONNECT"
        self.environ[
            "REQUEST_URI"
        ] = "127.0.0.1:8080"  # Might happen in a CONNECT request
        expected = {
            "http.host": "127.0.0.1:8080",
            "http.target": "127.0.0.1:8080",
        }
        self.assertGreaterEqual(
            otel_wsgi.collect_request_attributes(self.environ).items(),
            expected.items(),
        )

    def test_http_user_agent_attribute(self):
        self.environ["HTTP_USER_AGENT"] = "test-useragent"
        expected = {"http.user_agent": "test-useragent"}
        self.assertGreaterEqual(
            otel_wsgi.collect_request_attributes(self.environ).items(),
            expected.items(),
        )

    def test_response_attributes(self):
        otel_wsgi.add_response_attributes(self.span, "404 Not Found", {})
        expected = (
            mock.call("http.status_code", 404),
            mock.call("http.status_text", "Not Found"),
        )
        self.assertEqual(self.span.set_attribute.call_count, len(expected))
        self.span.set_attribute.assert_has_calls(expected, any_order=True)

    def test_response_attributes_invalid_status_code(self):
        otel_wsgi.add_response_attributes(self.span, "Invalid Status Code", {})
        self.assertEqual(self.span.set_attribute.call_count, 1)
        self.span.set_attribute.assert_called_with(
            "http.status_text", "Status Code"
        )


if __name__ == "__main__":
    unittest.main()
