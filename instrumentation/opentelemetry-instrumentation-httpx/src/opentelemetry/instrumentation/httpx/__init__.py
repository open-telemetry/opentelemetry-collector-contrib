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

import logging
import typing

import httpx
import wrapt

from opentelemetry import context
from opentelemetry.instrumentation.httpx.package import _instruments
from opentelemetry.instrumentation.httpx.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import (
    http_status_to_status_code,
    unwrap,
)
from opentelemetry.propagate import inject
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import SpanKind, Tracer, TracerProvider, get_tracer
from opentelemetry.trace.span import Span
from opentelemetry.trace.status import Status

_logger = logging.getLogger(__name__)

URL = typing.Tuple[bytes, bytes, typing.Optional[int], bytes]
Headers = typing.List[typing.Tuple[bytes, bytes]]
RequestHook = typing.Callable[[Span, "RequestInfo"], None]
ResponseHook = typing.Callable[[Span, "RequestInfo", "ResponseInfo"], None]
AsyncRequestHook = typing.Callable[
    [Span, "RequestInfo"], typing.Awaitable[typing.Any]
]
AsyncResponseHook = typing.Callable[
    [Span, "RequestInfo", "ResponseInfo"], typing.Awaitable[typing.Any]
]


class RequestInfo(typing.NamedTuple):
    method: bytes
    url: URL
    headers: typing.Optional[Headers]
    stream: typing.Optional[
        typing.Union[httpx.SyncByteStream, httpx.AsyncByteStream]
    ]
    extensions: typing.Optional[dict]


class ResponseInfo(typing.NamedTuple):
    status_code: int
    headers: typing.Optional[Headers]
    stream: typing.Iterable[bytes]
    extensions: typing.Optional[dict]


def _get_default_span_name(method: str) -> str:
    return "HTTP {}".format(method).strip()


def _apply_status_code(span: Span, status_code: int) -> None:
    if not span.is_recording():
        return

    span.set_attribute(SpanAttributes.HTTP_STATUS_CODE, status_code)
    span.set_status(Status(http_status_to_status_code(status_code)))


def _prepare_attributes(method: bytes, url: URL) -> typing.Dict[str, str]:
    _method = method.decode().upper()
    _url = str(httpx.URL(url))
    span_attributes = {
        SpanAttributes.HTTP_METHOD: _method,
        SpanAttributes.HTTP_URL: _url,
    }
    return span_attributes


def _prepare_headers(headers: typing.Optional[Headers]) -> httpx.Headers:
    return httpx.Headers(headers)


class SyncOpenTelemetryTransport(httpx.BaseTransport):
    """Sync transport class that will trace all requests made with a client.

    Args:
        transport: SyncHTTPTransport instance to wrap
        tracer_provider: Tracer provider to use
        request_hook: A hook that receives the span and request that is called
            right after the span is created
        response_hook: A hook that receives the span, request, and response
            that is called right before the span ends
    """

    def __init__(
        self,
        transport: httpx.BaseTransport,
        tracer_provider: typing.Optional[TracerProvider] = None,
        request_hook: typing.Optional[RequestHook] = None,
        response_hook: typing.Optional[ResponseHook] = None,
    ):
        self._transport = transport
        self._tracer = get_tracer(
            __name__,
            instrumenting_library_version=__version__,
            tracer_provider=tracer_provider,
        )
        self._request_hook = request_hook
        self._response_hook = response_hook

    def handle_request(
        self,
        method: bytes,
        url: URL,
        headers: typing.Optional[Headers] = None,
        stream: typing.Optional[httpx.SyncByteStream] = None,
        extensions: typing.Optional[dict] = None,
    ) -> typing.Tuple[int, "Headers", httpx.SyncByteStream, dict]:
        """Add request info to span."""
        if context.get_value("suppress_instrumentation"):
            return self._transport.handle_request(
                method,
                url,
                headers=headers,
                stream=stream,
                extensions=extensions,
            )

        span_attributes = _prepare_attributes(method, url)
        _headers = _prepare_headers(headers)
        span_name = _get_default_span_name(
            span_attributes[SpanAttributes.HTTP_METHOD]
        )
        request = RequestInfo(method, url, headers, stream, extensions)

        with self._tracer.start_as_current_span(
            span_name, kind=SpanKind.CLIENT, attributes=span_attributes
        ) as span:
            if self._request_hook is not None:
                self._request_hook(span, request)

            inject(_headers)

            (
                status_code,
                headers,
                stream,
                extensions,
            ) = self._transport.handle_request(
                method,
                url,
                headers=_headers.raw,
                stream=stream,
                extensions=extensions,
            )

            _apply_status_code(span, status_code)

            if self._response_hook is not None:
                self._response_hook(
                    span,
                    request,
                    ResponseInfo(status_code, headers, stream, extensions),
                )

        return status_code, headers, stream, extensions


