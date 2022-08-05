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

# Note: This package is not named "flask" because of
# https://github.com/PyCQA/pylint/issues/2648

"""
This library builds on the OpenTelemetry WSGI middleware to track web requests
in Flask applications. In addition to opentelemetry-util-http, it
supports Flask-specific features such as:

* The Flask url rule pattern is used as the Span name.
* The ``http.route`` Span attribute is set so that one can see which URL rule
  matched a request.

Usage
-----

.. code-block:: python

    from flask import Flask
    from opentelemetry.instrumentation.flask import FlaskInstrumentor

    app = Flask(__name__)

    FlaskInstrumentor().instrument_app(app)

    @app.route("/")
    def hello():
        return "Hello!"

    if __name__ == "__main__":
        app.run(debug=True)

Configuration
-------------

Exclude lists
*************
To exclude certain URLs from being tracked, set the environment variable ``OTEL_PYTHON_FLASK_EXCLUDED_URLS``
(or ``OTEL_PYTHON_EXCLUDED_URLS`` as fallback) with comma delimited regexes representing which URLs to exclude.

For example,

::

    export OTEL_PYTHON_FLASK_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.

You can also pass the comma delimited regexes to the ``instrument_app`` method directly:

.. code-block:: python

    FlaskInstrumentor().instrument_app(app, excluded_urls="client/.*/info,healthcheck")

Request/Response hooks
**********************

Utilize request/reponse hooks to execute custom logic to be performed before/after performing a request. Environ is an instance of WSGIEnvironment (flask.request.environ).
Response_headers is a list of key-value (tuples) representing the response headers returned from the response.

.. code-block:: python

    def request_hook(span: Span, environ: WSGIEnvironment):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_request_hook", "some-value")

    def response_hook(span: Span, status: str, response_headers: List):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_response_hook", "some-value")

    FlaskInstrumentation().instrument(request_hook=request_hook, response_hook=response_hook)

Flask Request object reference: https://flask.palletsprojects.com/en/2.0.x/api/#flask.Request

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
Request header names in flask are case insensitive and - characters are replaced by _. So, giving header name as ``CUStom_Header`` in environment variable will be able capture header with name ``custom-header``.

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
Response header names captured in flask are case insensitive. So, giving header name as ``CUStomHeader`` in environment variable will be able capture header with name ``customheader``.

The name of the added span attribute will follow the format ``http.response.header.<header_name>`` where ``<header_name>`` being the normalized HTTP header name (lowercase, with - characters replaced by _ ).
The value of the attribute will be single item list containing all the header values.

Example of the added span attribute,
``http.response.header.custom_response_header = ["<value1>,<value2>"]``

Note:
    Environment variable names to capture http headers are still experimental, and thus are subject to change.

API
---
"""

from logging import getLogger
from timeit import default_timer
from typing import Collection

import flask

import opentelemetry.instrumentation.wsgi as otel_wsgi
from opentelemetry import context, trace
from opentelemetry.instrumentation.flask.package import _instruments
from opentelemetry.instrumentation.flask.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.propagators import (
    get_global_response_propagator,
)
from opentelemetry.instrumentation.utils import _start_internal_or_server_span
from opentelemetry.metrics import get_meter
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.util._time import _time_ns
from opentelemetry.util.http import get_excluded_urls, parse_excluded_urls

_logger = getLogger(__name__)

_ENVIRON_STARTTIME_KEY = "opentelemetry-flask.starttime_key"
_ENVIRON_SPAN_KEY = "opentelemetry-flask.span_key"
_ENVIRON_ACTIVATION_KEY = "opentelemetry-flask.activation_key"
_ENVIRON_TOKEN = "opentelemetry-flask.token"

_excluded_urls_from_env = get_excluded_urls("FLASK")


def get_default_span_name():
    span_name = ""
    try:
        span_name = flask.request.url_rule.rule
    except AttributeError:
        span_name = otel_wsgi.get_default_span_name(flask.request.environ)
    return span_name


