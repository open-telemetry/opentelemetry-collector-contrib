# project
from .conf import settings
from .compat import user_is_authenticated, get_resolver
from .utils import get_request_uri

from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...contrib import func_name
from ...ext import SpanTypes, http
from ...internal.logger import get_logger
from ...propagation.http import HTTPPropagator
from ...settings import config

# 3p
from django.core.exceptions import MiddlewareNotUsed
from django.conf import settings as django_settings
import django

try:
    from django.utils.deprecation import MiddlewareMixin

    MiddlewareClass = MiddlewareMixin
except ImportError:
    MiddlewareClass = object

log = get_logger(__name__)

EXCEPTION_MIDDLEWARE = "ddtrace.contrib.django.TraceExceptionMiddleware"
TRACE_MIDDLEWARE = "ddtrace.contrib.django.TraceMiddleware"
MIDDLEWARE = "MIDDLEWARE"
MIDDLEWARE_CLASSES = "MIDDLEWARE_CLASSES"

# Default views list available from:
#   https://github.com/django/django/blob/38e2fdadfd9952e751deed662edf4c496d238f28/django/views/defaults.py
# DEV: Django doesn't call `process_view` when falling back to one of these internal error handling views
# DEV: We only use these names when `span.resource == 'unknown'` and we have one of these status codes
_django_default_views = {
    400: "django.views.defaults.bad_request",
    403: "django.views.defaults.permission_denied",
    404: "django.views.defaults.page_not_found",
    500: "django.views.defaults.server_error",
}


def _analytics_enabled():
    return (
        (config.analytics_enabled and settings.ANALYTICS_ENABLED is not False) or settings.ANALYTICS_ENABLED is True
    ) and settings.ANALYTICS_SAMPLE_RATE is not None


def get_middleware_insertion_point():
    """Returns the attribute name and collection object for the Django middleware.

    If middleware cannot be found, returns None for the middleware collection.
    """
    middleware = getattr(django_settings, MIDDLEWARE, None)
    # Prioritise MIDDLEWARE over ..._CLASSES, but only in 1.10 and later.
    if middleware is not None and django.VERSION >= (1, 10):
        return MIDDLEWARE, middleware
    return MIDDLEWARE_CLASSES, getattr(django_settings, MIDDLEWARE_CLASSES, None)


def insert_trace_middleware():
    middleware_attribute, middleware = get_middleware_insertion_point()
    if middleware is not None and TRACE_MIDDLEWARE not in set(middleware):
        setattr(django_settings, middleware_attribute, type(middleware)((TRACE_MIDDLEWARE,)) + middleware)


def remove_trace_middleware():
    _, middleware = get_middleware_insertion_point()
    if middleware and TRACE_MIDDLEWARE in set(middleware):
        middleware.remove(TRACE_MIDDLEWARE)


def insert_exception_middleware():
    middleware_attribute, middleware = get_middleware_insertion_point()
    if middleware is not None and EXCEPTION_MIDDLEWARE not in set(middleware):
        setattr(django_settings, middleware_attribute, middleware + type(middleware)((EXCEPTION_MIDDLEWARE,)))


def remove_exception_middleware():
    _, middleware = get_middleware_insertion_point()
    if middleware and EXCEPTION_MIDDLEWARE in set(middleware):
        middleware.remove(EXCEPTION_MIDDLEWARE)


class InstrumentationMixin(MiddlewareClass):
    """
    Useful mixin base class for tracing middlewares
    """

    def __init__(self, get_response=None):
        # disable the middleware if the tracer is not enabled
        # or if the auto instrumentation is disabled
        self.get_response = get_response
        if not settings.AUTO_INSTRUMENT:
            raise MiddlewareNotUsed


class TraceExceptionMiddleware(InstrumentationMixin):
    """
    Middleware that traces exceptions raised
    """

    def process_exception(self, request, exception):
        try:
            span = _get_req_span(request)
            if span:
                span.set_tag(http.STATUS_CODE, "500")
                span.set_traceback()  # will set the exception info
        except Exception:
            log.debug("error processing exception", exc_info=True)