class AsyncOpenTelemetryTransport(httpx.AsyncBaseTransport):
    """Async transport class that will trace all requests made with a client.

    Args:
        transport: AsyncHTTPTransport instance to wrap
        tracer_provider: Tracer provider to use
        request_hook: A hook that receives the span and request that is called
            right after the span is created
        response_hook: A hook that receives the span, request, and response
            that is called right before the span ends
    """

    def __init__(
        self,
        transport: httpx.AsyncBaseTransport,
        tracer_provider: typing.Optional[TracerProvider] = None,
        request_hook: typing.Optional[RequestHook] = None,
        response_hook: typing.Optional[ResponseHook] = None,
    ):
        self._transport = transport
        self._tracer = get_tracer(
            __name__,
            instrumenting_library_version=__version__,
            tracer_provider=tracer_provider,
        )
        self._request_hook = request_hook
        self._response_hook = response_hook

    async def handle_async_request(
        self,
        method: bytes,
        url: URL,
        headers: typing.Optional[Headers] = None,
        stream: typing.Optional[httpx.AsyncByteStream] = None,
        extensions: typing.Optional[dict] = None,
    ) -> typing.Tuple[int, "Headers", httpx.AsyncByteStream, dict]:
        """Add request info to span."""
        if context.get_value("suppress_instrumentation"):
            return await self._transport.handle_async_request(
                method,
                url,
                headers=headers,
                stream=stream,
                extensions=extensions,
            )

        span_attributes = _prepare_attributes(method, url)
        _headers = _prepare_headers(headers)
        span_name = _get_default_span_name(
            span_attributes[SpanAttributes.HTTP_METHOD]
        )
        request = RequestInfo(method, url, headers, stream, extensions)

        with self._tracer.start_as_current_span(
            span_name, kind=SpanKind.CLIENT, attributes=span_attributes
        ) as span:
            if self._request_hook is not None:
                await self._request_hook(span, request)

            inject(_headers)

            (
                status_code,
                headers,
                stream,
                extensions,
            ) = await self._transport.handle_async_request(
                method,
                url,
                headers=_headers.raw,
                stream=stream,
                extensions=extensions,
            )

            _apply_status_code(span, status_code)

            if self._response_hook is not None:
                await self._response_hook(
                    span,
                    request,
                    ResponseInfo(status_code, headers, stream, extensions),
                )

        return status_code, headers, stream, extensions


class _InstrumentedClient(httpx.Client):

    _tracer_provider = None
    _request_hook = None
    _response_hook = None

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        self._original_transport = self._transport
        self._is_instrumented_by_opentelemetry = True

        self._transport = SyncOpenTelemetryTransport(
            self._transport,
            tracer_provider=_InstrumentedClient._tracer_provider,
            request_hook=_InstrumentedClient._request_hook,
            response_hook=_InstrumentedClient._response_hook,
        )


class _InstrumentedAsyncClient(httpx.AsyncClient):

    _tracer_provider = None
    _request_hook = None
    _response_hook = None

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        self._original_transport = self._transport
        self._is_instrumented_by_opentelemetry = True

        self._transport = AsyncOpenTelemetryTransport(
            self._transport,
            tracer_provider=_InstrumentedAsyncClient._tracer_provider,
            request_hook=_InstrumentedAsyncClient._request_hook,
            response_hook=_InstrumentedAsyncClient._response_hook,
        )


