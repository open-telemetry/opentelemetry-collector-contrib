# Copyright 2020, OpenTelemetry Authors
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

import asyncio
import contextlib
import typing
import unittest
import urllib.parse
from http import HTTPStatus
from unittest import mock

import aiohttp
import aiohttp.test_utils
import yarl
from pkg_resources import iter_entry_points

from opentelemetry import context
from opentelemetry.instrumentation import aiohttp_client
from opentelemetry.instrumentation.aiohttp_client import (
    AioHttpClientInstrumentor,
)
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace.status import StatusCode


def run_with_test_server(
    runnable: typing.Callable, url: str, handler: typing.Callable
) -> typing.Tuple[str, int]:
    async def do_request():
        app = aiohttp.web.Application()
        parsed_url = urllib.parse.urlparse(url)
        app.add_routes([aiohttp.web.get(parsed_url.path, handler)])
        app.add_routes([aiohttp.web.post(parsed_url.path, handler)])
        app.add_routes([aiohttp.web.patch(parsed_url.path, handler)])

        with contextlib.suppress(aiohttp.ClientError):
            async with aiohttp.test_utils.TestServer(app) as server:
                netloc = (server.host, server.port)
                await server.start_server()
                await runnable(server)
        return netloc

    loop = asyncio.get_event_loop()
    return loop.run_until_complete(do_request())


