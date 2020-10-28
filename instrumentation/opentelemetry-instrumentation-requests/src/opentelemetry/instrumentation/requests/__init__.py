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
    # RequestInstrumentor.instrument()
    opentelemetry.instrumentation.requests.RequestsInstrumentor().instrument()
    response = requests.get(url="https://www.example.org/")

API
---
"""

import functools
import types

from requests import Timeout, URLRequired
from requests.exceptions import InvalidSchema, InvalidURL, MissingSchema
from requests.sessions import Session
from requests.structures import CaseInsensitiveDict

from opentelemetry import context, propagators
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.metric import (
    HTTPMetricRecorder,
    HTTPMetricType,
    MetricMixin,
)
from opentelemetry.instrumentation.requests.version import __version__
from opentelemetry.instrumentation.utils import http_status_to_status_code
from opentelemetry.trace import SpanKind, get_tracer
from opentelemetry.trace.status import (
    EXCEPTION_STATUS_FIELD,
    Status,
    StatusCode,
)

# A key to a context variable to avoid creating duplicate spans when instrumenting
# both, Session.request and Session.send, since Session.request calls into Session.send
_SUPPRESS_REQUESTS_INSTRUMENTATION_KEY = "suppress_requests_instrumentation"


# pylint: disable=unused-argument
# pylint: disable=R0915
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

    wrapped_request = Session.request
    wrapped_send = Session.send

    @functools.wraps(wrapped_request)
    def instrumented_request(self, method, url, *args, **kwargs):
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
        if context.get_value("suppress_instrumentation") or context.get_value(
            _SUPPRESS_REQUESTS_INSTRUMENTATION_KEY
        ):
            return call_wrapped()

        # See
        # https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md#http-client
        method = method.upper()
        span_name = "HTTP {}".format(method)

        recorder = RequestsInstrumentor().metric_recorder

        labels = {}
        labels["http.method"] = method
        labels["http.url"] = url

        with get_tracer(
            __name__, __version__, tracer_provider
        ).start_as_current_span(span_name, kind=SpanKind.CLIENT) as span:
            exception = None
            with recorder.record_client_duration(labels):
                if span.is_recording():
                    span.set_attribute("component", "http")
                    span.set_attribute("http.method", method)
                    span.set_attribute("http.url", url)

                headers = get_or_create_headers()
                propagators.inject(type(headers).__setitem__, headers)

                token = context.attach(
                    context.set_value(
                        _SUPPRESS_REQUESTS_INSTRUMENTATION_KEY, True
                    )
                )
                try:
                    result = call_wrapped()  # *** PROCEED
                except Exception as exc:  # pylint: disable=W0703
                    exception = exc
                    setattr(
                        exception, EXCEPTION_STATUS_FIELD, StatusCode.ERROR,
                    )
                    result = getattr(exc, "response", None)
                finally:
                    context.detach(token)

                if result is not None:
                    if span.is_recording():
                        span.set_attribute(
                            "http.status_code", result.status_code
                        )
                        span.set_attribute("http.status_text", result.reason)
                        span.set_status(
                            Status(
                                http_status_to_status_code(result.status_code)
                            )
                        )
                    labels["http.status_code"] = str(result.status_code)
                    if result.raw and result.raw.version:
                        labels["http.flavor"] = (
                            str(result.raw.version)[:1]
                            + "."
                            + str(result.raw.version)[:-1]
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


class RequestsInstrumentor(BaseInstrumentor, MetricMixin):
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
        self.init_metrics(
            __name__, __version__,
        )
        # pylint: disable=W0201
        self.metric_recorder = HTTPMetricRecorder(
            self.meter, HTTPMetricType.CLIENT
        )

    def _uninstrument(self, **kwargs):
        _uninstrument()

    @staticmethod
    def uninstrument_session(session):
        """Disables instrumentation on the session object."""
        _uninstrument_from(session, restore_as_bound_func=True)