def _rewrapped_app(
    wsgi_app,
    active_requests_counter,
    duration_histogram,
    response_hook=None,
    excluded_urls=None,
):
    def _wrapped_app(wrapped_app_environ, start_response):
        # We want to measure the time for route matching, etc.
        # In theory, we could start the span here and use
        # update_name later but that API is "highly discouraged" so
        # we better avoid it.
        wrapped_app_environ[_ENVIRON_STARTTIME_KEY] = _time_ns()
        start = default_timer()
        attributes = otel_wsgi.collect_request_attributes(wrapped_app_environ)
        active_requests_count_attrs = (
            otel_wsgi._parse_active_request_count_attrs(attributes)
        )
        duration_attrs = otel_wsgi._parse_duration_attrs(attributes)
        active_requests_counter.add(1, active_requests_count_attrs)

        def _start_response(status, response_headers, *args, **kwargs):
            if flask.request and (
                excluded_urls is None
                or not excluded_urls.url_disabled(flask.request.url)
            ):
                span = flask.request.environ.get(_ENVIRON_SPAN_KEY)

                propagator = get_global_response_propagator()
                if propagator:
                    propagator.inject(
                        response_headers,
                        setter=otel_wsgi.default_response_propagation_setter,
                    )

                if span:
                    otel_wsgi.add_response_attributes(
                        span, status, response_headers
                    )
                    status_code = otel_wsgi._parse_status_code(status)
                    if status_code is not None:
                        duration_attrs[
                            SpanAttributes.HTTP_STATUS_CODE
                        ] = status_code
                    if (
                        span.is_recording()
                        and span.kind == trace.SpanKind.SERVER
                    ):
                        custom_attributes = otel_wsgi.collect_custom_response_headers_attributes(
                            response_headers
                        )
                        if len(custom_attributes) > 0:
                            span.set_attributes(custom_attributes)
                else:
                    _logger.warning(
                        "Flask environ's OpenTelemetry span "
                        "missing at _start_response(%s)",
                        status,
                    )
                if response_hook is not None:
                    response_hook(span, status, response_headers)
            return start_response(status, response_headers, *args, **kwargs)

        result = wsgi_app(wrapped_app_environ, _start_response)
        duration = max(round((default_timer() - start) * 1000), 0)
        duration_histogram.record(duration, duration_attrs)
        active_requests_counter.add(-1, active_requests_count_attrs)
        return result

    return _wrapped_app


def _wrapped_before_request(
    request_hook=None,
    tracer=None,
    excluded_urls=None,
):
    def _before_request():
        if excluded_urls and excluded_urls.url_disabled(flask.request.url):
            return
        flask_request_environ = flask.request.environ
        span_name = get_default_span_name()

        span, token = _start_internal_or_server_span(
            tracer=tracer,
            span_name=span_name,
            start_time=flask_request_environ.get(_ENVIRON_STARTTIME_KEY),
            context_carrier=flask_request_environ,
            context_getter=otel_wsgi.wsgi_getter,
        )

        if request_hook:
            request_hook(span, flask_request_environ)

        if span.is_recording():
            attributes = otel_wsgi.collect_request_attributes(
                flask_request_environ
            )
            if flask.request.url_rule:
                # For 404 that result from no route found, etc, we
                # don't have a url_rule.
                attributes[
                    SpanAttributes.HTTP_ROUTE
                ] = flask.request.url_rule.rule
            for key, value in attributes.items():
                span.set_attribute(key, value)
            if span.is_recording() and span.kind == trace.SpanKind.SERVER:
                custom_attributes = (
                    otel_wsgi.collect_custom_request_headers_attributes(
                        flask_request_environ
                    )
                )
                if len(custom_attributes) > 0:
                    span.set_attributes(custom_attributes)

        activation = trace.use_span(span, end_on_exit=True)
        activation.__enter__()  # pylint: disable=E1101
        flask_request_environ[_ENVIRON_ACTIVATION_KEY] = activation
        flask_request_environ[_ENVIRON_SPAN_KEY] = span
        flask_request_environ[_ENVIRON_TOKEN] = token

    return _before_request


def _wrapped_teardown_request(
    excluded_urls=None,
):
    def _teardown_request(exc):
        # pylint: disable=E1101
        if excluded_urls and excluded_urls.url_disabled(flask.request.url):
            return

        activation = flask.request.environ.get(_ENVIRON_ACTIVATION_KEY)
        if not activation:
            # This request didn't start a span, maybe because it was created in
            # a way that doesn't run `before_request`, like when it is created
            # with `app.test_request_context`.
            return
        if exc is None:
            activation.__exit__(None, None, None)
        else:
            activation.__exit__(
                type(exc), exc, getattr(exc, "__traceback__", None)
            )

        if flask.request.environ.get(_ENVIRON_TOKEN, None):
            context.detach(flask.request.environ.get(_ENVIRON_TOKEN))

    return _teardown_request


