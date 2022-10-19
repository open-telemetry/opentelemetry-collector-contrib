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

Configuration
-------------

The following environment variables are supported as configuration options:

- OTEL_PYTHON_TORNADO_EXCLUDED_URLS

A comma separated list of paths that should not be automatically traced. For example, if this is set to

::

    export OTEL_PYTHON_TORNADO_EXCLUDED_URLS='/healthz,/ping'

Then any requests made to ``/healthz`` and ``/ping`` will not be automatically traced.

Request attributes
******************

To extract certain attributes from Tornado's request object and use them as span attributes, set the environment variable ``OTEL_PYTHON_TORNADO_TRACED_REQUEST_ATTRS`` to a comma
delimited list of request attribute names.

For example,

::

    export OTEL_PYTHON_TORNADO_TRACED_REQUEST_ATTRS='uri,query'

will extract path_info and content_type attributes from every traced request and add them as span attributes.

Request/Response hooks
**********************

Tornado instrumentation supports extending tracing behaviour with the help of hooks.
Its ``instrument()`` method accepts three optional functions that get called back with the
created span and some other contextual information. Example:

.. code-block:: python

    # will be called for each incoming request to Tornado
    # web server. `handler` is an instance of
    # `tornado.web.RequestHandler`.
    def server_request_hook(span, handler):
        pass

    # will be called just before sending out a request with
    # `tornado.httpclient.AsyncHTTPClient.fetch`.
    # `request` is an instance of ``tornado.httpclient.HTTPRequest`.
    def client_request_hook(span, request):
        pass

    # will be called after a outgoing request made with
    # `tornado.httpclient.AsyncHTTPClient.fetch` finishes.
    # `response`` is an instance of ``Future[tornado.httpclient.HTTPResponse]`.
    def client_resposne_hook(span, future):
        pass

    # apply tornado instrumentation with hooks
    TornadoInstrumentor().instrument(
        server_request_hook=server_request_hook,
        client_request_hook=client_request_hook,
        client_response_hook=client_resposne_hook
    )

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
Request header names in tornado are case insensitive. So, giving header name as ``CUStomHeader`` in environment variable will be able capture header with name ``customheader``.

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
Response header names captured in tornado are case insensitive. So, giving header name as ``CUStomHeader`` in environment variable will be able capture header with name ``customheader``.

The name of the added span attribute will follow the format ``http.response.header.<header_name>`` where ``<header_name>`` being the normalized HTTP header name (lowercase, with - characters replaced by _ ).
The value of the attribute will be single item list containing all the header values.

Example of the added span attribute,
``http.response.header.custom_response_header = ["<value1>,<value2>"]``

Note:
    Environment variable names to capture http headers are still experimental, and thus are subject to change.

