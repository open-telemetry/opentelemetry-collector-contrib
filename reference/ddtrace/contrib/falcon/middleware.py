import sys

from ddtrace.ext import SpanTypes, http as httpx
from ddtrace.http import store_request_headers, store_response_headers
from ddtrace.propagation.http import HTTPPropagator

from ...compat import iteritems
from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...settings import config


class TraceMiddleware(object):

    def __init__(self, tracer, service='falcon', distributed_tracing=True):
        # store tracing references
        self.tracer = tracer
        self.service = service
        self._distributed_tracing = distributed_tracing

    def process_request(self, req, resp):
        if self._distributed_tracing:
            # Falcon uppercases all header names.
            headers = dict((k.lower(), v) for k, v in iteritems(req.headers))
            propagator = HTTPPropagator()
            context = propagator.extract(headers)
            # Only activate the new context if there was a trace id extracted
            if context.trace_id:
                self.tracer.context_provider.activate(context)

        span = self.tracer.trace(
            'falcon.request',
            service=self.service,
            span_type=SpanTypes.WEB,
        )

        # set analytics sample rate with global config enabled
        span.set_tag(
            ANALYTICS_SAMPLE_RATE_KEY,
            config.falcon.get_analytics_sample_rate(use_global_config=True)
        )

        span.set_tag(httpx.METHOD, req.method)
        span.set_tag(httpx.URL, req.url)
        if config.falcon.trace_query_string:
            span.set_tag(httpx.QUERY_STRING, req.query_string)

        # Note: any request header set after this line will not be stored in the span
        store_request_headers(req.headers, span, config.falcon)

    def process_resource(self, req, resp, resource, params):
        span = self.tracer.current_span()
        if not span:
            return  # unexpected
        span.resource = '%s %s' % (req.method, _name(resource))

    def process_response(self, req, resp, resource, req_succeeded=None):
        # req_succeded is not a kwarg in the API, but we need that to support
        # Falcon 1.0 that doesn't provide this argument
        span = self.tracer.current_span()
        if not span:
            return  # unexpected

        status = httpx.normalize_status_code(resp.status)

        # Note: any response header set after this line will not be stored in the span
        store_response_headers(resp._headers, span, config.falcon)

        # FIXME[matt] falcon does not map errors or unmatched routes
        # to proper status codes, so we we have to try to infer them
        # here. See https://github.com/falconry/falcon/issues/606
        if resource is None:
            status = '404'
            span.resource = '%s 404' % req.method
            span.set_tag(httpx.STATUS_CODE, status)
            span.finish()
            return

        err_type = sys.exc_info()[0]
        if err_type is not None:
            if req_succeeded is None:
                # backward-compatibility with Falcon 1.0; any version
                # greater than 1.0 has req_succeded in [True, False]
                # TODO[manu]: drop the support at some point
                status = _detect_and_set_status_error(err_type, span)
            elif req_succeeded is False:
                # Falcon 1.1+ provides that argument that is set to False
                # if get an Exception (404 is still an exception)
                status = _detect_and_set_status_error(err_type, span)

        span.set_tag(httpx.STATUS_CODE, status)

        # Emit span hook for this response
        # DEV: Emit before closing so they can overwrite `span.resource` if they want
        config.falcon.hooks._emit('request', span, req, resp)

        # Close the span
        span.finish()


def _is_404(err_type):
    return 'HTTPNotFound' in err_type.__name__


def _detect_and_set_status_error(err_type, span):
    """Detect the HTTP status code from the current stacktrace and
    set the traceback to the given Span
    """
    if not _is_404(err_type):
        span.set_traceback()
        return '500'
    elif _is_404(err_type):
        return '404'


def _name(r):
    return '%s.%s' % (r.__module__, r.__class__.__name__)
