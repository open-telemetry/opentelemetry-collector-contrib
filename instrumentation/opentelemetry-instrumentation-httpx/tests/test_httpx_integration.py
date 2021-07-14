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
import asyncio
import typing
from unittest import mock

import httpx
import respx

import opentelemetry.instrumentation.httpx
from opentelemetry import context, trace
from opentelemetry.instrumentation.httpx import (
    AsyncOpenTelemetryTransport,
    HTTPXClientInstrumentor,
    SyncOpenTelemetryTransport,
)
from opentelemetry.propagate import get_global_textmap, set_global_textmap
from opentelemetry.sdk import resources
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.mock_textmap import MockTextMapPropagator
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import StatusCode

if typing.TYPE_CHECKING:
    from opentelemetry.instrumentation.httpx import (
        AsyncRequestHook,
        AsyncResponseHook,
        RequestHook,
        RequestInfo,
        ResponseHook,
        ResponseInfo,
    )
    from opentelemetry.sdk.trace.export import SpanExporter
    from opentelemetry.trace import TracerProvider
    from opentelemetry.trace.span import Span


HTTP_RESPONSE_BODY = "http.response.body"


def _async_call(coro: typing.Coroutine) -> asyncio.Task:
    loop = asyncio.get_event_loop()
    return loop.run_until_complete(coro)


def _response_hook(span, request: "RequestInfo", response: "ResponseInfo"):
    span.set_attribute(
        HTTP_RESPONSE_BODY, response[2].read(),
    )


async def _async_response_hook(
    span: "Span", request: "RequestInfo", response: "ResponseInfo"
):
    span.set_attribute(
        HTTP_RESPONSE_BODY, await response[2].aread(),
    )


def _request_hook(span: "Span", request: "RequestInfo"):
    url = httpx.URL(request[1])
    span.update_name("GET" + str(url))


async def _async_request_hook(span: "Span", request: "RequestInfo"):
    url = httpx.URL(request[1])
    span.update_name("GET" + str(url))


def _no_update_request_hook(span: "Span", request: "RequestInfo"):
    return 123


async def _async_no_update_request_hook(span: "Span", request: "RequestInfo"):
    return 123


