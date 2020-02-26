import sys

from webob import Request
from pylons import config

from .renderer import trace_rendering
from .constants import CONFIG_MIDDLEWARE

from ...compat import reraise
from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...ext import SpanTypes, http
from ...internal.logger import get_logger
from ...propagation.http import HTTPPropagator
from ...settings import config as ddconfig


log = get_logger(__name__)


class PylonsTraceMiddleware(object):

    def __init__(self, app, tracer, service='pylons', distributed_tracing=True):
        self.app = app
        self._service = service
        self._distributed_tracing = distributed_tracing
        self._tracer = tracer

        # register middleware reference
        config[CONFIG_MIDDLEWARE] = self

        # add template tracing
        trace_rendering()

    def __call__(self, environ, start_response):
        if self._distributed_tracing:
            # retrieve distributed tracing headers
            request = Request(environ)
            propagator = HTTPPropagator()
            context = propagator.extract(request.headers)
            # only need to active the new context if something was propagated
            if context.trace_id:
                self._tracer.context_provider.activate(context)

        with self._tracer.trace('pylons.request', service=self._service, span_type=SpanTypes.WEB) as span:
            # Set the service in tracer.trace() as priority sampling requires it to be
            # set as early as possible when different services share one single agent.

            # set analytics sample rate with global config enabled
            span.set_tag(
                ANALYTICS_SAMPLE_RATE_KEY,
                ddconfig.pylons.get_analytics_sample_rate(use_global_config=True)
            )

            if not span.sampled:
                return self.app(environ, start_response)

            # tentative on status code, otherwise will be caught by except below
            def _start_response(status, *args, **kwargs):
                """ a patched response callback which will pluck some metadata. """
                http_code = int(status.split()[0])
                span.set_tag(http.STATUS_CODE, http_code)
                if http_code >= 500:
                    span.error = 1
                return start_response(status, *args, **kwargs)

            try:
                return self.app(environ, _start_response)
            except Exception as e:
                # store current exceptions info so we can re-raise it later
                (typ, val, tb) = sys.exc_info()

                # e.code can either be a string or an int
                code = getattr(e, 'code', 500)
                try:
                    code = int(code)
                    if not 100 <= code < 600:
                        code = 500
                except Exception:
                    code = 500
                span.set_tag(http.STATUS_CODE, code)
                span.error = 1

                # re-raise the original exception with its original traceback
                reraise(typ, val, tb=tb)
            except SystemExit:
                span.set_tag(http.STATUS_CODE, 500)
                span.error = 1
                raise
            finally:
                controller = environ.get('pylons.routes_dict', {}).get('controller')
                action = environ.get('pylons.routes_dict', {}).get('action')

                # There are cases where users re-route requests and manually
                # set resources. If this is so, don't do anything, otherwise
                # set the resource to the controller / action that handled it.
                if span.resource == span.name:
                    span.resource = '%s.%s' % (controller, action)

                span.set_tags({
                    http.METHOD: environ.get('REQUEST_METHOD'),
                    http.URL: '%s://%s:%s%s' % (environ.get('wsgi.url_scheme'),
                                                environ.get('SERVER_NAME'),
                                                environ.get('SERVER_PORT'),
                                                environ.get('PATH_INFO')),
                    'pylons.user': environ.get('REMOTE_USER', ''),
                    'pylons.route.controller': controller,
                    'pylons.route.action': action,
                })
                if ddconfig.pylons.trace_query_string:
                    span.set_tag(http.QUERY_STRING, environ.get('QUERY_STRING'))
