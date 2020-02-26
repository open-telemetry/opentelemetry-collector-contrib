import pyramid.renderers
from pyramid.settings import asbool
from pyramid.httpexceptions import HTTPException
from ddtrace.vendor import wrapt

# project
import ddtrace
from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...ext import SpanTypes, http
from ...internal.logger import get_logger
from ...propagation.http import HTTPPropagator
from ...settings import config
from .constants import (
    SETTINGS_TRACER,
    SETTINGS_SERVICE,
    SETTINGS_TRACE_ENABLED,
    SETTINGS_DISTRIBUTED_TRACING,
    SETTINGS_ANALYTICS_ENABLED,
    SETTINGS_ANALYTICS_SAMPLE_RATE,
)


log = get_logger(__name__)

DD_TWEEN_NAME = 'ddtrace.contrib.pyramid:trace_tween_factory'
DD_SPAN = '_datadog_span'


def trace_pyramid(config):
    config.include('ddtrace.contrib.pyramid')


def includeme(config):
    # Add our tween just before the default exception handler
    config.add_tween(DD_TWEEN_NAME, over=pyramid.tweens.EXCVIEW)
    # ensure we only patch the renderer once.
    if not isinstance(pyramid.renderers.RendererHelper.render, wrapt.ObjectProxy):
        wrapt.wrap_function_wrapper('pyramid.renderers', 'RendererHelper.render', trace_render)


def trace_render(func, instance, args, kwargs):
    # If the request is not traced, we do not trace
    request = kwargs.get('request', {})
    if not request:
        log.debug('No request passed to render, will not be traced')
        return func(*args, **kwargs)
    span = getattr(request, DD_SPAN, None)
    if not span:
        log.debug('No span found in request, will not be traced')
        return func(*args, **kwargs)

    with span.tracer.trace('pyramid.render', span_type=SpanTypes.TEMPLATE) as span:
        return func(*args, **kwargs)


def trace_tween_factory(handler, registry):
    # configuration
    settings = registry.settings
    service = settings.get(SETTINGS_SERVICE) or 'pyramid'
    tracer = settings.get(SETTINGS_TRACER) or ddtrace.tracer
    enabled = asbool(settings.get(SETTINGS_TRACE_ENABLED, tracer.enabled))
    distributed_tracing = asbool(settings.get(SETTINGS_DISTRIBUTED_TRACING, True))

    if enabled:
        # make a request tracing function
        def trace_tween(request):
            if distributed_tracing:
                propagator = HTTPPropagator()
                context = propagator.extract(request.headers)
                # only need to active the new context if something was propagated
                if context.trace_id:
                    tracer.context_provider.activate(context)
            with tracer.trace('pyramid.request', service=service, resource='404', span_type=SpanTypes.WEB) as span:
                # Configure trace search sample rate
                # DEV: pyramid is special case maintains separate configuration from config api
                analytics_enabled = settings.get(SETTINGS_ANALYTICS_ENABLED)

                if (
                    config.analytics_enabled and analytics_enabled is not False
                ) or analytics_enabled is True:
                    span.set_tag(
                        ANALYTICS_SAMPLE_RATE_KEY,
                        settings.get(SETTINGS_ANALYTICS_SAMPLE_RATE, True)
                    )

                setattr(request, DD_SPAN, span)  # used to find the tracer in templates
                response = None
                try:
                    response = handler(request)
                except HTTPException as e:
                    # If the exception is a pyramid HTTPException,
                    # that's still valuable information that isn't necessarily
                    # a 500. For instance, HTTPFound is a 302.
                    # As described in docs, Pyramid exceptions are all valid
                    # response types
                    response = e
                    raise
                except BaseException:
                    span.set_tag(http.STATUS_CODE, 500)
                    raise
                finally:
                    # set request tags
                    span.set_tag(http.URL, request.path_url)
                    span.set_tag(http.METHOD, request.method)
                    if config.pyramid.trace_query_string:
                        span.set_tag(http.QUERY_STRING, request.query_string)
                    if request.matched_route:
                        span.resource = '{} {}'.format(request.method, request.matched_route.name)
                        span.set_tag('pyramid.route.name', request.matched_route.name)
                    # set response tags
                    if response:
                        span.set_tag(http.STATUS_CODE, response.status_code)
                        if 500 <= response.status_code < 600:
                            span.error = 1
                return response
        return trace_tween

    # if timing support is not enabled, return the original handler
    return handler
