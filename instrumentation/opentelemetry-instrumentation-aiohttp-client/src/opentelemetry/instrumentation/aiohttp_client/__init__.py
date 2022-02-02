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

"""
The opentelemetry-instrumentation-aiohttp-client package allows tracing HTTP
requests made by the aiohttp client library.

Usage
-----
Explicitly instrumenting a single client session:

.. code:: python

    import aiohttp
    from opentelemetry.instrumentation.aiohttp_client import create_trace_config
    import yarl

    def strip_query_params(url: yarl.URL) -> str:
        return str(url.with_query(None))

    async with aiohttp.ClientSession(trace_configs=[create_trace_config(
            # Remove all query params from the URL attribute on the span.
            url_filter=strip_query_params,
    )]) as session:
        async with session.get(url) as response:
            await response.text()

Instrumenting all client sessions:

.. code:: python

    import aiohttp
    from opentelemetry.instrumentation.aiohttp_client import (
        AioHttpClientInstrumentor
    )

    # Enable instrumentation
    AioHttpClientInstrumentor().instrument()

    # Create a session and make an HTTP get request
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            await response.text()

Configuration
-------------

Request/Response hooks
**********************

Utilize request/reponse hooks to execute custom logic to be performed before/after performing a request.

.. code-block:: python

   def request_hook(span: Span, params: aiohttp.TraceRequestStartParams):
      if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_request_hook", "some-value")

   def response_hook(span: Span, params: typing.Union[
                aiohttp.TraceRequestEndParams,
                aiohttp.TraceRequestExceptionParams,
            ]):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_response_hook", "some-value")

   AioHttpClientInstrumentor().instrument(request_hook=request_hook, response_hook=response_hook)

API
---
"""

import types
import typing
from typing import Collection

import aiohttp
import wrapt
import yarl

from opentelemetry import context as context_api
from opentelemetry import trace
from opentelemetry.instrumentation.aiohttp_client.package import _instruments
from opentelemetry.instrumentation.aiohttp_client.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import (
    _SUPPRESS_INSTRUMENTATION_KEY,
    http_status_to_status_code,
    unwrap,
)
from opentelemetry.propagate import inject
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import Span, SpanKind, TracerProvider, get_tracer
from opentelemetry.trace.status import Status, StatusCode
from opentelemetry.util.http import remove_url_credentials

_UrlFilterT = typing.Optional[typing.Callable[[yarl.URL], str]]
_RequestHookT = typing.Optional[
    typing.Callable[[Span, aiohttp.TraceRequestStartParams], None]
]
_ResponseHookT = typing.Optional[
    typing.Callable[
        [
            Span,
            typing.Union[
                aiohttp.TraceRequestEndParams,
                aiohttp.TraceRequestExceptionParams,
            ],
        ],
        None,
    ]
]


