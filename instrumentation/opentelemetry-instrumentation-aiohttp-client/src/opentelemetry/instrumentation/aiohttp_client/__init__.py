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

    .. code:: python

        import aiohttp
        from opentelemetry.instrumentation.aiohttp_client import (
            create_trace_config,
            url_path_span_name
        )
        import yarl

        def strip_query_params(url: yarl.URL) -> str:
            return str(url.with_query(None))

        async with aiohttp.ClientSession(trace_configs=[create_trace_config(
                # Remove all query params from the URL attribute on the span.
                url_filter=strip_query_params,
                # Use the URL's path as the span name.
                span_name=url_path_span_name
        )]) as session:
            async with session.get(url) as response:
                await response.text()

"""

import contextlib
import socket
import types
import typing

import aiohttp

from opentelemetry import context as context_api
from opentelemetry import propagators, trace
from opentelemetry.instrumentation.aiohttp_client.version import __version__
from opentelemetry.instrumentation.utils import http_status_to_canonical_code
from opentelemetry.trace import SpanKind
from opentelemetry.trace.status import Status, StatusCanonicalCode


def url_path_span_name(params: aiohttp.TraceRequestStartParams) -> str:
    """Extract a span name from the request URL path.

    A simple callable to extract the path portion of the requested URL
    for use as the span name.

    :param aiohttp.TraceRequestStartParams params: Parameters describing
        the traced request.

    :return: The URL path.
    :rtype: str
    """
    return params.url.path


def create_trace_config(
    url_filter: typing.Optional[typing.Callable[[str], str]] = None,
    span_name: typing.Optional[
        typing.Union[
            typing.Callable[[aiohttp.TraceRequestStartParams], str], str
        ]
    ] = None,
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

    :param str span_name: Override the default span name.

    :return: An object suitable for use with :py:class:`aiohttp.ClientSession`.
    :rtype: :py:class:`aiohttp.TraceConfig`
    """
    # `aiohttp.TraceRequestStartParams` resolves to `aiohttp.tracing.TraceRequestStartParams`
    # which doesn't exist in the aiottp intersphinx inventory.
    # Explicitly specify the type for the `span_name` param and rtype to work
    # around this issue.

    tracer = trace.get_tracer_provider().get_tracer(__name__, __version__)

    def _end_trace(trace_config_ctx: types.SimpleNamespace):
        context_api.detach(trace_config_ctx.token)
        trace_config_ctx.span.end()

    async def on_request_start(
        unused_session: aiohttp.ClientSession,
        trace_config_ctx: types.SimpleNamespace,
        params: aiohttp.TraceRequestStartParams,
    ):
        http_method = params.method.upper()
        if trace_config_ctx.span_name is None:
            request_span_name = http_method
        elif callable(trace_config_ctx.span_name):
            request_span_name = str(trace_config_ctx.span_name(params))
        else:
            request_span_name = str(trace_config_ctx.span_name)

        trace_config_ctx.span = trace_config_ctx.tracer.start_span(
            request_span_name,
            kind=SpanKind.CLIENT,
            attributes={
                "component": "http",
                "http.method": http_method,
                "http.url": trace_config_ctx.url_filter(params.url)
                if callable(trace_config_ctx.url_filter)
                else str(params.url),
            },
        )

        trace_config_ctx.token = context_api.attach(
            trace.set_span_in_context(trace_config_ctx.span)
        )

        propagators.inject(type(params.headers).__setitem__, params.headers)

    async def on_request_end(
        unused_session: aiohttp.ClientSession,
        trace_config_ctx: types.SimpleNamespace,
        params: aiohttp.TraceRequestEndParams,
    ):
        trace_config_ctx.span.set_status(
            Status(http_status_to_canonical_code(int(params.response.status)))
        )
        trace_config_ctx.span.set_attribute(
            "http.status_code", params.response.status
        )
        trace_config_ctx.span.set_attribute(
            "http.status_text", params.response.reason
        )
        _end_trace(trace_config_ctx)

    async def on_request_exception(
        unused_session: aiohttp.ClientSession,
        trace_config_ctx: types.SimpleNamespace,
        params: aiohttp.TraceRequestExceptionParams,
    ):
        if isinstance(
            params.exception,
            (aiohttp.ServerTimeoutError, aiohttp.TooManyRedirects),
        ):
            status = StatusCanonicalCode.DEADLINE_EXCEEDED
        # Assume any getaddrinfo error is a DNS failure.
        elif isinstance(
            params.exception, aiohttp.ClientConnectorError
        ) and isinstance(params.exception.os_error, socket.gaierror):
            # DNS resolution failed
            status = StatusCanonicalCode.UNKNOWN
        else:
            status = StatusCanonicalCode.UNAVAILABLE

        trace_config_ctx.span.set_status(Status(status))
        _end_trace(trace_config_ctx)

    def _trace_config_ctx_factory(**kwargs):
        kwargs.setdefault("trace_request_ctx", {})
        return types.SimpleNamespace(
            span_name=span_name, tracer=tracer, url_filter=url_filter, **kwargs
        )

    trace_config = aiohttp.TraceConfig(
        trace_config_ctx_factory=_trace_config_ctx_factory
    )

    trace_config.on_request_start.append(on_request_start)
    trace_config.on_request_end.append(on_request_end)
    trace_config.on_request_exception.append(on_request_exception)

    return trace_config
