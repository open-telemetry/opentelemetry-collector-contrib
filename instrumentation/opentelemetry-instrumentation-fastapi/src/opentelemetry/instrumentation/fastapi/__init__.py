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

"""
Usage
-----

.. code-block:: python

    import fastapi
    from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor

    app = fastapi.FastAPI()

    @app.get("/foobar")
    async def foobar():
        return {"message": "hello world"}

    FastAPIInstrumentor.instrument_app(app)

Configuration
-------------

Exclude lists
*************
To exclude certain URLs from being tracked, set the environment variable ``OTEL_PYTHON_FASTAPI_EXCLUDED_URLS``
(or ``OTEL_PYTHON_EXCLUDED_URLS`` as fallback) with comma delimited regexes representing which URLs to exclude.

For example,

::

    export OTEL_PYTHON_FASTAPI_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.

You can also pass the comma delimited regexes to the ``instrument_app`` method directly:

.. code-block:: python

    FastAPIInstrumentor.instrument_app(app, excluded_urls="client/.*/info,healthcheck")

Request/Response hooks
**********************

Utilize request/reponse hooks to execute custom logic to be performed before/after performing a request. The server request hook takes in a server span and ASGI
scope object for every incoming request. The client request hook is called with the internal span and an ASGI scope which is sent as a dictionary for when the method recieve is called.
The client response hook is called with the internal span and an ASGI event which is sent as a dictionary for when the method send is called.

.. code-block:: python

    def server_request_hook(span: Span, scope: dict):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_request_hook", "some-value")

    def client_request_hook(span: Span, scope: dict):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_client_request_hook", "some-value")

    def client_response_hook(span: Span, message: dict):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_response_hook", "some-value")

   FastAPIInstrumentor().instrument(server_request_hook=server_request_hook, client_request_hook=client_request_hook, client_response_hook=client_response_hook)

Capture HTTP request and response headers
*****************************************
You can configure the agent to capture predefined HTTP headers as span attributes, according to the `semantic convention <https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#http-request-and-response-headers>`_.

Request headers
***************
To capture predefined HTTP request headers as span attributes, set the environment variable ``OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST``
to a comma-separated list of HTTP header names.

For example,

::

    export OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST="content-type,custom_request_header"

will extract ``content-type`` and ``custom_request_header`` from request headers and add them as span attributes.

It is recommended that you should give the correct names of the headers to be captured in the environment variable.
Request header names in fastapi are case insensitive. So, giving header name as ``CUStom-Header`` in environment variable will be able capture header with name ``custom-header``.

The name of the added span attribute will follow the format ``http.request.header.<header_name>`` where ``<header_name>`` being the normalized HTTP header name (lowercase, with - characters replaced by _ ).
The value of the attribute will be single item list containing all the header values.

Example of the added span attribute,
``http.request.header.custom_request_header = ["<value1>,<value2>"]``

Response headers
****************
To capture predefined HTTP response headers as span attributes, set the environment variable ``OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE``
to a comma-separated list of HTTP header names.

For example,

::

    export OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE="content-type,custom_response_header"

will extract ``content-type`` and ``custom_response_header`` from response headers and add them as span attributes.

It is recommended that you should give the correct names of the headers to be captured in the environment variable.
Response header names captured in fastapi are case insensitive. So, giving header name as ``CUStomHeader`` in environment variable will be able capture header with name ``customheader``.

The name of the added span attribute will follow the format ``http.response.header.<header_name>`` where ``<header_name>`` being the normalized HTTP header name (lowercase, with - characters replaced by _ ).
The value of the attribute will be single item list containing all the header values.

Example of the added span attribute,
``http.response.header.custom_response_header = ["<value1>,<value2>"]``

Note:
    Environment variable names to caputre http headers are still experimental, and thus are subject to change.

API
---
"""
import logging
import typing
from typing import Collection

import fastapi
from starlette.routing import Match

from opentelemetry.instrumentation.asgi import OpenTelemetryMiddleware
from opentelemetry.instrumentation.asgi.package import _instruments
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import Span
from opentelemetry.util.http import get_excluded_urls, parse_excluded_urls

_excluded_urls_from_env = get_excluded_urls("FASTAPI")
_logger = logging.getLogger(__name__)