def create_trace_config(
    url_filter: _UrlFilterT = None,
    request_hook: _RequestHookT = None,
    response_hook: _ResponseHookT = None,
    tracer_provider: TracerProvider = None,
) -> aiohttp.TraceConfig:
    """Create an aiohttp-compatible trace configuration.

    One span is created for the entire HTTP request, including initial
    TCP/TLS setup if the connection doesn't exist.

    By default the span name is set to the HTTP request method.

    Example usage:

    .. code:: python

        import aiohttp
        from opentelemetry.instrumentation.aiohttp_client import create_trace_config

        async with aiohttp.ClientSession(trace_configs=[create_trace_config()]) as session:
            async with session.get(url) as response:
                await response.text()


    :param url_filter: A callback to process the requested URL prior to adding
        it as a span attribute. This can be useful to remove sensitive data
        such as API keys or user personal information.

    :param Callable request_hook: Optional callback that can modify span name and request params.
    :param Callable response_hook: Optional callback that can modify span name and response params.
    :param tracer_provider: optional TracerProvider from which to get a Tracer

    :return: An object suitable for use with :py:class:`aiohttp.ClientSession`.
    :rtype: :py:class:`aiohttp.TraceConfig`
    """
    # `aiohttp.TraceRequestStartParams` resolves to `aiohttp.tracing.TraceRequestStartParams`
    # which doesn't exist in the aiohttp intersphinx inventory.
    # Explicitly specify the type for the `request_hook` and `response_hook` param and rtype to work
    # around this issue.

    tracer = get_tracer(__name__, __version__, tracer_provider)

    def _end_trace(trace_config_ctx: types.SimpleNamespace):
        context_api.detach(trace_config_ctx.token)
        trace_config_ctx.span.end()

    async def on_request_start(
        unused_session: aiohttp.ClientSession,
        trace_config_ctx: types.SimpleNamespace,
        params: aiohttp.TraceRequestStartParams,
    ):
        if context_api.get_value(_SUPPRESS_INSTRUMENTATION_KEY):
            trace_config_ctx.span = None
            return

        http_method = params.method.upper()
        request_span_name = f"HTTP {http_method}"

        trace_config_ctx.span = trace_config_ctx.tracer.start_span(
            request_span_name,
            kind=SpanKind.CLIENT,
        )

        if callable(request_hook):
            request_hook(trace_config_ctx.span, params)

        if trace_config_ctx.span.is_recording():
            attributes = {
                SpanAttributes.HTTP_METHOD: http_method,
                SpanAttributes.HTTP_URL: remove_url_credentials(
                    trace_config_ctx.url_filter(params.url)
                )
                if callable(trace_config_ctx.url_filter)
                else remove_url_credentials(str(params.url)),
            }
            for key, value in attributes.items():
                trace_config_ctx.span.set_attribute(key, value)

        trace_config_ctx.token = context_api.attach(
            trace.set_span_in_context(trace_config_ctx.span)
        )

        inject(params.headers)

    async def on_request_end(
        unused_session: aiohttp.ClientSession,
        trace_config_ctx: types.SimpleNamespace,
        params: aiohttp.TraceRequestEndParams,
    ):
        if trace_config_ctx.span is None:
            return

        if callable(response_hook):
            response_hook(trace_config_ctx.span, params)

        if trace_config_ctx.span.is_recording():
            trace_config_ctx.span.set_status(
                Status(http_status_to_status_code(int(params.response.status)))
            )
            trace_config_ctx.span.set_attribute(
                SpanAttributes.HTTP_STATUS_CODE, params.response.status
            )
        _end_trace(trace_config_ctx)

    async def on_request_exception(
        unused_session: aiohttp.ClientSession,
        trace_config_ctx: types.SimpleNamespace,
        params: aiohttp.TraceRequestExceptionParams,
    ):
        if trace_config_ctx.span is None:
            return

        if callable(response_hook):
            response_hook(trace_config_ctx.span, params)

        if trace_config_ctx.span.is_recording() and params.exception:
            trace_config_ctx.span.set_status(Status(StatusCode.ERROR))
            trace_config_ctx.span.record_exception(params.exception)
        _end_trace(trace_config_ctx)

    def _trace_config_ctx_factory(**kwargs):
        kwargs.setdefault("trace_request_ctx", {})
        return types.SimpleNamespace(
            tracer=tracer, url_filter=url_filter, **kwargs
        )

    trace_config = aiohttp.TraceConfig(
        trace_config_ctx_factory=_trace_config_ctx_factory
    )

    trace_config.on_request_start.append(on_request_start)
    trace_config.on_request_end.append(on_request_end)
    trace_config.on_request_exception.append(on_request_exception)

    return trace_config


def _instrument(
    tracer_provider: TracerProvider = None,
    url_filter: _UrlFilterT = None,
    request_hook: _RequestHookT = None,
    response_hook: _ResponseHookT = None,
):
    """Enables tracing of all ClientSessions

    When a ClientSession gets created a TraceConfig is automatically added to
    the session's trace_configs.
    """
    # pylint:disable=unused-argument
    def instrumented_init(wrapped, instance, args, kwargs):
        if context_api.get_value(_SUPPRESS_INSTRUMENTATION_KEY):
            return wrapped(*args, **kwargs)

        trace_configs = list(kwargs.get("trace_configs") or ())

        trace_config = create_trace_config(
            url_filter=url_filter,
            request_hook=request_hook,
            response_hook=response_hook,
            tracer_provider=tracer_provider,
        )
        trace_config._is_instrumented_by_opentelemetry = True
        trace_configs.append(trace_config)

        kwargs["trace_configs"] = trace_configs
        return wrapped(*args, **kwargs)

    wrapt.wrap_function_wrapper(
        aiohttp.ClientSession, "__init__", instrumented_init
    )


def _uninstrument():
    """Disables instrumenting for all newly created ClientSessions"""
    unwrap(aiohttp.ClientSession, "__init__")


def _uninstrument_session(client_session: aiohttp.ClientSession):
    """Disables instrumentation for the given ClientSession"""
    # pylint: disable=protected-access
    trace_configs = client_session._trace_configs
    client_session._trace_configs = [
        trace_config
        for trace_config in trace_configs
        if not hasattr(trace_config, "_is_instrumented_by_opentelemetry")
    ]


class AioHttpClientInstrumentor(BaseInstrumentor):
    """An instrumentor for aiohttp client sessions

    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Instruments aiohttp ClientSession

        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global
                ``url_filter``: A callback to process the requested URL prior to adding
                    it as a span attribute. This can be useful to remove sensitive data
                    such as API keys or user personal information.
                ``request_hook``: An optional callback that is invoked right after a span is created.
                ``response_hook``: An optional callback which is invoked right before the span is finished processing a response.
        """
        _instrument(
            tracer_provider=kwargs.get("tracer_provider"),
            url_filter=kwargs.get("url_filter"),
            request_hook=kwargs.get("request_hook"),
            response_hook=kwargs.get("response_hook"),
        )

    def _uninstrument(self, **kwargs):
        _uninstrument()

    @staticmethod
    def uninstrument_session(client_session: aiohttp.ClientSession):
        """Disables instrumentation for the given session"""
        _uninstrument_session(client_session)