# Using this wrapper class to have a base class for the tests while also not
# angering pylint or mypy when calling methods not in the class when only
# subclassing abc.ABC.
class BaseTestCases:
    class BaseTest(TestBase, metaclass=abc.ABCMeta):
        # pylint: disable=no-member

        URL = "http://httpbin.org/status/200"
        response_hook = staticmethod(_response_hook)
        request_hook = staticmethod(_request_hook)
        no_update_request_hook = staticmethod(_no_update_request_hook)

        # pylint: disable=invalid-name
        def setUp(self):
            super().setUp()
            respx.start()
            respx.get(self.URL).mock(httpx.Response(200, text="Hello!"))

        # pylint: disable=invalid-name
        def tearDown(self):
            super().tearDown()
            respx.stop()

        def assert_span(
            self, exporter: "SpanExporter" = None, num_spans: int = 1
        ):
            if exporter is None:
                exporter = self.memory_exporter
            span_list = exporter.get_finished_spans()
            self.assertEqual(num_spans, len(span_list))
            if num_spans == 0:
                return None
            if num_spans == 1:
                return span_list[0]
            return span_list

        @abc.abstractmethod
        def perform_request(
            self,
            url: str,
            method: str = "GET",
            headers: typing.Dict[str, str] = None,
            client: typing.Union[httpx.Client, httpx.AsyncClient, None] = None,
        ):
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
                span, opentelemetry.instrumentation.httpx
            )

        def test_basic_multiple(self):
            self.perform_request(self.URL)
            self.perform_request(self.URL)
            self.assert_span(num_spans=2)

        def test_not_foundbasic(self):
            url_404 = "http://httpbin.org/status/404"

            with respx.mock:
                respx.get(url_404).mock(httpx.Response(404))
                result = self.perform_request(url_404)

            self.assertEqual(result.status_code, 404)
            span = self.assert_span()
            self.assertEqual(
                span.attributes.get(SpanAttributes.HTTP_STATUS_CODE), 404
            )
            self.assertIs(
                span.status.status_code, trace.StatusCode.ERROR,
            )

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

        def test_distributed_context(self):
            previous_propagator = get_global_textmap()
            try:
                set_global_textmap(MockTextMapPropagator())
                result = self.perform_request(self.URL)
                self.assertEqual(result.text, "Hello!")

                span = self.assert_span()

                headers = dict(respx.calls.last.request.headers)
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

        def test_requests_500_error(self):
            respx.get(self.URL).mock(httpx.Response(500))

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

        def test_requests_basic_exception(self):
            with respx.mock, self.assertRaises(Exception):
                respx.get(self.URL).mock(side_effect=Exception)
                self.perform_request(self.URL)

            span = self.assert_span()
            self.assertEqual(span.status.status_code, StatusCode.ERROR)

        def test_requests_timeout_exception(self):
            with respx.mock, self.assertRaises(httpx.TimeoutException):
                respx.get(self.URL).mock(side_effect=httpx.TimeoutException)
                self.perform_request(self.URL)

            span = self.assert_span()
            self.assertEqual(span.status.status_code, StatusCode.ERROR)

        def test_invalid_url(self):
            url = "invalid://nope"

            with respx.mock, self.assertRaises(httpx.UnsupportedProtocol):
                respx.post("invalid://nope").pass_through()
                self.perform_request(url, method="POST")

            span = self.assert_span()

            self.assertEqual(span.name, "HTTP POST")
            print(span.attributes)
            self.assertEqual(
                span.attributes,
                {
                    SpanAttributes.HTTP_METHOD: "POST",
                    SpanAttributes.HTTP_URL: "invalid://nope/",
                },
            )
            self.assertEqual(span.status.status_code, StatusCode.ERROR)

        def test_if_headers_equals_none(self):
            result = self.perform_request(self.URL)
            self.assertEqual(result.text, "Hello!")
            self.assert_span()

    class BaseManualTest(BaseTest, metaclass=abc.ABCMeta):
        @abc.abstractmethod
        def create_transport(
            self,
            tracer_provider: typing.Optional["TracerProvider"] = None,
            request_hook: typing.Optional["RequestHook"] = None,
            response_hook: typing.Optional["ResponseHook"] = None,
        ):
            pass

        @abc.abstractmethod
        def create_client(
            self,
            transport: typing.Union[
                SyncOpenTelemetryTransport, AsyncOpenTelemetryTransport, None
            ] = None,
        ):
            pass

        def test_default_client(self):
            client = self.create_client(transport=None)
            result = self.perform_request(self.URL, client=client)
            self.assertEqual(result.text, "Hello!")
            self.assert_span(num_spans=0)

            result = self.perform_request(self.URL)
            self.assertEqual(result.text, "Hello!")
            self.assert_span()

        def test_custom_tracer_provider(self):
            resource = resources.Resource.create({})
            result = self.create_tracer_provider(resource=resource)
            tracer_provider, exporter = result

            transport = self.create_transport(tracer_provider=tracer_provider)
            client = self.create_client(transport)
            result = self.perform_request(self.URL, client=client)

            self.assertEqual(result.text, "Hello!")
            span = self.assert_span(exporter=exporter)
            self.assertIs(span.resource, resource)

        def test_response_hook(self):
            transport = self.create_transport(
                tracer_provider=self.tracer_provider,
                response_hook=self.response_hook,
            )
            client = self.create_client(transport)
            result = self.perform_request(self.URL, client=client)

            self.assertEqual(result.text, "Hello!")
            span = self.assert_span()
            self.assertEqual(
                span.attributes,
                {
                    SpanAttributes.HTTP_METHOD: "GET",
                    SpanAttributes.HTTP_URL: self.URL,
                    SpanAttributes.HTTP_STATUS_CODE: 200,
                    HTTP_RESPONSE_BODY: "Hello!",
                },
            )

        def test_request_hook(self):
            transport = self.create_transport(request_hook=self.request_hook)
            client = self.create_client(transport)
            result = self.perform_request(self.URL, client=client)

            self.assertEqual(result.text, "Hello!")
            span = self.assert_span()
            self.assertEqual(span.name, "GET" + self.URL)

        def test_request_hook_no_span_change(self):
            transport = self.create_transport(
                request_hook=self.no_update_request_hook
            )
            client = self.create_client(transport)
            result = self.perform_request(self.URL, client=client)

            self.assertEqual(result.text, "Hello!")
            span = self.assert_span()
            self.assertEqual(span.name, "HTTP GET")

        def test_not_recording(self):
            with mock.patch("opentelemetry.trace.INVALID_SPAN") as mock_span:
                transport = self.create_transport(
                    tracer_provider=trace._DefaultTracerProvider()
                )
                client = self.create_client(transport)
                mock_span.is_recording.return_value = False
                result = self.perform_request(self.URL, client=client)

                self.assertEqual(result.text, "Hello!")
                self.assert_span(None, 0)
                self.assertFalse(mock_span.is_recording())
                self.assertTrue(mock_span.is_recording.called)
                self.assertFalse(mock_span.set_attribute.called)
                self.assertFalse(mock_span.set_status.called)

    class BaseInstrumentorTest(BaseTest, metaclass=abc.ABCMeta):
        @abc.abstractmethod
        def create_client(
            self,
            transport: typing.Union[
                SyncOpenTelemetryTransport, AsyncOpenTelemetryTransport, None
            ] = None,
        ):
            pass

        def setUp(self):
            super().setUp()
            HTTPXClientInstrumentor().instrument()
            self.client = self.create_client()
            HTTPXClientInstrumentor().uninstrument()

        def test_custom_tracer_provider(self):
            resource = resources.Resource.create({})
            result = self.create_tracer_provider(resource=resource)
            tracer_provider, exporter = result

            HTTPXClientInstrumentor().instrument(
                tracer_provider=tracer_provider
            )
            client = self.create_client()
            result = self.perform_request(self.URL, client=client)

            self.assertEqual(result.text, "Hello!")
            span = self.assert_span(exporter=exporter)
            self.assertIs(span.resource, resource)
            HTTPXClientInstrumentor().uninstrument()

        def test_response_hook(self):
            HTTPXClientInstrumentor().instrument(
                tracer_provider=self.tracer_provider,
                response_hook=self.response_hook,
            )
            client = self.create_client()
            result = self.perform_request(self.URL, client=client)

            self.assertEqual(result.text, "Hello!")
            span = self.assert_span()
            self.assertEqual(
                span.attributes,
                {
                    SpanAttributes.HTTP_METHOD: "GET",
                    SpanAttributes.HTTP_URL: self.URL,
                    SpanAttributes.HTTP_STATUS_CODE: 200,
                    HTTP_RESPONSE_BODY: "Hello!",
                },
            )
            HTTPXClientInstrumentor().uninstrument()

        def test_request_hook(self):
            HTTPXClientInstrumentor().instrument(
                tracer_provider=self.tracer_provider,
                request_hook=self.request_hook,
            )
            client = self.create_client()
            result = self.perform_request(self.URL, client=client)

            self.assertEqual(result.text, "Hello!")
            span = self.assert_span()
            self.assertEqual(span.name, "GET" + self.URL)
            HTTPXClientInstrumentor().uninstrument()

        def test_request_hook_no_span_update(self):
            HTTPXClientInstrumentor().instrument(
                tracer_provider=self.tracer_provider,
                request_hook=self.no_update_request_hook,
            )
            client = self.create_client()
            result = self.perform_request(self.URL, client=client)

            self.assertEqual(result.text, "Hello!")
            span = self.assert_span()
            self.assertEqual(span.name, "HTTP GET")
            HTTPXClientInstrumentor().uninstrument()

        def test_not_recording(self):
            with mock.patch("opentelemetry.trace.INVALID_SPAN") as mock_span:
                HTTPXClientInstrumentor().instrument(
                    tracer_provider=trace._DefaultTracerProvider()
                )
                client = self.create_client()

                mock_span.is_recording.return_value = False
                result = self.perform_request(self.URL, client=client)

                self.assertEqual(result.text, "Hello!")
                self.assert_span(None, 0)
                self.assertFalse(mock_span.is_recording())
                self.assertTrue(mock_span.is_recording.called)
                self.assertFalse(mock_span.set_attribute.called)
                self.assertFalse(mock_span.set_status.called)
                HTTPXClientInstrumentor().uninstrument()

        def test_suppress_instrumentation_new_client(self):
            HTTPXClientInstrumentor().instrument()
            token = context.attach(
                context.set_value("suppress_instrumentation", True)
            )
            try:
                client = self.create_client()
                result = self.perform_request(self.URL, client=client)
                self.assertEqual(result.text, "Hello!")
            finally:
                context.detach(token)

            self.assert_span(num_spans=0)
            HTTPXClientInstrumentor().uninstrument()

        def test_instrument_client(self):
            client = self.create_client()
            HTTPXClientInstrumentor().instrument_client(client)
            result = self.perform_request(self.URL, client=client)
            self.assertEqual(result.text, "Hello!")
            self.assert_span(num_spans=1)

        def test_uninstrument(self):
            HTTPXClientInstrumentor().instrument()
            HTTPXClientInstrumentor().uninstrument()
            client = self.create_client()
            result = self.perform_request(self.URL, client=client)
            self.assertEqual(result.text, "Hello!")
            self.assert_span(num_spans=0)

        def test_uninstrument_client(self):
            HTTPXClientInstrumentor().uninstrument_client(self.client)

            result = self.perform_request(self.URL)

            self.assertEqual(result.text, "Hello!")
            self.assert_span(num_spans=0)

        def test_uninstrument_new_client(self):
            HTTPXClientInstrumentor().instrument()
            client1 = self.create_client()
            HTTPXClientInstrumentor().uninstrument_client(client1)

            result = self.perform_request(self.URL, client=client1)
            self.assertEqual(result.text, "Hello!")
            self.assert_span(num_spans=0)

            # Test that other clients as well as instance client is still
            # instrumented
            client2 = self.create_client()
            result = self.perform_request(self.URL, client=client2)
            self.assertEqual(result.text, "Hello!")
            self.assert_span()

            self.memory_exporter.clear()

            result = self.perform_request(self.URL)
            self.assertEqual(result.text, "Hello!")
            self.assert_span()


