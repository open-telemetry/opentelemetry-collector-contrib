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
(such as Django / Flask) to track requests timing through OpenTelemetry.

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

API
---
"""

import functools
import typing
import wsgiref.util as wsgiref_util

from opentelemetry import context, propagators, trace
from opentelemetry.instrumentation.utils import http_status_to_status_code
from opentelemetry.instrumentation.wsgi.version import __version__
from opentelemetry.trace.propagation.textmap import DictGetter
from opentelemetry.trace.status import Status, StatusCode

_HTTP_VERSION_PREFIX = "HTTP/"


class CarrierGetter(DictGetter):
    def get(self, carrier: dict, key: str) -> typing.List[str]:
        """Getter implementation to retrieve a HTTP header value from the
            PEP3333-conforming WSGI environ

        Args:
            carrier: WSGI environ object
            key: header name in environ object
        Returns:
            A list with a single string with the header value if it exists,
            else an empty list.
        """
        environ_key = "HTTP_" + key.upper().replace("-", "_")
        value = carrier.get(environ_key)
        if value is not None:
            return [value]
        return []

    def keys(self, carrier):
        return []


carrier_getter = CarrierGetter()


def setifnotnone(dic, key, value):
    if value is not None:
        dic[key] = value


def collect_request_attributes(environ):
    """Collects HTTP request attributes from the PEP3333-conforming
    WSGI environ and returns a dictionary to be used as span creation attributes."""

    result = {
        "component": "http",
        "http.method": environ.get("REQUEST_METHOD"),
        "http.server_name": environ.get("SERVER_NAME"),
        "http.scheme": environ.get("wsgi.url_scheme"),
    }

    host_port = environ.get("SERVER_PORT")
    if host_port is not None:
        result.update({"host.port": int(host_port)})

    setifnotnone(result, "http.host", environ.get("HTTP_HOST"))
    target = environ.get("RAW_URI")
    if target is None:  # Note: `"" or None is None`
        target = environ.get("REQUEST_URI")
    if target is not None:
        result["http.target"] = target
    else:
        result["http.url"] = wsgiref_util.request_uri(environ)

    remote_addr = environ.get("REMOTE_ADDR")
    if remote_addr:
        result["net.peer.ip"] = remote_addr
    remote_host = environ.get("REMOTE_HOST")
    if remote_host and remote_host != remote_addr:
        result["net.peer.name"] = remote_host

    user_agent = environ.get("HTTP_USER_AGENT")
    if user_agent is not None and len(user_agent) > 0:
        result["http.user_agent"] = user_agent

    setifnotnone(result, "net.peer.port", environ.get("REMOTE_PORT"))
    flavor = environ.get("SERVER_PROTOCOL", "")
    if flavor.upper().startswith(_HTTP_VERSION_PREFIX):
        flavor = flavor[len(_HTTP_VERSION_PREFIX) :]
    if flavor:
        result["http.flavor"] = flavor

    return result


def add_response_attributes(
    span, start_response_status, response_headers
):  # pylint: disable=unused-argument
    """Adds HTTP response attributes to span using the arguments
    passed to a PEP3333-conforming start_response callable."""
    if not span.is_recording():
        return
    status_code, status_text = start_response_status.split(" ", 1)
    span.set_attribute("http.status_text", status_text)

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
        span.set_attribute("http.status_code", status_code)
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
        name_callback: Callback which calculates a generic span name for an
                       incoming HTTP request based on the PEP3333 WSGI environ.
                       Optional: Defaults to get_default_span_name.
    """

    def __init__(self, wsgi, name_callback=get_default_span_name):
        self.wsgi = wsgi
        self.tracer = trace.get_tracer(__name__, __version__)
        self.name_callback = name_callback

    @staticmethod
    def _create_start_response(span, start_response):
        @functools.wraps(start_response)
        def _start_response(status, response_headers, *args, **kwargs):
            add_response_attributes(span, status, response_headers)
            return start_response(status, response_headers, *args, **kwargs)

        return _start_response

    def __call__(self, environ, start_response):
        """The WSGI application

        Args:
            environ: A WSGI environment.
            start_response: The WSGI start_response callable.
        """

        token = context.attach(propagators.extract(carrier_getter, environ))
        span_name = self.name_callback(environ)

        span = self.tracer.start_span(
            span_name,
            kind=trace.SpanKind.SERVER,
            attributes=collect_request_attributes(environ),
        )

        try:
            with self.tracer.use_span(span):
                start_response = self._create_start_response(
                    span, start_response
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
        with tracer.use_span(span):
            for yielded in iterable:
                yield yielded
    finally:
        close = getattr(iterable, "close", None)
        if close:
            close()
        span.end()
        context.detach(token)