class TestAioHttpIntegration(TestBase):
    def assert_spans(self, spans):
        self.assertEqual(
            [
                (
                    span.name,
                    (span.status.status_code, span.status.description),
                    dict(span.attributes),
                )
                for span in self.memory_exporter.get_finished_spans()
            ],
            spans,
        )

    def test_url_path_span_name(self):
        for url, expected in (
            (
                yarl.URL("http://hostname.local:1234/some/path?query=params"),
                "/some/path",
            ),
            (yarl.URL("http://hostname.local:1234"), "/"),
        ):
            with self.subTest(url=url):
                params = aiohttp.TraceRequestStartParams("METHOD", url, {})
                actual = aiohttp_client.url_path_span_name(params)
                self.assertEqual(actual, expected)
                self.assertIsInstance(actual, str)

    @staticmethod
    def _http_request(
        trace_config,
        url: str,
        method: str = "GET",
        status_code: int = HTTPStatus.OK,
        request_handler: typing.Callable = None,
        **kwargs
    ) -> typing.Tuple[str, int]:
        """Helper to start an aiohttp test server and send an actual HTTP request to it."""

        async def default_handler(request):
            assert "traceparent" in request.headers
            return aiohttp.web.Response(status=int(status_code))

        async def client_request(server: aiohttp.test_utils.TestServer):
            async with aiohttp.test_utils.TestClient(
                server, trace_configs=[trace_config]
            ) as client:
                await client.request(
                    method, url, trace_request_ctx={}, **kwargs
                )

        handler = request_handler or default_handler
        return run_with_test_server(client_request, url, handler)

    def test_status_codes(self):
        for status_code, span_status in (
            (HTTPStatus.OK, StatusCode.UNSET),
            (HTTPStatus.TEMPORARY_REDIRECT, StatusCode.UNSET),
            (HTTPStatus.SERVICE_UNAVAILABLE, StatusCode.ERROR),
            (HTTPStatus.GATEWAY_TIMEOUT, StatusCode.ERROR,),
        ):
            with self.subTest(status_code=status_code):
                host, port = self._http_request(
                    trace_config=aiohttp_client.create_trace_config(),
                    url="/test-path?query=param#foobar",
                    status_code=status_code,
                )

                self.assert_spans(
                    [
                        (
                            "HTTP GET",
                            (span_status, None),
                            {
                                "component": "http",
                                "http.method": "GET",
                                "http.url": "http://{}:{}/test-path?query=param#foobar".format(
                                    host, port
                                ),
                                "http.status_code": int(status_code),
                                "http.status_text": status_code.phrase,
                            },
                        )
                    ]
                )

                self.memory_exporter.clear()

    def test_not_recording(self):
        mock_tracer = mock.Mock()
        mock_span = mock.Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with mock.patch("opentelemetry.trace.get_tracer"):
            # pylint: disable=W0612
            host, port = self._http_request(
                trace_config=aiohttp_client.create_trace_config(),
                url="/test-path?query=param#foobar",
            )
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_span_name_option(self):
        for span_name, method, path, expected in (
            ("static", "POST", "/static-span-name", "static"),
            (
                lambda params: "{} - {}".format(
                    params.method, params.url.path
                ),
                "PATCH",
                "/some/path",
                "PATCH - /some/path",
            ),
        ):
            with self.subTest(span_name=span_name, method=method, path=path):
                host, port = self._http_request(
                    trace_config=aiohttp_client.create_trace_config(
                        span_name=span_name
                    ),
                    method=method,
                    url=path,
                    status_code=HTTPStatus.OK,
                )

                self.assert_spans(
                    [
                        (
                            expected,
                            (StatusCode.UNSET, None),
                            {
                                "component": "http",
                                "http.method": method,
                                "http.url": "http://{}:{}{}".format(
                                    host, port, path
                                ),
                                "http.status_code": int(HTTPStatus.OK),
                                "http.status_text": HTTPStatus.OK.phrase,
                            },
                        )
                    ]
                )
                self.memory_exporter.clear()

    def test_url_filter_option(self):
        # Strips all query params from URL before adding as a span attribute.
        def strip_query_params(url: yarl.URL) -> str:
            return str(url.with_query(None))

        host, port = self._http_request(
            trace_config=aiohttp_client.create_trace_config(
                url_filter=strip_query_params
            ),
            url="/some/path?query=param&other=param2",
            status_code=HTTPStatus.OK,
        )

        self.assert_spans(
            [
                (
                    "HTTP GET",
                    (StatusCode.UNSET, None),
                    {
                        "component": "http",
                        "http.method": "GET",
                        "http.url": "http://{}:{}/some/path".format(
                            host, port
                        ),
                        "http.status_code": int(HTTPStatus.OK),
                        "http.status_text": HTTPStatus.OK.phrase,
                    },
                )
            ]
        )

    def test_connection_errors(self):
        trace_configs = [aiohttp_client.create_trace_config()]

        for url, expected_status in (
            ("http://this-is-unknown.local/", StatusCode.ERROR),
            ("http://127.0.0.1:1/", StatusCode.ERROR),
        ):
            with self.subTest(expected_status=expected_status):

                async def do_request(url):
                    async with aiohttp.ClientSession(
                        trace_configs=trace_configs,
                    ) as session:
                        async with session.get(url):
                            pass

                loop = asyncio.get_event_loop()
                with self.assertRaises(aiohttp.ClientConnectorError):
                    loop.run_until_complete(do_request(url))

            self.assert_spans(
                [
                    (
                        "HTTP GET",
                        (expected_status, None),
                        {
                            "component": "http",
                            "http.method": "GET",
                            "http.url": url,
                        },
                    )
                ]
            )
            self.memory_exporter.clear()

    def test_timeout(self):
        async def request_handler(request):
            await asyncio.sleep(1)
            assert "traceparent" in request.headers
            return aiohttp.web.Response()

        host, port = self._http_request(
            trace_config=aiohttp_client.create_trace_config(),
            url="/test_timeout",
            request_handler=request_handler,
            timeout=aiohttp.ClientTimeout(sock_read=0.01),
        )

        self.assert_spans(
            [
                (
                    "HTTP GET",
                    (StatusCode.ERROR, None),
                    {
                        "component": "http",
                        "http.method": "GET",
                        "http.url": "http://{}:{}/test_timeout".format(
                            host, port
                        ),
                    },
                )
            ]
        )

    def test_too_many_redirects(self):
        async def request_handler(request):
            # Create a redirect loop.
            location = request.url
            assert "traceparent" in request.headers
            raise aiohttp.web.HTTPFound(location=location)

        host, port = self._http_request(
            trace_config=aiohttp_client.create_trace_config(),
            url="/test_too_many_redirects",
            request_handler=request_handler,
            max_redirects=2,
        )

        self.assert_spans(
            [
                (
                    "HTTP GET",
                    (StatusCode.ERROR, None),
                    {
                        "component": "http",
                        "http.method": "GET",
                        "http.url": "http://{}:{}/test_too_many_redirects".format(
                            host, port
                        ),
                    },
                )
            ]
        )


