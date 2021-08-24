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
This library builds on the OpenTelemetry WSGI middleware to track web requests
in Falcon applications. In addition to opentelemetry-instrumentation-wsgi,
it supports falcon-specific features such as:

* The Falcon resource and method name is used as the Span name.
* The ``falcon.resource`` Span attribute is set so the matched resource.
* Error from Falcon resources are properly caught and recorded.

Configuration
-------------

Exclude lists
*************
To exclude certain URLs from being tracked, set the environment variable ``OTEL_PYTHON_FALCON_EXCLUDED_URLS`` with comma delimited regexes representing which URLs to exclude.

For example,

::

    export OTEL_PYTHON_FALCON_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.

Request attributes
********************
To extract certain attributes from Falcon's request object and use them as span attributes, set the environment variable ``OTEL_PYTHON_FALCON_TRACED_REQUEST_ATTRS`` to a comma
delimited list of request attribute names.

For example,

::

    export OTEL_PYTHON_FALCON_TRACED_REQUEST_ATTRS='query_string,uri_template'

will extract query_string and uri_template attributes from every traced request and add them as span attritbues.

Falcon Request object reference: https://falcon.readthedocs.io/en/stable/api/request_and_response.html#id1

Usage
-----

.. code-block:: python

    from falcon import API
    from opentelemetry.instrumentation.falcon import FalconInstrumentor

    FalconInstrumentor().instrument()

    app = falcon.API()

    class HelloWorldResource(object):
        def on_get(self, req, resp):
            resp.body = 'Hello World'

    app.add_route('/hello', HelloWorldResource())


Request and Response hooks
***************************
The instrumentation supports specifying request and response hooks. These are functions that get called back by the instrumentation right after a Span is created for a request
and right before the span is finished while processing a response. The hooks can be configured as follows:

::

    def request_hook(span, req):
        pass

    def response_hook(span, req, resp):
        pass

    FalconInstrumentation().instrument(request_hook=request_hook, response_hook=response_hook)

API
---
"""

from functools import partial
from logging import getLogger
from sys import exc_info
from typing import Collection

import falcon

import opentelemetry.instrumentation.wsgi as otel_wsgi
from opentelemetry import context, trace
from opentelemetry.instrumentation.falcon.package import _instruments
from opentelemetry.instrumentation.falcon.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.propagators import (
    FuncSetter,
    get_global_response_propagator,
)
from opentelemetry.instrumentation.utils import (
    extract_attributes_from_object,
    http_status_to_status_code,
)
from opentelemetry.propagate import extract
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace.status import Status
from opentelemetry.util._time import _time_ns
from opentelemetry.util.http import get_excluded_urls, get_traced_request_attrs

_logger = getLogger(__name__)

_ENVIRON_STARTTIME_KEY = "opentelemetry-falcon.starttime_key"
_ENVIRON_SPAN_KEY = "opentelemetry-falcon.span_key"
_ENVIRON_ACTIVATION_KEY = "opentelemetry-falcon.activation_key"
_ENVIRON_TOKEN = "opentelemetry-falcon.token"
_ENVIRON_EXC = "opentelemetry-falcon.exc"


_excluded_urls = get_excluded_urls("FALCON")
_traced_request_attrs = get_traced_request_attrs("FALCON")
_response_propagation_setter = FuncSetter(falcon.Response.append_header)


class FalconInstrumentor(BaseInstrumentor):
    # pylint: disable=protected-access,attribute-defined-outside-init
    """An instrumentor for falcon.API

    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        self._original_falcon_api = falcon.API
        falcon.API = partial(_InstrumentedFalconAPI, **kwargs)

    def _uninstrument(self, **kwargs):
        falcon.API = self._original_falcon_api


