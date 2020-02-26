from ..context import Context
from ..internal.logger import get_logger

from .utils import get_wsgi_header

log = get_logger(__name__)

# HTTP headers one should set for distributed tracing.
# These are cross-language (eg: Python, Go and other implementations should honor these)
HTTP_HEADER_TRACE_ID = 'x-datadog-trace-id'
HTTP_HEADER_PARENT_ID = 'x-datadog-parent-id'
HTTP_HEADER_SAMPLING_PRIORITY = 'x-datadog-sampling-priority'
HTTP_HEADER_ORIGIN = 'x-datadog-origin'


# Note that due to WSGI spec we have to also check for uppercased and prefixed
# versions of these headers
POSSIBLE_HTTP_HEADER_TRACE_IDS = frozenset(
    [HTTP_HEADER_TRACE_ID, get_wsgi_header(HTTP_HEADER_TRACE_ID)]
)
POSSIBLE_HTTP_HEADER_PARENT_IDS = frozenset(
    [HTTP_HEADER_PARENT_ID, get_wsgi_header(HTTP_HEADER_PARENT_ID)]
)
POSSIBLE_HTTP_HEADER_SAMPLING_PRIORITIES = frozenset(
    [HTTP_HEADER_SAMPLING_PRIORITY, get_wsgi_header(HTTP_HEADER_SAMPLING_PRIORITY)]
)
POSSIBLE_HTTP_HEADER_ORIGIN = frozenset(
    [HTTP_HEADER_ORIGIN, get_wsgi_header(HTTP_HEADER_ORIGIN)]
)


class HTTPPropagator(object):
    """A HTTP Propagator using HTTP headers as carrier."""

    def inject(self, span_context, headers):
        """Inject Context attributes that have to be propagated as HTTP headers.

        Here is an example using `requests`::

            import requests
            from ddtrace.propagation.http import HTTPPropagator

            def parent_call():
                with tracer.trace('parent_span') as span:
                    headers = {}
                    propagator = HTTPPropagator()
                    propagator.inject(span.context, headers)
                    url = '<some RPC endpoint>'
                    r = requests.get(url, headers=headers)

        :param Context span_context: Span context to propagate.
        :param dict headers: HTTP headers to extend with tracing attributes.
        """
        headers[HTTP_HEADER_TRACE_ID] = str(span_context.trace_id)
        headers[HTTP_HEADER_PARENT_ID] = str(span_context.span_id)
        sampling_priority = span_context.sampling_priority
        # Propagate priority only if defined
        if sampling_priority is not None:
            headers[HTTP_HEADER_SAMPLING_PRIORITY] = str(span_context.sampling_priority)
        # Propagate origin only if defined
        if span_context._dd_origin is not None:
            headers[HTTP_HEADER_ORIGIN] = str(span_context._dd_origin)

    @staticmethod
    def extract_header_value(possible_header_names, headers, default=None):
        for header, value in headers.items():
            for header_name in possible_header_names:
                if header.lower() == header_name.lower():
                    return value

        return default

    @staticmethod
    def extract_trace_id(headers):
        return int(
            HTTPPropagator.extract_header_value(
                POSSIBLE_HTTP_HEADER_TRACE_IDS, headers, default=0,
            )
        )

    @staticmethod
    def extract_parent_span_id(headers):
        return int(
            HTTPPropagator.extract_header_value(
                POSSIBLE_HTTP_HEADER_PARENT_IDS, headers, default=0,
            )
        )

    @staticmethod
    def extract_sampling_priority(headers):
        return HTTPPropagator.extract_header_value(
            POSSIBLE_HTTP_HEADER_SAMPLING_PRIORITIES, headers,
        )

    @staticmethod
    def extract_origin(headers):
        return HTTPPropagator.extract_header_value(
            POSSIBLE_HTTP_HEADER_ORIGIN, headers,
        )

    def extract(self, headers):
        """Extract a Context from HTTP headers into a new Context.

        Here is an example from a web endpoint::

            from ddtrace.propagation.http import HTTPPropagator

            def my_controller(url, headers):
                propagator = HTTPPropagator()
                context = propagator.extract(headers)
                tracer.context_provider.activate(context)

                with tracer.trace('my_controller') as span:
                    span.set_meta('http.url', url)

        :param dict headers: HTTP headers to extract tracing attributes.
        :return: New `Context` with propagated attributes.
        """
        if not headers:
            return Context()

        try:
            trace_id = HTTPPropagator.extract_trace_id(headers)
            parent_span_id = HTTPPropagator.extract_parent_span_id(headers)
            sampling_priority = HTTPPropagator.extract_sampling_priority(headers)
            origin = HTTPPropagator.extract_origin(headers)

            if sampling_priority is not None:
                sampling_priority = int(sampling_priority)

            return Context(
                trace_id=trace_id,
                span_id=parent_span_id,
                sampling_priority=sampling_priority,
                _dd_origin=origin,
            )
        # If headers are invalid and cannot be parsed, return a new context and log the issue.
        except Exception:
            log.debug(
                'invalid x-datadog-* headers, trace-id: %s, parent-id: %s, priority: %s, origin: %s',
                headers.get(HTTP_HEADER_TRACE_ID, 0),
                headers.get(HTTP_HEADER_PARENT_ID, 0),
                headers.get(HTTP_HEADER_SAMPLING_PRIORITY),
                headers.get(HTTP_HEADER_ORIGIN, ''),
                exc_info=True,
            )
            return Context()
