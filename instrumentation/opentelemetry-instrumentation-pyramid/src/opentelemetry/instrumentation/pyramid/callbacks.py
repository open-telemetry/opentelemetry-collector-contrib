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

from logging import getLogger

from pyramid.events import BeforeTraversal
from pyramid.httpexceptions import HTTPException
from pyramid.settings import asbool
from pyramid.tweens import EXCVIEW

import opentelemetry.instrumentation.wsgi as otel_wsgi
from opentelemetry import context, trace
from opentelemetry.instrumentation.propagators import (
    get_global_response_propagator,
)
from opentelemetry.instrumentation.pyramid.version import __version__
from opentelemetry.instrumentation.utils import _start_internal_or_server_span
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.util._time import _time_ns
from opentelemetry.util.http import get_excluded_urls

TWEEN_NAME = "opentelemetry.instrumentation.pyramid.trace_tween_factory"
SETTING_TRACE_ENABLED = "opentelemetry-pyramid.trace_enabled"

_ENVIRON_STARTTIME_KEY = "opentelemetry-pyramid.starttime_key"
_ENVIRON_SPAN_KEY = "opentelemetry-pyramid.span_key"
_ENVIRON_ACTIVATION_KEY = "opentelemetry-pyramid.activation_key"
_ENVIRON_ENABLED_KEY = "opentelemetry-pyramid.tracing_enabled_key"
_ENVIRON_TOKEN = "opentelemetry-pyramid.token"

_logger = getLogger(__name__)


_excluded_urls = get_excluded_urls("PYRAMID")


def includeme(config):
    config.add_settings({SETTING_TRACE_ENABLED: True})

    config.add_subscriber(_before_traversal, BeforeTraversal)
    _insert_tween(config)


def _insert_tween(config):
    settings = config.get_settings()
    tweens = settings.get("pyramid.tweens")
    # If the list is empty, pyramid does not consider the tweens have been
    # set explicitly. And if our tween is already there, nothing to do
    if not tweens or not tweens.strip():
        # Add our tween just before the default exception handler
        config.add_tween(TWEEN_NAME, over=EXCVIEW)


def _before_traversal(event):
    request = event.request
    request_environ = request.environ
    span_name = otel_wsgi.get_default_span_name(request_environ)

    enabled = request_environ.get(_ENVIRON_ENABLED_KEY)
    if enabled is None:
        _logger.warning(
            "Opentelemetry pyramid tween 'opentelemetry.instrumentation.pyramid.trace_tween_factory'"
            "was not called. Make sure that the tween is included in 'pyramid.tweens' if"
            "the tween list was created manually"
        )
        return

    if not enabled:
        # Tracing not enabled, return
        return

    start_time = request_environ.get(_ENVIRON_STARTTIME_KEY)
    tracer = trace.get_tracer(__name__, __version__)

    if request.matched_route:
        span_name = request.matched_route.pattern
    else:
        span_name = otel_wsgi.get_default_span_name(request_environ)

    span, token = _start_internal_or_server_span(
        tracer=tracer,
        span_name=span_name,
        start_time=start_time,
        context_carrier=request_environ,
        context_getter=otel_wsgi.wsgi_getter,
    )

    if span.is_recording():
        attributes = otel_wsgi.collect_request_attributes(request_environ)
        if request.matched_route:
            attributes[
                SpanAttributes.HTTP_ROUTE
            ] = request.matched_route.pattern
        for key, value in attributes.items():
            span.set_attribute(key, value)
        if span.kind == trace.SpanKind.SERVER:
            otel_wsgi.add_custom_request_headers(span, request_environ)

    activation = trace.use_span(span, end_on_exit=True)
    activation.__enter__()  # pylint: disable=E1101
    request_environ[_ENVIRON_ACTIVATION_KEY] = activation
    request_environ[_ENVIRON_SPAN_KEY] = span
    if token:
        request_environ[_ENVIRON_TOKEN] = token


def trace_tween_factory(handler, registry):
    settings = registry.settings
    enabled = asbool(settings.get(SETTING_TRACE_ENABLED, True))

    if not enabled:
        # If disabled, make a tween that signals to the
        # BeforeTraversal subscriber that tracing is disabled
        def disabled_tween(request):
            request.environ[_ENVIRON_ENABLED_KEY] = False
            return handler(request)

        return disabled_tween

    # make a request tracing function
    # pylint: disable=too-many-branches
    def trace_tween(request):
        # pylint: disable=E1101
        if _excluded_urls.url_disabled(request.url):
            request.environ[_ENVIRON_ENABLED_KEY] = False
            # short-circuit when we don't want to trace anything
            return handler(request)

        request.environ[_ENVIRON_ENABLED_KEY] = True
        request.environ[_ENVIRON_STARTTIME_KEY] = _time_ns()

        response = None
        status = None

        try:
            response = handler(request)
        except HTTPException as exc:
            # If the exception is a pyramid HTTPException,
            # that's still valuable information that isn't necessarily
            # a 500. For instance, HTTPFound is a 302.
            # As described in docs, Pyramid exceptions are all valid
            # response types
            response = exc
            raise
        except BaseException:
            # In the case that a non-HTTPException is bubbled up we
            # should infer a internal server error and raise
            status = "500 InternalServerError"
            raise
        finally:
            span = request.environ.get(_ENVIRON_SPAN_KEY)
            enabled = request.environ.get(_ENVIRON_ENABLED_KEY)
            if not span and enabled:
                _logger.warning(
                    "Pyramid environ's OpenTelemetry span missing."
                    "If the OpenTelemetry tween was added manually, make sure"
                    "PyramidInstrumentor().instrument_config(config) is called"
                )
            elif enabled:
                status = getattr(response, "status", status)

                if status is not None:
                    otel_wsgi.add_response_attributes(
                        span,
                        status,
                        getattr(response, "headerlist", None),
                    )

                if span.is_recording() and span.kind == trace.SpanKind.SERVER:
                    otel_wsgi.add_custom_response_headers(
                        span, getattr(response, "headerlist", None)
                    )

                propagator = get_global_response_propagator()
                if propagator and hasattr(response, "headers"):
                    propagator.inject(response.headers)

                activation = request.environ.get(_ENVIRON_ACTIVATION_KEY)

                if isinstance(response, HTTPException):
                    activation.__exit__(
                        type(response),
                        response,
                        getattr(response, "__traceback__", None),
                    )
                else:
                    activation.__exit__(None, None, None)

                env_token = request.environ.get(_ENVIRON_TOKEN, None)
                if env_token is not None:
                    context.detach(env_token)

        return response

    return trace_tween
