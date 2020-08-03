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
    import opentelemetry.instrumentation.requests

    # You can optionally pass a custom TracerProvider to
    RequestInstrumentor.instrument()
    opentelemetry.instrumentation.requests.RequestsInstrumentor().instrument()
    response = requests.get(url="https://www.example.org/")

Limitations
-----------

Note that calls that do not use the higher-level APIs but use
:code:`requests.sessions.Session.send` (or an alias thereof) directly, are
currently not traced. If you find any other way to trigger an untraced HTTP
request, please report it via a GitHub issue with :code:`[requests: untraced
API]` in the title.

API
---
"""

import functools
import types
from urllib.parse import urlparse

from requests import Timeout, URLRequired
from requests.exceptions import InvalidSchema, InvalidURL, MissingSchema
from requests.sessions import Session

from opentelemetry import context, propagators
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.requests.version import __version__
from opentelemetry.instrumentation.utils import http_status_to_canonical_code
from opentelemetry.trace import SpanKind, get_tracer
from opentelemetry.trace.status import Status, StatusCanonicalCode


# pylint: disable=unused-argument
def _instrument(tracer_provider=None, span_callback=None):
    """Enables tracing of all requests calls that go through
      :code:`requests.session.Session.request` (this includes
      :code:`requests.get`, etc.)."""

    # Since
    # https://github.com/psf/requests/commit/d72d1162142d1bf8b1b5711c664fbbd674f349d1
    # (v0.7.0, Oct 23, 2011), get, post, etc are implemented via request which
    # again, is implemented via Session.request (`Session` was named `session`
    # before v1.0.0, Dec 17, 2012, see
    # https://github.com/psf/requests/commit/4e5c4a6ab7bb0195dececdd19bb8505b872fe120)

    wrapped = Session.request

    @functools.wraps(wrapped)
    def instrumented_request(self, method, url, *args, **kwargs):
        if context.get_value("suppress_instrumentation"):
            return wrapped(self, method, url, *args, **kwargs)

        # See
        # https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md#http-client
        try:
            parsed_url = urlparse(url)
            span_name = parsed_url.path
        except ValueError as exc:  # Invalid URL
            span_name = "<Unparsable URL: {}>".format(exc)

        exception = None

        with get_tracer(
            __name__, __version__, tracer_provider
        ).start_as_current_span(span_name, kind=SpanKind.CLIENT) as span:
            span.set_attribute("component", "http")
            span.set_attribute("http.method", method.upper())
            span.set_attribute("http.url", url)

            headers = kwargs.get("headers", {}) or {}
            propagators.inject(type(headers).__setitem__, headers)
            kwargs["headers"] = headers

            try:
                result = wrapped(
                    self, method, url, *args, **kwargs
                )  # *** PROCEED
            except Exception as exc:  # pylint: disable=W0703
                exception = exc
                result = getattr(exc, "response", None)

            if exception is not None:
                span.set_status(
                    Status(_exception_to_canonical_code(exception))
                )

            if result is not None:
                span.set_attribute("http.status_code", result.status_code)
                span.set_attribute("http.status_text", result.reason)
                span.set_status(
                    Status(http_status_to_canonical_code(result.status_code))
                )

            if span_callback is not None:
                span_callback(span, result)

        if exception is not None:
            raise exception.with_traceback(exception.__traceback__)

        return result

    instrumented_request.opentelemetry_ext_requests_applied = True

    Session.request = instrumented_request

    # TODO: We should also instrument requests.sessions.Session.send
    # but to avoid doubled spans, we would need some context-local
    # state (i.e., only create a Span if the current context's URL is
    # different, then push the current URL, pop it afterwards)


def _uninstrument():
    # pylint: disable=global-statement
    """Disables instrumentation of :code:`requests` through this module.

    Note that this only works if no other module also patches requests."""
    if getattr(Session.request, "opentelemetry_ext_requests_applied", False):
        original = Session.request.__wrapped__  # pylint:disable=no-member
        Session.request = original


def _exception_to_canonical_code(exc: Exception) -> StatusCanonicalCode:
    if isinstance(
        exc,
        (InvalidURL, InvalidSchema, MissingSchema, URLRequired, ValueError),
    ):
        return StatusCanonicalCode.INVALID_ARGUMENT
    if isinstance(exc, Timeout):
        return StatusCanonicalCode.DEADLINE_EXCEEDED
    return StatusCanonicalCode.UNKNOWN


class RequestsInstrumentor(BaseInstrumentor):
    """An instrumentor for requests
    See `BaseInstrumentor`
    """

    def _instrument(self, **kwargs):
        """Instruments requests module

        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global
                ``span_callback``: An optional callback invoked before returning the http response. Invoked with Span and requests.Response
        """
        _instrument(
            tracer_provider=kwargs.get("tracer_provider"),
            span_callback=kwargs.get("span_callback"),
        )

    def _uninstrument(self, **kwargs):
        _uninstrument()

    @staticmethod
    def uninstrument_session(session):
        """Disables instrumentation on the session object."""
        if getattr(
            session.request, "opentelemetry_ext_requests_applied", False
        ):
            original = session.request.__wrapped__  # pylint:disable=no-member
            session.request = types.MethodType(original, session)
