from logging import getLogger

from pyramid.events import BeforeTraversal
from pyramid.httpexceptions import HTTPException
from pyramid.settings import asbool
from pyramid.tweens import EXCVIEW

import opentelemetry.instrumentation.wsgi as otel_wsgi
from opentelemetry import configuration, context, propagators, trace
from opentelemetry.instrumentation.pyramid.version import __version__
from opentelemetry.util import ExcludeList, time_ns

TWEEN_NAME = "opentelemetry.instrumentation.pyramid.trace_tween_factory"
SETTING_TRACE_ENABLED = "opentelemetry-pyramid.trace_enabled"

_ENVIRON_STARTTIME_KEY = "opentelemetry-pyramid.starttime_key"
_ENVIRON_SPAN_KEY = "opentelemetry-pyramid.span_key"
_ENVIRON_ACTIVATION_KEY = "opentelemetry-pyramid.activation_key"
_ENVIRON_ENABLED_KEY = "opentelemetry-pyramid.tracing_enabled_key"
_ENVIRON_TOKEN = "opentelemetry-pyramid.token"

_logger = getLogger(__name__)


def get_excluded_urls():
    urls = configuration.Configuration().PYRAMID_EXCLUDED_URLS or []
    if urls:
        urls = str.split(urls, ",")
    return ExcludeList(urls)


_excluded_urls = get_excluded_urls()


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
    environ = request.environ
    span_name = otel_wsgi.get_default_span_name(environ)

    enabled = environ.get(_ENVIRON_ENABLED_KEY)
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

    start_time = environ.get(_ENVIRON_STARTTIME_KEY)

    token = context.attach(
        propagators.extract(otel_wsgi.carrier_getter, environ)
    )
    tracer = trace.get_tracer(__name__, __version__)

    if request.matched_route:
        span_name = request.matched_route.pattern
    else:
        span_name = otel_wsgi.get_default_span_name(environ)

    span = tracer.start_span(
        span_name, kind=trace.SpanKind.SERVER, start_time=start_time,
    )

    if span.is_recording():
        attributes = otel_wsgi.collect_request_attributes(environ)
        if request.matched_route:
            attributes["http.route"] = request.matched_route.pattern
        for key, value in attributes.items():
            span.set_attribute(key, value)

    activation = tracer.use_span(span, end_on_exit=True)
    activation.__enter__()
    environ[_ENVIRON_ACTIVATION_KEY] = activation
    environ[_ENVIRON_SPAN_KEY] = span
    environ[_ENVIRON_TOKEN] = token


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
    def trace_tween(request):
        if _excluded_urls.url_disabled(request.url):
            request.environ[_ENVIRON_ENABLED_KEY] = False
            # short-circuit when we don't want to trace anything
            return handler(request)

        request.environ[_ENVIRON_ENABLED_KEY] = True
        request.environ[_ENVIRON_STARTTIME_KEY] = time_ns()

        try:
            response = handler(request)
            response_or_exception = response
        except HTTPException as exc:
            # If the exception is a pyramid HTTPException,
            # that's still valuable information that isn't necessarily
            # a 500. For instance, HTTPFound is a 302.
            # As described in docs, Pyramid exceptions are all valid
            # response types
            response_or_exception = exc
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
                otel_wsgi.add_response_attributes(
                    span,
                    response_or_exception.status,
                    response_or_exception.headers,
                )

                activation = request.environ.get(_ENVIRON_ACTIVATION_KEY)

                if isinstance(response_or_exception, HTTPException):
                    activation.__exit__(
                        type(response_or_exception),
                        response_or_exception,
                        getattr(response_or_exception, "__traceback__", None),
                    )
                else:
                    activation.__exit__(None, None, None)

                context.detach(request.environ.get(_ENVIRON_TOKEN))

        return response

    return trace_tween