class TestAioHttpClientInstrumentor(TestBase):
    URL = "/test-path"

    def setUp(self):
        super().setUp()
        AioHttpClientInstrumentor().instrument()

    def tearDown(self):
        super().tearDown()
        AioHttpClientInstrumentor().uninstrument()

    @staticmethod
    # pylint:disable=unused-argument
    async def default_handler(request):
        return aiohttp.web.Response(status=int(200))

    @staticmethod
    def get_default_request(url: str = URL):
        async def default_request(server: aiohttp.test_utils.TestServer):
            async with aiohttp.test_utils.TestClient(server) as session:
                await session.get(url)

        return default_request

    def assert_spans(self, num_spans: int):
        finished_spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(num_spans, len(finished_spans))
        if num_spans == 0:
            return None
        if num_spans == 1:
            return finished_spans[0]
        return finished_spans

    def test_instrument(self):
        host, port = run_with_test_server(
            self.get_default_request(), self.URL, self.default_handler
        )
        span = self.assert_spans(1)
        self.assertEqual("http", span.attributes["component"])
        self.assertEqual("GET", span.attributes["http.method"])
        self.assertEqual(
            "http://{}:{}/test-path".format(host, port),
            span.attributes["http.url"],
        )
        self.assertEqual(200, span.attributes["http.status_code"])
        self.assertEqual("OK", span.attributes["http.status_text"])

    def test_instrument_with_existing_trace_config(self):
        trace_config = aiohttp.TraceConfig()

        async def create_session(server: aiohttp.test_utils.TestServer):
            async with aiohttp.test_utils.TestClient(
                server, trace_configs=[trace_config]
            ) as client:
                # pylint:disable=protected-access
                trace_configs = client.session._trace_configs
                self.assertEqual(2, len(trace_configs))
                self.assertTrue(trace_config in trace_configs)
                async with client as session:
                    await session.get(TestAioHttpClientInstrumentor.URL)

        run_with_test_server(create_session, self.URL, self.default_handler)
        self.assert_spans(1)

    def test_uninstrument(self):
        AioHttpClientInstrumentor().uninstrument()
        run_with_test_server(
            self.get_default_request(), self.URL, self.default_handler
        )

        self.assert_spans(0)

        AioHttpClientInstrumentor().instrument()
        run_with_test_server(
            self.get_default_request(), self.URL, self.default_handler
        )
        self.assert_spans(1)

    def test_uninstrument_session(self):
        async def uninstrument_request(server: aiohttp.test_utils.TestServer):
            client = aiohttp.test_utils.TestClient(server)
            AioHttpClientInstrumentor().uninstrument_session(client.session)
            async with client as session:
                await session.get(self.URL)

        run_with_test_server(
            uninstrument_request, self.URL, self.default_handler
        )
        self.assert_spans(0)

        run_with_test_server(
            self.get_default_request(), self.URL, self.default_handler
        )
        self.assert_spans(1)

    def test_suppress_instrumentation(self):
        token = context.attach(
            context.set_value("suppress_instrumentation", True)
        )
        try:
            run_with_test_server(
                self.get_default_request(), self.URL, self.default_handler
            )
        finally:
            context.detach(token)
        self.assert_spans(0)

    @staticmethod
    async def suppressed_request(server: aiohttp.test_utils.TestServer):
        async with aiohttp.test_utils.TestClient(server) as client:
            token = context.attach(
                context.set_value("suppress_instrumentation", True)
            )
            await client.get(TestAioHttpClientInstrumentor.URL)
            context.detach(token)

    def test_suppress_instrumentation_after_creation(self):
        run_with_test_server(
            self.suppressed_request, self.URL, self.default_handler
        )
        self.assert_spans(0)

    def test_suppress_instrumentation_with_server_exception(self):
        # pylint:disable=unused-argument
        async def raising_handler(request):
            raise aiohttp.web.HTTPFound(location=self.URL)

        run_with_test_server(
            self.suppressed_request, self.URL, raising_handler
        )
        self.assert_spans(0)

    def test_url_filter(self):
        def strip_query_params(url: yarl.URL) -> str:
            return str(url.with_query(None))

        AioHttpClientInstrumentor().uninstrument()
        AioHttpClientInstrumentor().instrument(url_filter=strip_query_params)

        url = "/test-path?query=params"
        host, port = run_with_test_server(
            self.get_default_request(url), url, self.default_handler
        )
        span = self.assert_spans(1)
        self.assertEqual(
            "http://{}:{}/test-path".format(host, port),
            span.attributes["http.url"],
        )

    def test_span_name(self):
        def span_name_callback(params: aiohttp.TraceRequestStartParams) -> str:
            return "{} - {}".format(params.method, params.url.path)

        AioHttpClientInstrumentor().uninstrument()
        AioHttpClientInstrumentor().instrument(span_name=span_name_callback)

        url = "/test-path"
        run_with_test_server(
            self.get_default_request(url), url, self.default_handler
        )
        span = self.assert_spans(1)
        self.assertEqual("GET - /test-path", span.name)


class TestLoadingAioHttpInstrumentor(unittest.TestCase):
    def test_loading_instrumentor(self):
        entry_points = iter_entry_points(
            "opentelemetry_instrumentor", "aiohttp-client"
        )

        instrumentor = next(entry_points).load()()
        self.assertIsInstance(instrumentor, AioHttpClientInstrumentor)