class TestSyncIntegration(BaseTestCases.BaseManualTest):
    def setUp(self):
        super().setUp()
        self.transport = self.create_transport()
        self.client = self.create_client(self.transport)

    def tearDown(self):
        super().tearDown()
        self.client.close()

    def create_transport(
        self,
        tracer_provider: typing.Optional["TracerProvider"] = None,
        request_hook: typing.Optional["RequestHook"] = None,
        response_hook: typing.Optional["ResponseHook"] = None,
    ):
        transport = httpx.HTTPTransport()
        telemetry_transport = SyncOpenTelemetryTransport(
            transport,
            tracer_provider=tracer_provider,
            request_hook=request_hook,
            response_hook=response_hook,
        )
        return telemetry_transport

    def create_client(
        self, transport: typing.Optional[SyncOpenTelemetryTransport] = None,
    ):
        return httpx.Client(transport=transport)

    def perform_request(
        self,
        url: str,
        method: str = "GET",
        headers: typing.Dict[str, str] = None,
        client: typing.Union[httpx.Client, httpx.AsyncClient, None] = None,
    ):
        if client is None:
            return self.client.request(method, url, headers=headers)
        return client.request(method, url, headers=headers)


class TestAsyncIntegration(BaseTestCases.BaseManualTest):
    response_hook = staticmethod(_async_response_hook)
    request_hook = staticmethod(_async_request_hook)
    no_update_request_hook = staticmethod(_async_no_update_request_hook)

    def setUp(self):
        super().setUp()
        self.transport = self.create_transport()
        self.client = self.create_client(self.transport)

    def create_transport(
        self,
        tracer_provider: typing.Optional["TracerProvider"] = None,
        request_hook: typing.Optional["AsyncRequestHook"] = None,
        response_hook: typing.Optional["AsyncResponseHook"] = None,
    ):
        transport = httpx.AsyncHTTPTransport()
        telemetry_transport = AsyncOpenTelemetryTransport(
            transport,
            tracer_provider=tracer_provider,
            request_hook=request_hook,
            response_hook=response_hook,
        )
        return telemetry_transport

    def create_client(
        self, transport: typing.Optional[AsyncOpenTelemetryTransport] = None,
    ):
        return httpx.AsyncClient(transport=transport)

    def perform_request(
        self,
        url: str,
        method: str = "GET",
        headers: typing.Dict[str, str] = None,
        client: typing.Union[httpx.Client, httpx.AsyncClient, None] = None,
    ):
        async def _perform_request():
            nonlocal client
            nonlocal method
            if client is None:
                client = self.client
            async with client as _client:
                return await _client.request(method, url, headers=headers)

        return _async_call(_perform_request())