class TraceMiddleware(InstrumentationMixin):
    """
    Middleware that traces Django requests
    """

    def process_request(self, request):
        tracer = settings.TRACER
        if settings.DISTRIBUTED_TRACING:
            propagator = HTTPPropagator()
            context = propagator.extract(request.META)
            # Only need to active the new context if something was propagated
            if context.trace_id:
                tracer.context_provider.activate(context)
        try:
            span = tracer.trace(
                "django.request",
                service=settings.DEFAULT_SERVICE,
                resource="unknown",  # will be filled by process view
                span_type=SpanTypes.WEB,
            )

            # set analytics sample rate
            # DEV: django is special case maintains separate configuration from config api
            if _analytics_enabled() and settings.ANALYTICS_SAMPLE_RATE is not None:
                span.set_tag(
                    ANALYTICS_SAMPLE_RATE_KEY, settings.ANALYTICS_SAMPLE_RATE,
                )

            # Set HTTP Request tags
            span.set_tag(http.METHOD, request.method)
            span.set_tag(http.URL, get_request_uri(request))
            trace_query_string = settings.TRACE_QUERY_STRING
            if trace_query_string is None:
                trace_query_string = config.django.trace_query_string
            if trace_query_string:
                span.set_tag(http.QUERY_STRING, request.META["QUERY_STRING"])
            _set_req_span(request, span)
        except Exception:
            log.debug("error tracing request", exc_info=True)

    def process_view(self, request, view_func, *args, **kwargs):
        span = _get_req_span(request)
        if span:
            span.resource = func_name(view_func)

    def process_response(self, request, response):
        try:
            span = _get_req_span(request)
            if span:
                if response.status_code < 500 and span.error:
                    # remove any existing stack trace since it must have been
                    # handled appropriately
                    span._remove_exc_info()

                # If `process_view` was not called, try to determine the correct `span.resource` to set
                # DEV: `process_view` won't get called if a middle `process_request` returns an HttpResponse
                # DEV: `process_view` won't get called when internal error handlers are used (e.g. for 404 responses)
                if span.resource == "unknown":
                    try:
                        # Attempt to lookup the view function from the url resolver
                        #   https://github.com/django/django/blob/38e2fdadfd9952e751deed662edf4c496d238f28/django/core/handlers/base.py#L104-L113  # noqa
                        urlconf = None
                        if hasattr(request, "urlconf"):
                            urlconf = request.urlconf
                        resolver = get_resolver(urlconf)

                        # Try to resolve the Django view for handling this request
                        if getattr(request, "request_match", None):
                            request_match = request.request_match
                        else:
                            # This may raise a `django.urls.exceptions.Resolver404` exception
                            request_match = resolver.resolve(request.path_info)
                        span.resource = func_name(request_match.func)
                    except Exception:
                        log.debug("error determining request view function", exc_info=True)

                        # If the view could not be found, try to set from a static list of
                        # known internal error handler views
                        span.resource = _django_default_views.get(response.status_code, "unknown")

                span.set_tag(http.STATUS_CODE, response.status_code)
                span = _set_auth_tags(span, request)
                span.finish()
        except Exception:
            log.debug("error tracing request", exc_info=True)
        finally:
            return response


def _get_req_span(request):
    """ Return the datadog span from the given request. """
    return getattr(request, "_datadog_request_span", None)


def _set_req_span(request, span):
    """ Set the datadog span on the given request. """
    return setattr(request, "_datadog_request_span", span)


def _set_auth_tags(span, request):
    """ Patch any available auth tags from the request onto the span. """
    user = getattr(request, "user", None)
    if not user:
        return span

    if hasattr(user, "is_authenticated"):
        span.set_tag("django.user.is_authenticated", user_is_authenticated(user))

    uid = getattr(user, "pk", None)
    if uid:
        span.set_tag("django.user.id", uid)

    uname = getattr(user, "username", None)
    if uname:
        span.set_tag("django.user.name", uname)

    return span