_ServerRequestHookT = typing.Optional[typing.Callable[[Span, dict], None]]
_ClientRequestHookT = typing.Optional[typing.Callable[[Span, dict], None]]
_ClientResponseHookT = typing.Optional[typing.Callable[[Span, dict], None]]


class FastAPIInstrumentor(BaseInstrumentor):
    """An instrumentor for FastAPI

    See `BaseInstrumentor`
    """

    _original_fastapi = None

    @staticmethod
    def instrument_app(
        app: fastapi.FastAPI,
        server_request_hook: _ServerRequestHookT = None,
        client_request_hook: _ClientRequestHookT = None,
        client_response_hook: _ClientResponseHookT = None,
        tracer_provider=None,
        excluded_urls=None,
    ):
        """Instrument an uninstrumented FastAPI application."""
        if not hasattr(app, "_is_instrumented_by_opentelemetry"):
            app._is_instrumented_by_opentelemetry = False

        if not getattr(app, "_is_instrumented_by_opentelemetry", False):
            if excluded_urls is None:
                excluded_urls = _excluded_urls_from_env
            else:
                excluded_urls = parse_excluded_urls(excluded_urls)

            app.add_middleware(
                OpenTelemetryMiddleware,
                excluded_urls=excluded_urls,
                default_span_details=_get_route_details,
                server_request_hook=server_request_hook,
                client_request_hook=client_request_hook,
                client_response_hook=client_response_hook,
                tracer_provider=tracer_provider,
            )
            app._is_instrumented_by_opentelemetry = True
        else:
            _logger.warning(
                "Attempting to instrument FastAPI app while already instrumented"
            )

    @staticmethod
    def uninstrument_app(app: fastapi.FastAPI):
        app.user_middleware = [
            x
            for x in app.user_middleware
            if x.cls is not OpenTelemetryMiddleware
        ]
        app.middleware_stack = app.build_middleware_stack()
        app._is_instrumented_by_opentelemetry = False

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        self._original_fastapi = fastapi.FastAPI
        _InstrumentedFastAPI._tracer_provider = kwargs.get("tracer_provider")
        _InstrumentedFastAPI._server_request_hook = kwargs.get(
            "server_request_hook"
        )
        _InstrumentedFastAPI._client_request_hook = kwargs.get(
            "client_request_hook"
        )
        _InstrumentedFastAPI._client_response_hook = kwargs.get(
            "client_response_hook"
        )
        _excluded_urls = kwargs.get("excluded_urls")
        _InstrumentedFastAPI._excluded_urls = (
            _excluded_urls_from_env
            if _excluded_urls is None
            else parse_excluded_urls(_excluded_urls)
        )
        fastapi.FastAPI = _InstrumentedFastAPI

    def _uninstrument(self, **kwargs):
        fastapi.FastAPI = self._original_fastapi


class _InstrumentedFastAPI(fastapi.FastAPI):
    _tracer_provider = None
    _excluded_urls = None
    _server_request_hook: _ServerRequestHookT = None
    _client_request_hook: _ClientRequestHookT = None
    _client_response_hook: _ClientResponseHookT = None

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.add_middleware(
            OpenTelemetryMiddleware,
            excluded_urls=_InstrumentedFastAPI._excluded_urls,
            default_span_details=_get_route_details,
            server_request_hook=_InstrumentedFastAPI._server_request_hook,
            client_request_hook=_InstrumentedFastAPI._client_request_hook,
            client_response_hook=_InstrumentedFastAPI._client_response_hook,
            tracer_provider=_InstrumentedFastAPI._tracer_provider,
        )


def _get_route_details(scope):
    """Callback to retrieve the fastapi route being served.

    TODO: there is currently no way to retrieve http.route from
    a starlette application from scope.

    See: https://github.com/encode/starlette/pull/804
    """
    app = scope["app"]
    route = None
    for starlette_route in app.routes:
        match, _ = starlette_route.matches(scope)
        if match == Match.FULL:
            route = starlette_route.path
            break
        if match == Match.PARTIAL:
            route = starlette_route.path
    # method only exists for http, if websocket
    # leave it blank.
    span_name = route or scope.get("method", "")
    attributes = {}
    if route:
        attributes[SpanAttributes.HTTP_ROUTE] = route
    return span_name, attributes
