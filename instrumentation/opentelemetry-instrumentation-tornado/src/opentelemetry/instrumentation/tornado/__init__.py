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
This library uses OpenTelemetry to track web requests in Tornado applications.

Usage
-----

.. code-block:: python

    import tornado.web
    from opentelemetry.instrumentation.tornado import TornadoInstrumentor

    # apply tornado instrumentation
    TornadoInstrumentor().instrument()

    class Handler(tornado.web.RequestHandler):
        def get(self):
            self.set_status(200)

    app = tornado.web.Application([(r"/", Handler)])
    app.listen(8080)
    tornado.ioloop.IOLoop.current().start()
"""

import inspect
import typing
from collections import namedtuple
from functools import partial, wraps
from logging import getLogger

import tornado.web
import wrapt
from tornado.routing import Rule
from wrapt import wrap_function_wrapper

from opentelemetry import configuration, context, propagators, trace
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.tornado.version import __version__
from opentelemetry.instrumentation.utils import (
    extract_attributes_from_object,
    http_status_to_status_code,
    unwrap,
)
from opentelemetry.trace.propagation.textmap import DictGetter
from opentelemetry.trace.status import Status
from opentelemetry.util import ExcludeList, time_ns

from .client import fetch_async  # pylint: disable=E0401

_logger = getLogger(__name__)
_TraceContext = namedtuple("TraceContext", ["activation", "span", "token"])
_HANDLER_CONTEXT_KEY = "_otel_trace_context_key"
_OTEL_PATCHED_KEY = "_otel_patched_key"


def get_excluded_urls():
    urls = configuration.Configuration().TORNADO_EXCLUDED_URLS or ""
    if urls:
        urls = str.split(urls, ",")
    return ExcludeList(urls)


def get_traced_request_attrs():
    attrs = configuration.Configuration().TORNADO_TRACED_REQUEST_ATTRS or ""
    if attrs:
        attrs = [attr.strip() for attr in attrs.split(",")]
    else:
        attrs = []
    return attrs


_excluded_urls = get_excluded_urls()
_traced_attrs = get_traced_request_attrs()

carrier_getter = DictGetter()


class TornadoInstrumentor(BaseInstrumentor):
    patched_handlers = []
    original_handler_new = None

    def _instrument(self, **kwargs):
        """
        _instrument patches tornado.web.RequestHandler and tornado.httpclient.AsyncHTTPClient classes
        to automatically instrument requests both received and sent by Tornado.

        We don't patch RequestHandler._execute as it causes some issues with contextvars based context.
        Mainly the context fails to detach from within RequestHandler.on_finish() if it is attached inside
        RequestHandler._execute. Same issue plagues RequestHandler.initialize. RequestHandler.prepare works
        perfectly on the other hand as it executes in the same context as on_finish and log_exection which
        are patched to finish a span after a request is served.

        However, we cannot just patch RequestHandler's prepare method because it is supposed to be overwridden
        by sub-classes and since the parent prepare method does not do anything special, sub-classes don't
        have to call super() when overriding the method.

        In order to work around this, we patch the __init__ method of RequestHandler and then dynamically patch
        the prepare, on_finish and log_exception methods of the derived classes _only_ the first time we see them.
        Note that the patch does not apply on every single __init__ call, only the first one for the enture
        process lifetime.
        """
        tracer_provider = kwargs.get("tracer_provider")
        tracer = trace.get_tracer(__name__, __version__, tracer_provider)

        def handler_init(init, handler, args, kwargs):
            cls = handler.__class__
            if patch_handler_class(tracer, cls):
                self.patched_handlers.append(cls)
            return init(*args, **kwargs)

        wrap_function_wrapper(
            "tornado.web", "RequestHandler.__init__", handler_init
        )
        wrap_function_wrapper(
            "tornado.httpclient",
            "AsyncHTTPClient.fetch",
            partial(fetch_async, tracer),
        )

    def _uninstrument(self, **kwargs):
        unwrap(tornado.web.RequestHandler, "__init__")
        unwrap(tornado.httpclient.AsyncHTTPClient, "fetch")
        for handler in self.patched_handlers:
            unpatch_handler_class(handler)
        self.patched_handlers = []


def patch_handler_class(tracer, cls):
    if getattr(cls, _OTEL_PATCHED_KEY, False):
        return False

    setattr(cls, _OTEL_PATCHED_KEY, True)
    _wrap(cls, "prepare", partial(_prepare, tracer))
    _wrap(cls, "on_finish", partial(_on_finish, tracer))
    _wrap(cls, "log_exception", partial(_log_exception, tracer))
    return True


def unpatch_handler_class(cls):
    if not getattr(cls, _OTEL_PATCHED_KEY, False):
        return

    unwrap(cls, "prepare")
    unwrap(cls, "on_finish")
    unwrap(cls, "log_exception")
    delattr(cls, _OTEL_PATCHED_KEY)


def _wrap(cls, method_name, wrapper):
    original = getattr(cls, method_name)
    wrapper = wrapt.FunctionWrapper(original, wrapper)
    wrapt.apply_patch(cls, method_name, wrapper)


def _prepare(tracer, func, handler, args, kwargs):
    start_time = time_ns()
    request = handler.request
    if _excluded_urls.url_disabled(request.uri):
        return func(*args, **kwargs)
    _start_span(tracer, handler, start_time)
    return func(*args, **kwargs)


def _on_finish(tracer, func, handler, args, kwargs):
    _finish_span(tracer, handler)
    return func(*args, **kwargs)


def _log_exception(tracer, func, handler, args, kwargs):
    error = None
    if len(args) == 3:
        error = args[1]

    _finish_span(tracer, handler, error)
    return func(*args, **kwargs)


def _get_attributes_from_request(request):
    attrs = {
        "component": "tornado",
        "http.method": request.method,
        "http.scheme": request.protocol,
        "http.host": request.host,
        "http.target": request.path,
    }

    if request.host:
        attrs["http.host"] = request.host

    if request.remote_ip:
        attrs["net.peer.ip"] = request.remote_ip

    return extract_attributes_from_object(request, _traced_attrs, attrs)


def _get_operation_name(handler, request):
    full_class_name = type(handler).__name__
    class_name = full_class_name.rsplit(".")[-1]
    return "{0}.{1}".format(class_name, request.method.lower())


def _start_span(tracer, handler, start_time) -> _TraceContext:
    token = context.attach(
        propagators.extract(carrier_getter, handler.request.headers,)
    )

    span = tracer.start_span(
        _get_operation_name(handler, handler.request),
        kind=trace.SpanKind.SERVER,
        start_time=start_time,
    )
    if span.is_recording():
        attributes = _get_attributes_from_request(handler.request)
        for key, value in attributes.items():
            span.set_attribute(key, value)

    activation = tracer.use_span(span, end_on_exit=True)
    activation.__enter__()
    ctx = _TraceContext(activation, span, token)
    setattr(handler, _HANDLER_CONTEXT_KEY, ctx)
    return ctx


def _finish_span(tracer, handler, error=None):
    status_code = handler.get_status()
    reason = getattr(handler, "_reason")
    finish_args = (None, None, None)
    ctx = getattr(handler, _HANDLER_CONTEXT_KEY, None)

    if error:
        if isinstance(error, tornado.web.HTTPError):
            status_code = error.status_code
            if not ctx and status_code == 404:
                ctx = _start_span(tracer, handler, time_ns())
        if status_code != 404:
            finish_args = (
                type(error),
                error,
                getattr(error, "__traceback__", None),
            )
            status_code = 500
            reason = None

    if not ctx:
        return

    if ctx.span.is_recording():
        if reason:
            ctx.span.set_attribute("http.status_text", reason)
        ctx.span.set_attribute("http.status_code", status_code)
        ctx.span.set_status(
            Status(
                status_code=http_status_to_status_code(status_code),
                description=reason,
            )
        )

    ctx.activation.__exit__(*finish_args)
    context.detach(ctx.token)
    delattr(handler, _HANDLER_CONTEXT_KEY)