API
---
"""


from collections import namedtuple
from functools import partial
from logging import getLogger
from time import time_ns
from timeit import default_timer
from typing import Collection, Dict

import tornado.web
import wrapt
from wrapt import wrap_function_wrapper

from opentelemetry import context, trace
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.propagators import (
    FuncSetter,
    get_global_response_propagator,
)
from opentelemetry.instrumentation.tornado.package import _instruments
from opentelemetry.instrumentation.tornado.version import __version__
from opentelemetry.instrumentation.utils import (
    _start_internal_or_server_span,
    extract_attributes_from_object,
    http_status_to_status_code,
    unwrap,
)
from opentelemetry.metrics import get_meter
from opentelemetry.metrics._internal.instrument import Histogram
from opentelemetry.propagators import textmap
from opentelemetry.semconv.metrics import MetricInstruments
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace.status import Status, StatusCode
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
    get_custom_headers,
    get_excluded_urls,
    get_traced_request_attrs,
    normalise_request_header_name,
    normalise_response_header_name,
)

from .client import fetch_async  # pylint: disable=E0401

_logger = getLogger(__name__)
_TraceContext = namedtuple("TraceContext", ["activation", "span", "token"])
_HANDLER_CONTEXT_KEY = "_otel_trace_context_key"
_OTEL_PATCHED_KEY = "_otel_patched_key"

_START_TIME = "start_time"

_excluded_urls = get_excluded_urls("TORNADO")
_traced_request_attrs = get_traced_request_attrs("TORNADO")
response_propagation_setter = FuncSetter(tornado.web.RequestHandler.add_header)


class TornadoInstrumentor(BaseInstrumentor):
    patched_handlers = []
    original_handler_new = None

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """
        _instrument patches tornado.web.RequestHandler and tornado.httpclient.AsyncHTTPClient classes
        to automatically instrument requests both received and sent by Tornado.

        We don't patch RequestHandler._execute as it causes some issues with contextvars based context.
        Mainly the context fails to detach from within RequestHandler.on_finish() if it is attached inside
        RequestHandler._execute. Same issue plagues RequestHandler.initialize. RequestHandler.prepare works
        perfectly on the other hand as it executes in the same context as on_finish and log_exection which
        are patched to finish a span after a request is served.

        However, we cannot just patch RequestHandler's prepare method because it is supposed to be overridden
        by sub-classes and since the parent prepare method does not do anything special, sub-classes don't
        have to call super() when overriding the method.

        In order to work around this, we patch the __init__ method of RequestHandler and then dynamically patch
        the prepare, on_finish and log_exception methods of the derived classes _only_ the first time we see them.
        Note that the patch does not apply on every single __init__ call, only the first one for the entire
        process lifetime.
        """
        tracer_provider = kwargs.get("tracer_provider")
        tracer = trace.get_tracer(__name__, __version__, tracer_provider)

        meter_provider = kwargs.get("meter_provider")
        meter = get_meter(__name__, __version__, meter_provider)

        client_histograms = _create_client_histograms(meter)
        server_histograms = _create_server_histograms(meter)

        client_request_hook = kwargs.get("client_request_hook", None)
        client_response_hook = kwargs.get("client_response_hook", None)
        server_request_hook = kwargs.get("server_request_hook", None)

        def handler_init(init, handler, args, kwargs):
            cls = handler.__class__
            if patch_handler_class(
                tracer, server_histograms, cls, server_request_hook
            ):
                self.patched_handlers.append(cls)
            return init(*args, **kwargs)

        wrap_function_wrapper(
            "tornado.web", "RequestHandler.__init__", handler_init
        )
        wrap_function_wrapper(
            "tornado.httpclient",
            "AsyncHTTPClient.fetch",
            partial(
                fetch_async,
                tracer,
                client_request_hook,
                client_response_hook,
                client_histograms[MetricInstruments.HTTP_CLIENT_DURATION],
                client_histograms[MetricInstruments.HTTP_CLIENT_REQUEST_SIZE],
                client_histograms[MetricInstruments.HTTP_CLIENT_RESPONSE_SIZE],
            ),
        )

    def _uninstrument(self, **kwargs):
        unwrap(tornado.web.RequestHandler, "__init__")
        unwrap(tornado.httpclient.AsyncHTTPClient, "fetch")
        for handler in self.patched_handlers:
            unpatch_handler_class(handler)
        self.patched_handlers = []


def _create_server_histograms(meter) -> Dict[str, Histogram]:
    histograms = {
        MetricInstruments.HTTP_SERVER_DURATION: meter.create_histogram(
            name=MetricInstruments.HTTP_SERVER_DURATION,
            unit="ms",
            description="measures the duration outbound HTTP requests",
        ),
        MetricInstruments.HTTP_SERVER_REQUEST_SIZE: meter.create_histogram(
            name=MetricInstruments.HTTP_SERVER_REQUEST_SIZE,
            unit="By",
            description="measures the size of HTTP request messages (compressed)",
        ),
        MetricInstruments.HTTP_SERVER_RESPONSE_SIZE: meter.create_histogram(
            name=MetricInstruments.HTTP_SERVER_RESPONSE_SIZE,
            unit="By",
            description="measures the size of HTTP response messages (compressed)",
        ),
        MetricInstruments.HTTP_SERVER_ACTIVE_REQUESTS: meter.create_up_down_counter(
            name=MetricInstruments.HTTP_SERVER_ACTIVE_REQUESTS,
            unit="requests",
            description="measures the number of concurrent HTTP requests that are currently in-flight",
        ),
    }

    return histograms


def _create_client_histograms(meter) -> Dict[str, Histogram]:
    histograms = {
        MetricInstruments.HTTP_CLIENT_DURATION: meter.create_histogram(
            name=MetricInstruments.HTTP_CLIENT_DURATION,
            unit="ms",
            description="measures the duration outbound HTTP requests",
        ),
        MetricInstruments.HTTP_CLIENT_REQUEST_SIZE: meter.create_histogram(
            name=MetricInstruments.HTTP_CLIENT_REQUEST_SIZE,
            unit="By",
            description="measures the size of HTTP request messages (compressed)",
        ),
        MetricInstruments.HTTP_CLIENT_RESPONSE_SIZE: meter.create_histogram(
            name=MetricInstruments.HTTP_CLIENT_RESPONSE_SIZE,
            unit="By",
            description="measures the size of HTTP response messages (compressed)",
        ),
    }

    return histograms


def patch_handler_class(tracer, server_histograms, cls, request_hook=None):
    if getattr(cls, _OTEL_PATCHED_KEY, False):
        return False

    setattr(cls, _OTEL_PATCHED_KEY, True)
    _wrap(
        cls,
        "prepare",
        partial(_prepare, tracer, server_histograms, request_hook),
    )
    _wrap(cls, "on_finish", partial(_on_finish, tracer, server_histograms))
    _wrap(
        cls,
        "log_exception",
        partial(_log_exception, tracer, server_histograms),
    )
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


def _prepare(
    tracer, server_histograms, request_hook, func, handler, args, kwargs
):
    server_histograms[_START_TIME] = default_timer()

    request = handler.request
    if _excluded_urls.url_disabled(request.uri):
        return func(*args, **kwargs)

    _record_prepare_metrics(server_histograms, handler)

    ctx = _start_span(tracer, handler)
    if request_hook:
        request_hook(ctx.span, handler)
    return func(*args, **kwargs)


def _on_finish(tracer, server_histograms, func, handler, args, kwargs):
    response = func(*args, **kwargs)

    _record_on_finish_metrics(server_histograms, handler)

    _finish_span(tracer, handler)

    return response


def _log_exception(tracer, server_histograms, func, handler, args, kwargs):
    error = None
    if len(args) == 3:
        error = args[1]

    _record_on_finish_metrics(server_histograms, handler, error)

    _finish_span(tracer, handler, error)
    return func(*args, **kwargs)


def _collect_custom_request_headers_attributes(request_headers):
    custom_request_headers_name = get_custom_headers(
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST
    )
    attributes = {}
    for header_name in custom_request_headers_name:
        header_values = request_headers.get(header_name)
        if header_values:
            key = normalise_request_header_name(header_name.lower())
            attributes[key] = [header_values]
    return attributes


def _collect_custom_response_headers_attributes(response_headers):
    custom_response_headers_name = get_custom_headers(
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE
    )
    attributes = {}
    for header_name in custom_response_headers_name:
        header_values = response_headers.get(header_name)
        if header_values:
            key = normalise_response_header_name(header_name.lower())
            attributes[key] = [header_values]
    return attributes


def _get_attributes_from_request(request):
    attrs = {
        SpanAttributes.HTTP_METHOD: request.method,
        SpanAttributes.HTTP_SCHEME: request.protocol,
        SpanAttributes.HTTP_HOST: request.host,
        SpanAttributes.HTTP_TARGET: request.path,
    }

    if request.remote_ip:
        # NET_PEER_IP is the address of the network peer
        # HTTP_CLIENT_IP is the address of the client, which might be different
        # if Tornado is set to trust X-Forwarded-For headers (xheaders=True)
        attrs[SpanAttributes.HTTP_CLIENT_IP] = request.remote_ip
        if hasattr(request.connection, "context") and getattr(
            request.connection.context, "_orig_remote_ip", None
        ):
            attrs[
                SpanAttributes.NET_PEER_IP
            ] = request.connection.context._orig_remote_ip

    return extract_attributes_from_object(
        request, _traced_request_attrs, attrs
    )


def _get_operation_name(handler, request):
    full_class_name = type(handler).__name__
    class_name = full_class_name.rsplit(".")[-1]
    return f"{class_name}.{request.method.lower()}"


def _get_full_handler_name(handler):
    klass = type(handler)
    return f"{klass.__module__}.{klass.__qualname__}"


def _start_span(tracer, handler) -> _TraceContext:
    span, token = _start_internal_or_server_span(
        tracer=tracer,
        span_name=_get_operation_name(handler, handler.request),
        start_time=time_ns(),
        context_carrier=handler.request.headers,
        context_getter=textmap.default_getter,
    )

    if span.is_recording():
        attributes = _get_attributes_from_request(handler.request)
        for key, value in attributes.items():
            span.set_attribute(key, value)
        span.set_attribute("tornado.handler", _get_full_handler_name(handler))
        if span.is_recording() and span.kind == trace.SpanKind.SERVER:
            custom_attributes = _collect_custom_request_headers_attributes(
                handler.request.headers
            )
            if len(custom_attributes) > 0:
                span.set_attributes(custom_attributes)

    activation = trace.use_span(span, end_on_exit=True)
    activation.__enter__()  # pylint: disable=E1101
    ctx = _TraceContext(activation, span, token)
    setattr(handler, _HANDLER_CONTEXT_KEY, ctx)

    # finish handler is called after the response is sent back to
    # the client so it is too late to inject trace response headers
    # there.
    propagator = get_global_response_propagator()
    if propagator:
        propagator.inject(handler, setter=response_propagation_setter)

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
                ctx = _start_span(tracer, handler)
        else:
            status_code = 500
            reason = None
        if status_code >= 500:
            finish_args = (
                type(error),
                error,
                getattr(error, "__traceback__", None),
            )

    if not ctx:
        return

    if ctx.span.is_recording():
        ctx.span.set_attribute(SpanAttributes.HTTP_STATUS_CODE, status_code)
        otel_status_code = http_status_to_status_code(
            status_code, server_span=True
        )
        otel_status_description = None
        if otel_status_code is StatusCode.ERROR:
            otel_status_description = reason
        ctx.span.set_status(
            Status(
                status_code=otel_status_code,
                description=otel_status_description,
            )
        )
        if ctx.span.is_recording() and ctx.span.kind == trace.SpanKind.SERVER:
            custom_attributes = _collect_custom_response_headers_attributes(
                handler._headers
            )
            if len(custom_attributes) > 0:
                ctx.span.set_attributes(custom_attributes)

    ctx.activation.__exit__(*finish_args)  # pylint: disable=E1101
    if ctx.token:
        context.detach(ctx.token)
    delattr(handler, _HANDLER_CONTEXT_KEY)


def _record_prepare_metrics(server_histograms, handler):
    request_size = int(handler.request.headers.get("Content-Length", 0))
    metric_attributes = _create_metric_attributes(handler)

    server_histograms[MetricInstruments.HTTP_SERVER_REQUEST_SIZE].record(
        request_size, attributes=metric_attributes
    )

    active_requests_attributes = _create_active_requests_attributes(
        handler.request
    )
    server_histograms[MetricInstruments.HTTP_SERVER_ACTIVE_REQUESTS].add(
        1, attributes=active_requests_attributes
    )


def _record_on_finish_metrics(server_histograms, handler, error=None):
    elapsed_time = round(
        (default_timer() - server_histograms[_START_TIME]) * 1000
    )

    response_size = int(handler._headers.get("Content-Length", 0))
    metric_attributes = _create_metric_attributes(handler)

    if isinstance(error, tornado.web.HTTPError):
        metric_attributes[SpanAttributes.HTTP_STATUS_CODE] = error.status_code

    server_histograms[MetricInstruments.HTTP_SERVER_RESPONSE_SIZE].record(
        response_size, attributes=metric_attributes
    )

    server_histograms[MetricInstruments.HTTP_SERVER_DURATION].record(
        elapsed_time, attributes=metric_attributes
    )

    active_requests_attributes = _create_active_requests_attributes(
        handler.request
    )
    server_histograms[MetricInstruments.HTTP_SERVER_ACTIVE_REQUESTS].add(
        -1, attributes=active_requests_attributes
    )


def _create_active_requests_attributes(request):
    metric_attributes = {
        SpanAttributes.HTTP_METHOD: request.method,
        SpanAttributes.HTTP_SCHEME: request.protocol,
        SpanAttributes.HTTP_FLAVOR: request.version,
        SpanAttributes.HTTP_HOST: request.host,
        SpanAttributes.HTTP_TARGET: request.path,
    }

    return metric_attributes


def _create_metric_attributes(handler):
    metric_attributes = _create_active_requests_attributes(handler.request)
    metric_attributes[SpanAttributes.HTTP_STATUS_CODE] = handler.get_status()

    return metric_attributes