class _InstrumentedFalconAPI(falcon.API):
    def __init__(self, *args, **kwargs):
        # inject trace middleware
        middlewares = kwargs.pop("middleware", [])
        tracer_provider = kwargs.pop("tracer_provider", None)
        if not isinstance(middlewares, (list, tuple)):
            middlewares = [middlewares]

        self._tracer = trace.get_tracer(__name__, __version__, tracer_provider)

        trace_middleware = _TraceMiddleware(
            self._tracer,
            kwargs.pop("traced_request_attributes", _traced_request_attrs),
            kwargs.pop("request_hook", None),
            kwargs.pop("response_hook", None),
        )
        middlewares.insert(0, trace_middleware)
        kwargs["middleware"] = middlewares
        super().__init__(*args, **kwargs)

    def __call__(self, env, start_response):
        # pylint: disable=E1101
        if _excluded_urls.url_disabled(env.get("PATH_INFO", "/")):
            return super().__call__(env, start_response)

        start_time = _time_ns()

        token = context.attach(extract(env, getter=otel_wsgi.wsgi_getter))
        span = self._tracer.start_span(
            otel_wsgi.get_default_span_name(env),
            kind=trace.SpanKind.SERVER,
            start_time=start_time,
        )
        if span.is_recording():
            attributes = otel_wsgi.collect_request_attributes(env)
            for key, value in attributes.items():
                span.set_attribute(key, value)

        activation = trace.use_span(span, end_on_exit=True)
        activation.__enter__()
        env[_ENVIRON_SPAN_KEY] = span
        env[_ENVIRON_ACTIVATION_KEY] = activation

        def _start_response(status, response_headers, *args, **kwargs):
            otel_wsgi.add_response_attributes(span, status, response_headers)
            response = start_response(
                status, response_headers, *args, **kwargs
            )
            activation.__exit__(None, None, None)
            context.detach(token)
            return response

        try:
            return super().__call__(env, _start_response)
        except Exception as exc:
            activation.__exit__(
                type(exc), exc, getattr(exc, "__traceback__", None),
            )
            context.detach(token)
            raise


class _TraceMiddleware:
    # pylint:disable=R0201,W0613

    def __init__(
        self,
        tracer=None,
        traced_request_attrs=None,
        request_hook=None,
        response_hook=None,
    ):
        self.tracer = tracer
        self._traced_request_attrs = traced_request_attrs
        self._request_hook = request_hook
        self._response_hook = response_hook

    def process_request(self, req, resp):
        span = req.env.get(_ENVIRON_SPAN_KEY)
        if span and self._request_hook:
            self._request_hook(span, req)

        if not span or not span.is_recording():
            return

        attributes = extract_attributes_from_object(
            req, self._traced_request_attrs
        )
        for key, value in attributes.items():
            span.set_attribute(key, value)

    def process_resource(self, req, resp, resource, params):
        span = req.env.get(_ENVIRON_SPAN_KEY)
        if not span or not span.is_recording():
            return

        resource_name = resource.__class__.__name__
        span.set_attribute("falcon.resource", resource_name)
        span.update_name(
            "{0}.on_{1}".format(resource_name, req.method.lower())
        )

    def process_response(
        self, req, resp, resource, req_succeeded=None
    ):  # pylint:disable=R0201
        span = req.env.get(_ENVIRON_SPAN_KEY)

        if not span or not span.is_recording():
            return

        status = resp.status
        reason = None
        if resource is None:
            status = "404"
            reason = "NotFound"

        exc_type, exc, _ = exc_info()
        if exc_type and not req_succeeded:
            if "HTTPNotFound" in exc_type.__name__:
                status = "404"
                reason = "NotFound"
            else:
                status = "500"
                reason = "{}: {}".format(exc_type.__name__, exc)

        status = status.split(" ")[0]
        try:
            status_code = int(status)
        except ValueError:
            pass
        finally:
            span.set_attribute(SpanAttributes.HTTP_STATUS_CODE, status_code)
            span.set_status(
                Status(
                    status_code=http_status_to_status_code(status_code),
                    description=reason,
                )
            )

        propagator = get_global_response_propagator()
        if propagator:
            propagator.inject(resp, setter=_response_propagation_setter)

        if self._response_hook:
            self._response_hook(span, req, resp)
