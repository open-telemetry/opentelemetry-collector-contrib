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
This library provides a WSGI middleware that can be used on any WSGI framework
(such as Django / Flask / Web.py) to track requests timing through OpenTelemetry.

Usage (Flask)
-------------

.. code-block:: python

    from flask import Flask
    from opentelemetry.instrumentation.wsgi import OpenTelemetryMiddleware

    app = Flask(__name__)
    app.wsgi_app = OpenTelemetryMiddleware(app.wsgi_app)

    @app.route("/")
    def hello():
        return "Hello!"

    if __name__ == "__main__":
        app.run(debug=True)


Usage (Django)
--------------

Modify the application's ``wsgi.py`` file as shown below.

.. code-block:: python

    import os
    from opentelemetry.instrumentation.wsgi import OpenTelemetryMiddleware
    from django.core.wsgi import get_wsgi_application

    os.environ.setdefault('DJANGO_SETTINGS_MODULE', 'application.settings')

    application = get_wsgi_application()
    application = OpenTelemetryMiddleware(application)

Usage (Web.py)
--------------

.. code-block:: python

    import web
    from opentelemetry.instrumentation.wsgi import OpenTelemetryMiddleware
    from cheroot import wsgi

    urls = ('/', 'index')


    class index:

        def GET(self):
            return "Hello, world!"


    if __name__ == "__main__":
        app = web.application(urls, globals())
        func = app.wsgifunc()

        func = OpenTelemetryMiddleware(func)

        server = wsgi.WSGIServer(
            ("localhost", 5100), func, server_name="localhost"
        )
        server.start()

Configuration
-------------

Request/Response hooks
**********************

Utilize request/reponse hooks to execute custom logic to be performed before/after performing a request. Environ is an instance of WSGIEnvironment.
Response_headers is a list of key-value (tuples) representing the response headers returned from the response.

.. code-block:: python

    def request_hook(span: Span, environ: WSGIEnvironment):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_request_hook", "some-value")

    def response_hook(span: Span, environ: WSGIEnvironment, status: str, response_headers: List):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_response_hook", "some-value")

    OpenTelemetryMiddleware(request_hook=request_hook, response_hook=response_hook)

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
Request header names in wsgi are case insensitive and - characters are replaced by _. So, giving header name as ``CUStom_Header`` in environment variable will be able capture header with name ``custom-header``.

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
Response header names captured in wsgi are case insensitive. So, giving header name as ``CUStomHeader`` in environment variable will be able capture header with name ``customheader``.

The name of the added span attribute will follow the format ``http.response.header.<header_name>`` where ``<header_name>`` being the normalized HTTP header name (lowercase, with - characters replaced by _ ).
The value of the attribute will be single item list containing all the header values.

Example of the added span attribute,
``http.response.header.custom_response_header = ["<value1>,<value2>"]``

Note:
    Environment variable names to capture http headers are still experimental, and thus are subject to change.