class TestSyncInstrumentationIntegration(BaseTestCases.BaseInstrumentorTest):
    def create_client(
        self, transport: typing.Optional[SyncOpenTelemetryTransport] = None,
    ):
        return httpx.Client()

    def perform_request(
        self,
        url: str,
        method: str = "GET",
        headers: typing.Dict[str, str] = None,
        client: typing.Union[httpx.Client, httpx.AsyncClient, None] = None,
    ):
        if client is None:
            return self.client.request(method, url, headers=headers)
        return client.request(method, url, headers=headers)


class TestAsyncInstrumentationIntegration(BaseTestCases.BaseInstrumentorTest):
    response_hook = staticmethod(_async_response_hook)
    request_hook = staticmethod(_async_request_hook)
    no_update_request_hook = staticmethod(_async_no_update_request_hook)

    def create_client(
        self, transport: typing.Optional[AsyncOpenTelemetryTransport] = None,
    ):
        return httpx.AsyncClient()

    def perform_request(
        self,
        url: str,
        method: str = "GET",
        headers: typing.Dict[str, str] = None,
        client: typing.Union[httpx.Client, httpx.AsyncClient, None] = None,
    ):
        async def _perform_request():
            nonlocal client
            nonlocal method
            if client is None:
                client = self.client
            async with client as _client:
                return await _client.request(method, url, headers=headers)

        return _async_call(_perform_request())
