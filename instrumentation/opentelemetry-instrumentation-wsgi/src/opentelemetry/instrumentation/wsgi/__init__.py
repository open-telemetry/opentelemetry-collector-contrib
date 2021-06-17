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

API
---
"""

import functools
import typing
import wsgiref.util as wsgiref_util

from opentelemetry import context, trace
from opentelemetry.instrumentation.utils import http_status_to_status_code
from opentelemetry.instrumentation.wsgi.version import __version__
from opentelemetry.propagate import extract
from opentelemetry.propagators.textmap import Getter
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace.status import Status, StatusCode
from opentelemetry.util.http import remove_url_credentials

_HTTP_VERSION_PREFIX = "HTTP/"
_CARRIER_KEY_PREFIX = "HTTP_"
_CARRIER_KEY_PREFIX_LEN = len(_CARRIER_KEY_PREFIX)


class WSGIGetter(Getter):
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
        span.set_status(Status(http_status_to_status_code(status_code)))


def get_default_span_name(environ):
    """Default implementation for name_callback, returns HTTP {METHOD_NAME}."""
    return "HTTP {}".format(environ.get("REQUEST_METHOD", "")).strip()


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
        self, wsgi, request_hook=None, response_hook=None, tracer_provider=None
    ):
        self.wsgi = wsgi
        self.tracer = trace.get_tracer(__name__, __version__, tracer_provider)
        self.request_hook = request_hook
        self.response_hook = response_hook

    @staticmethod
    def _create_start_response(span, start_response, response_hook):
        @functools.wraps(start_response)
        def _start_response(status, response_headers, *args, **kwargs):
            add_response_attributes(span, status, response_headers)
            if response_hook:
                response_hook(status, response_headers)
            return start_response(status, response_headers, *args, **kwargs)

        return _start_response

    def __call__(self, environ, start_response):
        """The WSGI application

        Args:
            environ: A WSGI environment.
            start_response: The WSGI start_response callable.
        """

        token = context.attach(extract(environ, getter=wsgi_getter))

        span = self.tracer.start_span(
            get_default_span_name(environ),
            kind=trace.SpanKind.SERVER,
            attributes=collect_request_attributes(environ),
        )

        if self.request_hook:
            self.request_hook(span, environ)

        response_hook = self.response_hook
        if response_hook:
            response_hook = functools.partial(response_hook, span, environ)

        try:
            with trace.use_span(span):
                start_response = self._create_start_response(
                    span, start_response, response_hook
                )
                iterable = self.wsgi(environ, start_response)
                return _end_span_after_iterating(
                    iterable, span, self.tracer, token
                )
        except Exception as ex:
            if span.is_recording():
                span.set_status(Status(StatusCode.ERROR, str(ex)))
            span.end()
            context.detach(token)
            raise


# Put this in a subfunction to not delay the call to the wrapped
# WSGI application (instrumentation should change the application
# behavior as little as possible).
def _end_span_after_iterating(iterable, span, tracer, token):
    try:
        with trace.use_span(span):
            for yielded in iterable:
                yield yielded
    finally:
        close = getattr(iterable, "close", None)
        if close:
            close()
        span.end()
        context.detach(token)


# TODO: inherit from opentelemetry.instrumentation.propagators.Setter


class ResponsePropagationSetter:
    def set(self, carrier, key, value):  # pylint: disable=no-self-use
        carrier.append((key, value))


default_response_propagation_setter = ResponsePropagationSetter()