API
---
"""

import functools
import typing
import wsgiref.util as wsgiref_util
from timeit import default_timer

from opentelemetry import context, trace
from opentelemetry.instrumentation.utils import (
    _start_internal_or_server_span,
    http_status_to_status_code,
)
from opentelemetry.instrumentation.wsgi.version import __version__
from opentelemetry.metrics import get_meter
from opentelemetry.propagators.textmap import Getter
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace.status import Status, StatusCode
from opentelemetry.util.http import (
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST,
    OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE,
    get_custom_headers,
    normalise_request_header_name,
    normalise_response_header_name,
    remove_url_credentials,
)

_HTTP_VERSION_PREFIX = "HTTP/"
_CARRIER_KEY_PREFIX = "HTTP_"
_CARRIER_KEY_PREFIX_LEN = len(_CARRIER_KEY_PREFIX)

# List of recommended attributes
_duration_attrs = [
    SpanAttributes.HTTP_METHOD,
    SpanAttributes.HTTP_HOST,
    SpanAttributes.HTTP_SCHEME,
    SpanAttributes.HTTP_STATUS_CODE,
    SpanAttributes.HTTP_FLAVOR,
    SpanAttributes.HTTP_SERVER_NAME,
    SpanAttributes.NET_HOST_NAME,
    SpanAttributes.NET_HOST_PORT,
]

_active_requests_count_attrs = [
    SpanAttributes.HTTP_METHOD,
    SpanAttributes.HTTP_HOST,
    SpanAttributes.HTTP_SCHEME,
    SpanAttributes.HTTP_FLAVOR,
    SpanAttributes.HTTP_SERVER_NAME,
]


class WSGIGetter(Getter[dict]):
    def get(
        self, carrier: dict, key: str
    ) -> typing.Optional[typing.List[str]]:
        """Getter implementation to retrieve a HTTP header value from the
             PEP3333-conforming WSGI environ

        Args:
             carrier: WSGI environ object
             key: header name in environ object
         Returns:
             A list with a single string with the header value if it exists,
             else None.
        """
        environ_key = "HTTP_" + key.upper().replace("-", "_")
        value = carrier.get(environ_key)
        if value is not None:
            return [value]
        return None

    def keys(self, carrier):
        return [
            key[_CARRIER_KEY_PREFIX_LEN:].lower().replace("_", "-")
            for key in carrier
            if key.startswith(_CARRIER_KEY_PREFIX)
        ]


wsgi_getter = WSGIGetter()


def setifnotnone(dic, key, value):
    if value is not None:
        dic[key] = value


def collect_request_attributes(environ):
    """Collects HTTP request attributes from the PEP3333-conforming
    WSGI environ and returns a dictionary to be used as span creation attributes."""

    result = {
        SpanAttributes.HTTP_METHOD: environ.get("REQUEST_METHOD"),
        SpanAttributes.HTTP_SERVER_NAME: environ.get("SERVER_NAME"),
        SpanAttributes.HTTP_SCHEME: environ.get("wsgi.url_scheme"),
    }

    host_port = environ.get("SERVER_PORT")
    if host_port is not None and not host_port == "":
        result.update({SpanAttributes.NET_HOST_PORT: int(host_port)})

    setifnotnone(result, SpanAttributes.HTTP_HOST, environ.get("HTTP_HOST"))
    target = environ.get("RAW_URI")
    if target is None:  # Note: `"" or None is None`
        target = environ.get("REQUEST_URI")
    if target is not None:
        result[SpanAttributes.HTTP_TARGET] = target
    else:
        result[SpanAttributes.HTTP_URL] = remove_url_credentials(
            wsgiref_util.request_uri(environ)
        )

    remote_addr = environ.get("REMOTE_ADDR")
    if remote_addr:
        result[SpanAttributes.NET_PEER_IP] = remote_addr
    remote_host = environ.get("REMOTE_HOST")
    if remote_host and remote_host != remote_addr:
        result[SpanAttributes.NET_PEER_NAME] = remote_host

    user_agent = environ.get("HTTP_USER_AGENT")
    if user_agent is not None and len(user_agent) > 0:
        result[SpanAttributes.HTTP_USER_AGENT] = user_agent

    setifnotnone(
        result, SpanAttributes.NET_PEER_PORT, environ.get("REMOTE_PORT")
    )
    flavor = environ.get("SERVER_PROTOCOL", "")
    if flavor.upper().startswith(_HTTP_VERSION_PREFIX):
        flavor = flavor[len(_HTTP_VERSION_PREFIX) :]
    if flavor:
        result[SpanAttributes.HTTP_FLAVOR] = flavor

    return result


def collect_custom_request_headers_attributes(environ):
    """Returns custom HTTP request headers which are configured by the user
    from the PEP3333-conforming WSGI environ to be used as span creation attributes as described
    in the specification https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#http-request-and-response-headers"""
    attributes = {}
    custom_request_headers_name = get_custom_headers(
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST
    )
    for header_name in custom_request_headers_name:
        wsgi_env_var = header_name.upper().replace("-", "_")
        header_values = environ.get(f"HTTP_{wsgi_env_var}")
        if header_values:
            key = normalise_request_header_name(header_name)
            attributes[key] = [header_values]
    return attributes


def collect_custom_response_headers_attributes(response_headers):
    """Returns custom HTTP response headers which are configured by the user from the
    PEP3333-conforming WSGI environ as described in the specification
    https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#http-request-and-response-headers"""
    attributes = {}
    custom_response_headers_name = get_custom_headers(
        OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE
    )
    response_headers_dict = {}
    if response_headers:
        for header_name, header_value in response_headers:
            response_headers_dict[header_name.lower()] = header_value

    for header_name in custom_response_headers_name:
        header_values = response_headers_dict.get(header_name.lower())
        if header_values:
            key = normalise_response_header_name(header_name)
            attributes[key] = [header_values]
    return attributes


def _parse_status_code(resp_status):
    status_code, _ = resp_status.split(" ", 1)
    try:
        return int(status_code)
    except ValueError:
        return None


def add_response_attributes(
    span, start_response_status, response_headers
):  # pylint: disable=unused-argument
    """Adds HTTP response attributes to span using the arguments
    passed to a PEP3333-conforming start_response callable."""
    if not span.is_recording():
        return
    status_code, _ = start_response_status.split(" ", 1)

    try:
        status_code = int(status_code)
    except ValueError:
        span.set_status(
            Status(
                StatusCode.ERROR,
                "Non-integer HTTP status: " + repr(status_code),
            )
        )
    else:
        span.set_attribute(SpanAttributes.HTTP_STATUS_CODE, status_code)
        span.set_status(
            Status(http_status_to_status_code(status_code, server_span=True))
        )


