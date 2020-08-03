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
import urllib.parse
from http import HTTPStatus

import aiohttp
import aiohttp.test_utils
import yarl

import opentelemetry.instrumentation.aiohttp_client
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace.status import StatusCanonicalCode


class TestAioHttpIntegration(TestBase):
    maxDiff = None

    def assert_spans(self, spans):
        self.assertEqual(
            [
                (
                    span.name,
                    (span.status.canonical_code, span.status.description),
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
                actual = opentelemetry.instrumentation.aiohttp_client.url_path_span_name(
                    params
                )
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

        async def do_request():
            async def default_handler(request):
                assert "traceparent" in request.headers
                return aiohttp.web.Response(status=int(status_code))

            handler = request_handler or default_handler

            app = aiohttp.web.Application()
            parsed_url = urllib.parse.urlparse(url)
            app.add_routes([aiohttp.web.get(parsed_url.path, handler)])
            app.add_routes([aiohttp.web.post(parsed_url.path, handler)])
            app.add_routes([aiohttp.web.patch(parsed_url.path, handler)])

            with contextlib.suppress(aiohttp.ClientError):
                async with aiohttp.test_utils.TestServer(app) as server:
                    netloc = (server.host, server.port)
                    async with aiohttp.test_utils.TestClient(
                        server, trace_configs=[trace_config]
                    ) as client:
                        await client.start_server()
                        await client.request(
                            method, url, trace_request_ctx={}, **kwargs
                        )
            return netloc

        loop = asyncio.get_event_loop()
        return loop.run_until_complete(do_request())

    def test_status_codes(self):
        for status_code, span_status in (
            (HTTPStatus.OK, StatusCanonicalCode.OK),
            (HTTPStatus.TEMPORARY_REDIRECT, StatusCanonicalCode.OK),
            (HTTPStatus.SERVICE_UNAVAILABLE, StatusCanonicalCode.UNAVAILABLE),
            (
                HTTPStatus.GATEWAY_TIMEOUT,
                StatusCanonicalCode.DEADLINE_EXCEEDED,
            ),
        ):
            with self.subTest(status_code=status_code):
                host, port = self._http_request(
                    trace_config=opentelemetry.instrumentation.aiohttp_client.create_trace_config(),
                    url="/test-path?query=param#foobar",
                    status_code=status_code,
                )

                self.assert_spans(
                    [
                        (
                            "GET",
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
                    trace_config=opentelemetry.instrumentation.aiohttp_client.create_trace_config(
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
                            (StatusCanonicalCode.OK, None),
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
            trace_config=opentelemetry.instrumentation.aiohttp_client.create_trace_config(
                url_filter=strip_query_params
            ),
            url="/some/path?query=param&other=param2",
            status_code=HTTPStatus.OK,
        )

        self.assert_spans(
            [
                (
                    "GET",
                    (StatusCanonicalCode.OK, None),
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
        trace_configs = [
            opentelemetry.instrumentation.aiohttp_client.create_trace_config()
        ]

        for url, expected_status in (
            ("http://this-is-unknown.local/", StatusCanonicalCode.UNKNOWN),
            ("http://127.0.0.1:1/", StatusCanonicalCode.UNAVAILABLE),
        ):
            with self.subTest(expected_status=expected_status):

                async def do_request(url):
                    async with aiohttp.ClientSession(
                        trace_configs=trace_configs
                    ) as session:
                        async with session.get(url):
                            pass

                loop = asyncio.get_event_loop()
                with self.assertRaises(aiohttp.ClientConnectorError):
                    loop.run_until_complete(do_request(url))

            self.assert_spans(
                [
                    (
                        "GET",
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
            trace_config=opentelemetry.instrumentation.aiohttp_client.create_trace_config(),
            url="/test_timeout",
            request_handler=request_handler,
            timeout=aiohttp.ClientTimeout(sock_read=0.01),
        )

        self.assert_spans(
            [
                (
                    "GET",
                    (StatusCanonicalCode.DEADLINE_EXCEEDED, None),
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
            trace_config=opentelemetry.instrumentation.aiohttp_client.create_trace_config(),
            url="/test_too_many_redirects",
            request_handler=request_handler,
            max_redirects=2,
        )

        self.assert_spans(
            [
                (
                    "GET",
                    (StatusCanonicalCode.DEADLINE_EXCEEDED, None),
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
