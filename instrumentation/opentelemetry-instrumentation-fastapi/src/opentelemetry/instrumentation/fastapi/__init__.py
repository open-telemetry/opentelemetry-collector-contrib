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