def get_default_span_name(environ):
    """Default implementation for name_callback, returns HTTP {METHOD_NAME}."""
    return f"HTTP {environ.get('REQUEST_METHOD', '')}".strip()


class OpenTelemetryMiddleware:
    """The WSGI application middleware.

    This class is a PEP 3333 conforming WSGI middleware that starts and
    annotates spans for any requests it is invoked with.

    Args:
        wsgi: The WSGI application callable to forward requests to.
        request_hook: Optional callback which is called with the server span and WSGI
                      environ object for every incoming request.
        response_hook: Optional callback which is called with the server span,
                       WSGI environ, status_code and response_headers for every
                       incoming request.
        tracer_provider: Optional tracer provider to use. If omitted the current
                         globally configured one is used.
    """

    def __init__(
        self,
        wsgi,
        request_hook=None,
        response_hook=None,
        tracer_provider=None,
        meter_provider=None,
    ):
        self.wsgi = wsgi
        self.tracer = trace.get_tracer(__name__, __version__, tracer_provider)
        self.meter = get_meter(__name__, __version__, meter_provider)
        self.duration_histogram = self.meter.create_histogram(
            name="http.server.duration",
            unit="ms",
            description="measures the duration of the inbound HTTP request",
        )
        self.active_requests_counter = self.meter.create_up_down_counter(
            name="http.server.active_requests",
            unit="requests",
            description="measures the number of concurrent HTTP requests that are currently in-flight",
        )
        self.request_hook = request_hook
        self.response_hook = response_hook

    @staticmethod
    def _create_start_response(
        span, start_response, response_hook, duration_attrs
    ):
        @functools.wraps(start_response)
        def _start_response(status, response_headers, *args, **kwargs):
            add_response_attributes(span, status, response_headers)
            status_code = _parse_status_code(status)
            if status_code is not None:
                duration_attrs[SpanAttributes.HTTP_STATUS_CODE] = status_code
            if span.is_recording() and span.kind == trace.SpanKind.SERVER:
                custom_attributes = collect_custom_response_headers_attributes(
                    response_headers
                )
                if len(custom_attributes) > 0:
                    span.set_attributes(custom_attributes)
            if response_hook:
                response_hook(status, response_headers)
            return start_response(status, response_headers, *args, **kwargs)

        return _start_response

    # pylint: disable=too-many-branches
    def __call__(self, environ, start_response):
        """The WSGI application

        Args:
            environ: A WSGI environment.
            start_response: The WSGI start_response callable.
        """
        req_attrs = collect_request_attributes(environ)
        active_requests_count_attrs = {}
        for attr_key in _active_requests_count_attrs:
            if req_attrs.get(attr_key) is not None:
                active_requests_count_attrs[attr_key] = req_attrs[attr_key]

        duration_attrs = {}
        for attr_key in _duration_attrs:
            if req_attrs.get(attr_key) is not None:
                duration_attrs[attr_key] = req_attrs[attr_key]

        span, token = _start_internal_or_server_span(
            tracer=self.tracer,
            span_name=get_default_span_name(environ),
            start_time=None,
            context_carrier=environ,
            context_getter=wsgi_getter,
            attributes=req_attrs,
        )
        if span.is_recording() and span.kind == trace.SpanKind.SERVER:
            custom_attributes = collect_custom_request_headers_attributes(
                environ
            )
            if len(custom_attributes) > 0:
                span.set_attributes(custom_attributes)

        if self.request_hook:
            self.request_hook(span, environ)

        response_hook = self.response_hook
        if response_hook:
            response_hook = functools.partial(response_hook, span, environ)

        start = default_timer()
        self.active_requests_counter.add(1, active_requests_count_attrs)
        try:
            with trace.use_span(span):
                start_response = self._create_start_response(
                    span, start_response, response_hook, duration_attrs
                )
                iterable = self.wsgi(environ, start_response)
                return _end_span_after_iterating(iterable, span, token)
        except Exception as ex:
            if span.is_recording():
                span.set_status(Status(StatusCode.ERROR, str(ex)))
            span.end()
            if token is not None:
                context.detach(token)
            raise
        finally:
            duration = max(round((default_timer() - start) * 1000), 0)
            self.duration_histogram.record(duration, duration_attrs)
            self.active_requests_counter.add(-1, active_requests_count_attrs)


# Put this in a subfunction to not delay the call to the wrapped
# WSGI application (instrumentation should change the application
# behavior as little as possible).
def _end_span_after_iterating(iterable, span, token):
    try:
        with trace.use_span(span):
            yield from iterable
    finally:
        close = getattr(iterable, "close", None)
        if close:
            close()
        span.end()
        if token is not None:
            context.detach(token)


# TODO: inherit from opentelemetry.instrumentation.propagators.Setter


class ResponsePropagationSetter:
    def set(self, carrier, key, value):  # pylint: disable=no-self-use
        carrier.append((key, value))


default_response_propagation_setter = ResponsePropagationSetter()
