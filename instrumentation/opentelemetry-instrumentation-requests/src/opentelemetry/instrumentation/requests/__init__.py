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
This library allows tracing HTTP requests made by the
`requests <https://requests.readthedocs.io/en/master/>`_ library.

Usage
-----

.. code-block:: python

    import requests
    from opentelemetry.instrumentation.requests import RequestsInstrumentor

    # You can optionally pass a custom TracerProvider to instrument().
    RequestsInstrumentor().instrument()
    response = requests.get(url="https://www.example.org/")

Configuration
-------------

Exclude lists
*************
To exclude certain URLs from being tracked, set the environment variable ``OTEL_PYTHON_REQUESTS_EXCLUDED_URLS``
(or ``OTEL_PYTHON_EXCLUDED_URLS`` as fallback) with comma delimited regexes representing which URLs to exclude.

For example,

::

    export OTEL_PYTHON_REQUESTS_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.

API
---
"""

import functools
import types
from typing import Collection

from requests.models import Response
from requests.sessions import Session
from requests.structures import CaseInsensitiveDict

from opentelemetry import context
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.requests.package import _instruments
from opentelemetry.instrumentation.requests.version import __version__
from opentelemetry.instrumentation.utils import (
    _SUPPRESS_INSTRUMENTATION_KEY,
    http_status_to_status_code,
)
from opentelemetry.propagate import inject
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import SpanKind, get_tracer
from opentelemetry.trace.status import Status
from opentelemetry.util.http import (
    get_excluded_urls,
    parse_excluded_urls,
    remove_url_credentials,
)
from opentelemetry.util.http.httplib import set_ip_on_next_http_connection

# A key to a context variable to avoid creating duplicate spans when instrumenting
# both, Session.request and Session.send, since Session.request calls into Session.send
_SUPPRESS_HTTP_INSTRUMENTATION_KEY = context.create_key(
    "suppress_http_instrumentation"
)

_excluded_urls_from_env = get_excluded_urls("REQUESTS")


# pylint: disable=unused-argument
# pylint: disable=R0915
def _instrument(
    tracer, span_callback=None, name_callback=None, excluded_urls=None
):
    """Enables tracing of all requests calls that go through
    :code:`requests.session.Session.request` (this includes
    :code:`requests.get`, etc.)."""

    # Since
    # https://github.com/psf/requests/commit/d72d1162142d1bf8b1b5711c664fbbd674f349d1
    # (v0.7.0, Oct 23, 2011), get, post, etc are implemented via request which
    # again, is implemented via Session.request (`Session` was named `session`
    # before v1.0.0, Dec 17, 2012, see
    # https://github.com/psf/requests/commit/4e5c4a6ab7bb0195dececdd19bb8505b872fe120)

    wrapped_request = Session.request
    wrapped_send = Session.send

    @functools.wraps(wrapped_request)
    def instrumented_request(self, method, url, *args, **kwargs):
        if excluded_urls and excluded_urls.url_disabled(url):
            return wrapped_request(self, method, url, *args, **kwargs)

        def get_or_create_headers():
            headers = kwargs.get("headers")
            if headers is None:
                headers = {}
                kwargs["headers"] = headers

            return headers

        def call_wrapped():
            return wrapped_request(self, method, url, *args, **kwargs)

        return _instrumented_requests_call(
            method, url, call_wrapped, get_or_create_headers
        )

    @functools.wraps(wrapped_send)
    def instrumented_send(self, request, **kwargs):
        if excluded_urls and excluded_urls.url_disabled(request.url):
            return wrapped_send(self, request, **kwargs)

        def get_or_create_headers():
            request.headers = (
                request.headers
                if request.headers is not None
                else CaseInsensitiveDict()
            )
            return request.headers

        def call_wrapped():
            return wrapped_send(self, request, **kwargs)

        return _instrumented_requests_call(
            request.method, request.url, call_wrapped, get_or_create_headers
        )

    def _instrumented_requests_call(
        method: str, url: str, call_wrapped, get_or_create_headers
    ):
        if context.get_value(
            _SUPPRESS_INSTRUMENTATION_KEY
        ) or context.get_value(_SUPPRESS_HTTP_INSTRUMENTATION_KEY):
            return call_wrapped()

        # See
        # https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#http-client
        method = method.upper()
        span_name = ""
        if name_callback is not None:
            span_name = name_callback(method, url)
        if not span_name or not isinstance(span_name, str):
            span_name = get_default_span_name(method)

        url = remove_url_credentials(url)

        span_attributes = {
            SpanAttributes.HTTP_METHOD: method,
            SpanAttributes.HTTP_URL: url,
        }

        with tracer.start_as_current_span(
            span_name, kind=SpanKind.CLIENT, attributes=span_attributes
        ) as span, set_ip_on_next_http_connection(span):
            exception = None

            headers = get_or_create_headers()
            inject(headers)

            token = context.attach(
                context.set_value(_SUPPRESS_HTTP_INSTRUMENTATION_KEY, True)
            )
            try:
                result = call_wrapped()  # *** PROCEED
            except Exception as exc:  # pylint: disable=W0703
                exception = exc
                result = getattr(exc, "response", None)
            finally:
                context.detach(token)

            if isinstance(result, Response):
                if span.is_recording():
                    span.set_attribute(
                        SpanAttributes.HTTP_STATUS_CODE, result.status_code
                    )
                    span.set_status(
                        Status(http_status_to_status_code(result.status_code))
                    )
            if span_callback is not None:
                span_callback(span, result)

            if exception is not None:
                raise exception.with_traceback(exception.__traceback__)

        return result

    instrumented_request.opentelemetry_instrumentation_requests_applied = True
    Session.request = instrumented_request

    instrumented_send.opentelemetry_instrumentation_requests_applied = True
    Session.send = instrumented_send


def _uninstrument():
    """Disables instrumentation of :code:`requests` through this module.

    Note that this only works if no other module also patches requests."""
    _uninstrument_from(Session)


def _uninstrument_from(instr_root, restore_as_bound_func=False):
    for instr_func_name in ("request", "send"):
        instr_func = getattr(instr_root, instr_func_name)
        if not getattr(
            instr_func,
            "opentelemetry_instrumentation_requests_applied",
            False,
        ):
            continue

        original = instr_func.__wrapped__  # pylint:disable=no-member
        if restore_as_bound_func:
            original = types.MethodType(original, instr_root)
        setattr(instr_root, instr_func_name, original)


def get_default_span_name(method):
    """Default implementation for name_callback, returns HTTP {method_name}."""
    return f"HTTP {method.strip()}"


class RequestsInstrumentor(BaseInstrumentor):
    """An instrumentor for requests
    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Instruments requests module

        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global
                ``span_callback``: An optional callback invoked before returning the http response. Invoked with Span and requests.Response
                ``name_callback``: Callback which calculates a generic span name for an
                    outgoing HTTP request based on the method and url.
                    Optional: Defaults to get_default_span_name.
                ``excluded_urls``: A string containing a comma-delimited
                    list of regexes used to exclude URLs from tracking
        """
        tracer_provider = kwargs.get("tracer_provider")
        tracer = get_tracer(__name__, __version__, tracer_provider)
        excluded_urls = kwargs.get("excluded_urls")
        _instrument(
            tracer,
            span_callback=kwargs.get("span_callback"),
            name_callback=kwargs.get("name_callback"),
            excluded_urls=_excluded_urls_from_env
            if excluded_urls is None
            else parse_excluded_urls(excluded_urls),
        )

    def _uninstrument(self, **kwargs):
        _uninstrument()

    @staticmethod
    def uninstrument_session(session):
        """Disables instrumentation on the session object."""
        _uninstrument_from(session, restore_as_bound_func=True)