class HTTPXClientInstrumentor(BaseInstrumentor):
    # pylint: disable=protected-access,attribute-defined-outside-init
    """An instrumentor for httpx Client and AsyncClient

    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> typing.Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Instruments httpx Client and AsyncClient

        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global
                ``request_hook``: A hook that receives the span and request that is called
                    right after the span is created
                ``response_hook``: A hook that receives the span, request, and response
                    that is called right before the span ends
        """
        self._original_client = httpx.Client
        self._original_async_client = httpx.AsyncClient
        request_hook = kwargs.get("request_hook")
        response_hook = kwargs.get("response_hook")
        if callable(request_hook):
            _InstrumentedClient._request_hook = request_hook
            _InstrumentedAsyncClient._request_hook = request_hook
        if callable(response_hook):
            _InstrumentedClient._response_hook = response_hook
            _InstrumentedAsyncClient._response_hook = response_hook
        tracer_provider = kwargs.get("tracer_provider")
        _InstrumentedClient._tracer_provider = tracer_provider
        _InstrumentedAsyncClient._tracer_provider = tracer_provider
        httpx.Client = _InstrumentedClient
        httpx.AsyncClient = _InstrumentedAsyncClient

    def _uninstrument(self, **kwargs):
        httpx.Client = self._original_client
        httpx.AsyncClient = self._original_async_client
        _InstrumentedClient._tracer_provider = None
        _InstrumentedClient._request_hook = None
        _InstrumentedClient._response_hook = None
        _InstrumentedAsyncClient._tracer_provider = None
        _InstrumentedAsyncClient._request_hook = None
        _InstrumentedAsyncClient._response_hook = None

    @staticmethod
    def instrument_client(
        client: typing.Union[httpx.Client, httpx.AsyncClient],
        tracer_provider: TracerProvider = None,
        request_hook: typing.Optional[RequestHook] = None,
        response_hook: typing.Optional[ResponseHook] = None,
    ) -> None:
        """Instrument httpx Client or AsyncClient

        Args:
            client: The httpx Client or AsyncClient instance
            tracer_provider: A TracerProvider, defaults to global
            request_hook: A hook that receives the span and request that is called
                right after the span is created
            response_hook: A hook that receives the span, request, and response
                that is called right before the span ends
        """
        # pylint: disable=protected-access
        if not hasattr(client, "_is_instrumented_by_opentelemetry"):
            client._is_instrumented_by_opentelemetry = False

        if not client._is_instrumented_by_opentelemetry:
            if isinstance(client, httpx.Client):
                client._original_transport = client._transport
                transport = client._transport or httpx.HTTPTransport()
                client._transport = SyncOpenTelemetryTransport(
                    transport,
                    tracer_provider=tracer_provider,
                    request_hook=request_hook,
                    response_hook=response_hook,
                )
                client._is_instrumented_by_opentelemetry = True
            if isinstance(client, httpx.AsyncClient):
                transport = client._transport or httpx.AsyncHTTPTransport()
                client._transport = AsyncOpenTelemetryTransport(
                    transport,
                    tracer_provider=tracer_provider,
                    request_hook=request_hook,
                    response_hook=response_hook,
                )
                client._is_instrumented_by_opentelemetry = True
        else:
            _logger.warning(
                "Attempting to instrument Httpx client while already instrumented"
            )

    @staticmethod
    def uninstrument_client(
        client: typing.Union[httpx.Client, httpx.AsyncClient]
    ):
        """Disables instrumentation for the given client instance

        Args:
            client: The httpx Client or AsyncClient instance
        """
        if hasattr(client, "_original_transport"):
            client._transport = client._original_transport
            del client._original_transport
            client._is_instrumented_by_opentelemetry = False
        else:
            _logger.warning(
                "Attempting to uninstrument Httpx "
                "client while already uninstrumented"
            )