class _InstrumentedFlask(flask.Flask):

    _excluded_urls = None
    _tracer_provider = None
    _request_hook = None
    _response_hook = None
    _meter_provider = None

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        self._original_wsgi_app = self.wsgi_app
        self._is_instrumented_by_opentelemetry = True

        meter = get_meter(
            __name__, __version__, _InstrumentedFlask._meter_provider
        )
        duration_histogram = meter.create_histogram(
            name="http.server.duration",
            unit="ms",
            description="measures the duration of the inbound HTTP request",
        )
        active_requests_counter = meter.create_up_down_counter(
            name="http.server.active_requests",
            unit="requests",
            description="measures the number of concurrent HTTP requests that are currently in-flight",
        )

        self.wsgi_app = _rewrapped_app(
            self.wsgi_app,
            active_requests_counter,
            duration_histogram,
            _InstrumentedFlask._response_hook,
            excluded_urls=_InstrumentedFlask._excluded_urls,
        )

        tracer = trace.get_tracer(
            __name__, __version__, _InstrumentedFlask._tracer_provider
        )

        _before_request = _wrapped_before_request(
            _InstrumentedFlask._request_hook,
            tracer,
            excluded_urls=_InstrumentedFlask._excluded_urls,
        )
        self._before_request = _before_request
        self.before_request(_before_request)

        _teardown_request = _wrapped_teardown_request(
            excluded_urls=_InstrumentedFlask._excluded_urls,
        )
        self.teardown_request(_teardown_request)


class FlaskInstrumentor(BaseInstrumentor):
    # pylint: disable=protected-access,attribute-defined-outside-init
    """An instrumentor for flask.Flask

    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        self._original_flask = flask.Flask
        request_hook = kwargs.get("request_hook")
        response_hook = kwargs.get("response_hook")
        if callable(request_hook):
            _InstrumentedFlask._request_hook = request_hook
        if callable(response_hook):
            _InstrumentedFlask._response_hook = response_hook
        tracer_provider = kwargs.get("tracer_provider")
        _InstrumentedFlask._tracer_provider = tracer_provider
        excluded_urls = kwargs.get("excluded_urls")
        _InstrumentedFlask._excluded_urls = (
            _excluded_urls_from_env
            if excluded_urls is None
            else parse_excluded_urls(excluded_urls)
        )
        meter_provider = kwargs.get("meter_provider")
        _InstrumentedFlask._meter_provider = meter_provider
        flask.Flask = _InstrumentedFlask

    def _uninstrument(self, **kwargs):
        flask.Flask = self._original_flask

    @staticmethod
    def instrument_app(
        app,
        request_hook=None,
        response_hook=None,
        tracer_provider=None,
        excluded_urls=None,
        meter_provider=None,
    ):
        if not hasattr(app, "_is_instrumented_by_opentelemetry"):
            app._is_instrumented_by_opentelemetry = False

        if not app._is_instrumented_by_opentelemetry:
            excluded_urls = (
                parse_excluded_urls(excluded_urls)
                if excluded_urls is not None
                else _excluded_urls_from_env
            )
            meter = get_meter(__name__, __version__, meter_provider)
            duration_histogram = meter.create_histogram(
                name="http.server.duration",
                unit="ms",
                description="measures the duration of the inbound HTTP request",
            )
            active_requests_counter = meter.create_up_down_counter(
                name="http.server.active_requests",
                unit="requests",
                description="measures the number of concurrent HTTP requests that are currently in-flight",
            )

            app._original_wsgi_app = app.wsgi_app
            app.wsgi_app = _rewrapped_app(
                app.wsgi_app,
                active_requests_counter,
                duration_histogram,
                response_hook,
                excluded_urls=excluded_urls,
            )

            tracer = trace.get_tracer(__name__, __version__, tracer_provider)

            _before_request = _wrapped_before_request(
                request_hook,
                tracer,
                excluded_urls=excluded_urls,
            )
            app._before_request = _before_request
            app.before_request(_before_request)

            _teardown_request = _wrapped_teardown_request(
                excluded_urls=excluded_urls,
            )
            app._teardown_request = _teardown_request
            app.teardown_request(_teardown_request)
            app._is_instrumented_by_opentelemetry = True
        else:
            _logger.warning(
                "Attempting to instrument Flask app while already instrumented"
            )

    @staticmethod
    def uninstrument_app(app):
        if hasattr(app, "_original_wsgi_app"):
            app.wsgi_app = app._original_wsgi_app

            # FIXME add support for other Flask blueprints that are not None
            app.before_request_funcs[None].remove(app._before_request)
            app.teardown_request_funcs[None].remove(app._teardown_request)
            del app._original_wsgi_app
            app._is_instrumented_by_opentelemetry = False
        else:
            _logger.warning(
                "Attempting to uninstrument Flask "
                "app while already uninstrumented"
            )
