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
from opentelemetry.instrumentation.utils import _SUPPRESS_INSTRUMENTATION_KEY
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import Span, StatusCode


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

    @staticmethod
    def _http_request(
        trace_config,
        url: str,
        method: str = "GET",
        status_code: int = HTTPStatus.OK,
        request_handler: typing.Callable = None,
        **kwargs,
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
            (
                HTTPStatus.GATEWAY_TIMEOUT,
                StatusCode.ERROR,
            ),
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
                                SpanAttributes.HTTP_METHOD: "GET",
                                SpanAttributes.HTTP_URL: f"http://{host}:{port}/test-path?query=param#foobar",
                                SpanAttributes.HTTP_STATUS_CODE: int(
                                    status_code
                                ),
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

    def test_hooks(self):
        method = "PATCH"
        path = "/some/path"
        expected = "PATCH - /some/path"

        def request_hook(span: Span, params: aiohttp.TraceRequestStartParams):
            span.update_name(f"{params.method} - {params.url.path}")

        def response_hook(
            span: Span,
            params: typing.Union[
                aiohttp.TraceRequestEndParams,
                aiohttp.TraceRequestExceptionParams,
            ],
        ):
            span.set_attribute("response_hook_attr", "value")

        host, port = self._http_request(
            trace_config=aiohttp_client.create_trace_config(
                request_hook=request_hook,
                response_hook=response_hook,
            ),
            method=method,
            url=path,
            status_code=HTTPStatus.OK,
        )

        for span in self.memory_exporter.get_finished_spans():
            self.assertEqual(span.name, expected)
            self.assertEqual(
                (span.status.status_code, span.status.description),
                (StatusCode.UNSET, None),
            )
            self.assertEqual(
                span.attributes[SpanAttributes.HTTP_METHOD], method
            )
            self.assertEqual(
                span.attributes[SpanAttributes.HTTP_URL],
                f"http://{host}:{port}{path}",
            )
            self.assertEqual(
                span.attributes[SpanAttributes.HTTP_STATUS_CODE], HTTPStatus.OK
            )
            self.assertIn("response_hook_attr", span.attributes)
            self.assertEqual(span.attributes["response_hook_attr"], "value")
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
                        SpanAttributes.HTTP_METHOD: "GET",
                        SpanAttributes.HTTP_URL: f"http://{host}:{port}/some/path",
                        SpanAttributes.HTTP_STATUS_CODE: int(HTTPStatus.OK),
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
                            SpanAttributes.HTTP_METHOD: "GET",
                            SpanAttributes.HTTP_URL: url,
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
                        SpanAttributes.HTTP_METHOD: "GET",
                        SpanAttributes.HTTP_URL: f"http://{host}:{port}/test_timeout",
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
                        SpanAttributes.HTTP_METHOD: "GET",
                        SpanAttributes.HTTP_URL: f"http://{host}:{port}/test_too_many_redirects",
                    },
                )
            ]
        )

    def test_credential_removal(self):
        trace_configs = [aiohttp_client.create_trace_config()]

        url = "http://username:password@httpbin.org/status/200"
        with self.subTest(url=url):

            async def do_request(url):
                async with aiohttp.ClientSession(
                    trace_configs=trace_configs,
                ) as session:
                    async with session.get(url):
                        pass

            loop = asyncio.get_event_loop()
            loop.run_until_complete(do_request(url))

        self.assert_spans(
            [
                (
                    "HTTP GET",
                    (StatusCode.UNSET, None),
                    {
                        SpanAttributes.HTTP_METHOD: "GET",
                        SpanAttributes.HTTP_URL: "http://httpbin.org/status/200",
                        SpanAttributes.HTTP_STATUS_CODE: int(HTTPStatus.OK),
                    },
                )
            ]
        )
        self.memory_exporter.clear()


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
        self.assertEqual("GET", span.attributes[SpanAttributes.HTTP_METHOD])
        self.assertEqual(
            f"http://{host}:{port}/test-path",
            span.attributes[SpanAttributes.HTTP_URL],
        )
        self.assertEqual(200, span.attributes[SpanAttributes.HTTP_STATUS_CODE])

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
            context.set_value(_SUPPRESS_INSTRUMENTATION_KEY, True)
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
                context.set_value(_SUPPRESS_INSTRUMENTATION_KEY, True)
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
            f"http://{host}:{port}/test-path",
            span.attributes[SpanAttributes.HTTP_URL],
        )

    def test_hooks(self):
        def request_hook(span: Span, params: aiohttp.TraceRequestStartParams):
            span.update_name(f"{params.method} - {params.url.path}")

        def response_hook(
            span: Span,
            params: typing.Union[
                aiohttp.TraceRequestEndParams,
                aiohttp.TraceRequestExceptionParams,
            ],
        ):
            span.set_attribute("response_hook_attr", "value")

        AioHttpClientInstrumentor().uninstrument()
        AioHttpClientInstrumentor().instrument(
            request_hook=request_hook, response_hook=response_hook
        )

        url = "/test-path"
        run_with_test_server(
            self.get_default_request(url), url, self.default_handler
        )
        span = self.assert_spans(1)
        self.assertEqual("GET - /test-path", span.name)
        self.assertIn("response_hook_attr", span.attributes)
        self.assertEqual(span.attributes["response_hook_attr"], "value")


class TestLoadingAioHttpInstrumentor(unittest.TestCase):
    def test_loading_instrumentor(self):
        entry_points = iter_entry_points(
            "opentelemetry_instrumentor", "aiohttp-client"
        )

        instrumentor = next(entry_points).load()()
        self.assertIsInstance(instrumentor, AioHttpClientInstrumentor)
