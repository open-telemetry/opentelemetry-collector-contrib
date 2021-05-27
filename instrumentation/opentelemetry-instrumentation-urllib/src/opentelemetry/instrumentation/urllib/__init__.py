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
`urllib https://docs.python.org/3/library/urllib.html>`_ library.

Usage
-----

.. code-block:: python

    from urllib import request
    from opentelemetry.instrumentation.urllib import URLLibInstrumentor

    # You can optionally pass a custom TracerProvider to
    # URLLibInstrumentor().instrument()

    URLLibInstrumentor().instrument()
    req = request.Request('https://postman-echo.com/post', method="POST")
    r = request.urlopen(req)

API
---
"""

import functools
import types
from typing import Collection
from urllib.request import (  # pylint: disable=no-name-in-module,import-error
    OpenerDirector,
    Request,
)

from opentelemetry import context
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.urllib.package import _instruments
from opentelemetry.instrumentation.urllib.version import __version__
from opentelemetry.instrumentation.utils import http_status_to_status_code
from opentelemetry.propagate import inject
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import SpanKind, get_tracer
from opentelemetry.trace.status import Status

# A key to a context variable to avoid creating duplicate spans when instrumenting
_SUPPRESS_HTTP_INSTRUMENTATION_KEY = "suppress_http_instrumentation"


class URLLibInstrumentor(BaseInstrumentor):
    """An instrumentor for urllib
    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Instruments urllib module

        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global
                ``span_callback``: An optional callback invoked before returning the http response.
                 Invoked with Span and http.client.HTTPResponse
                ``name_callback``: Callback which calculates a generic span name for an
                    outgoing HTTP request based on the method and url.
                    Optional: Defaults to get_default_span_name.
        """
        tracer_provider = kwargs.get("tracer_provider")
        tracer = get_tracer(__name__, __version__, tracer_provider)
        _instrument(
            tracer,
            span_callback=kwargs.get("span_callback"),
            name_callback=kwargs.get("name_callback"),
        )

    def _uninstrument(self, **kwargs):
        _uninstrument()

    def uninstrument_opener(
        self, opener: OpenerDirector
    ):  # pylint: disable=no-self-use
        """uninstrument_opener a specific instance of urllib.request.OpenerDirector"""
        _uninstrument_from(opener, restore_as_bound_func=True)


def get_default_span_name(method):
    """Default implementation for name_callback, returns HTTP {method_name}."""
    return "HTTP {}".format(method).strip()


def _instrument(tracer, span_callback=None, name_callback=None):
    """Enables tracing of all requests calls that go through
    :code:`urllib.Client._make_request`"""

    opener_open = OpenerDirector.open

    @functools.wraps(opener_open)
    def instrumented_open(opener, fullurl, data=None, timeout=None):

        if isinstance(fullurl, str):
            request_ = Request(fullurl, data)
        else:
            request_ = fullurl

        def get_or_create_headers():
            return getattr(request_, "headers", {})

        def call_wrapped():
            return opener_open(opener, request_, data=data, timeout=timeout)

        return _instrumented_open_call(
            opener, request_, call_wrapped, get_or_create_headers
        )

    def _instrumented_open_call(
        _, request, call_wrapped, get_or_create_headers
    ):  # pylint: disable=too-many-locals
        if context.get_value("suppress_instrumentation") or context.get_value(
            _SUPPRESS_HTTP_INSTRUMENTATION_KEY
        ):
            return call_wrapped()

        method = request.get_method().upper()
        url = request.full_url

        span_name = ""
        if name_callback is not None:
            span_name = name_callback(method, url)
        if not span_name or not isinstance(span_name, str):
            span_name = get_default_span_name(method)

        labels = {
            SpanAttributes.HTTP_METHOD: method,
            SpanAttributes.HTTP_URL: url,
        }

        with tracer.start_as_current_span(
            span_name, kind=SpanKind.CLIENT
        ) as span:
            exception = None
            if span.is_recording():
                span.set_attribute(SpanAttributes.HTTP_METHOD, method)
                span.set_attribute(SpanAttributes.HTTP_URL, url)

            headers = get_or_create_headers()
            inject(headers)

            token = context.attach(
                context.set_value(_SUPPRESS_HTTP_INSTRUMENTATION_KEY, True)
            )
            try:
                result = call_wrapped()  # *** PROCEED
            except Exception as exc:  # pylint: disable=W0703
                exception = exc
                result = getattr(exc, "file", None)
            finally:
                context.detach(token)

            if result is not None:

                code_ = result.getcode()
                labels[SpanAttributes.HTTP_STATUS_CODE] = str(code_)

                if span.is_recording():
                    span.set_attribute(SpanAttributes.HTTP_STATUS_CODE, code_)
                    span.set_status(Status(http_status_to_status_code(code_)))

                ver_ = str(getattr(result, "version", ""))
                if ver_:
                    labels[SpanAttributes.HTTP_FLAVOR] = "{}.{}".format(
                        ver_[:1], ver_[:-1]
                    )

            if span_callback is not None:
                span_callback(span, result)

            if exception is not None:
                raise exception.with_traceback(exception.__traceback__)

        return result

    instrumented_open.opentelemetry_instrumentation_urllib_applied = True
    OpenerDirector.open = instrumented_open


def _uninstrument():
    """Disables instrumentation of :code:`urllib` through this module.

    Note that this only works if no other module also patches urllib."""
    _uninstrument_from(OpenerDirector)


def _uninstrument_from(instr_root, restore_as_bound_func=False):

    instr_func_name = "open"
    instr_func = getattr(instr_root, instr_func_name)
    if not getattr(
        instr_func, "opentelemetry_instrumentation_urllib_applied", False,
    ):
        return

    original = instr_func.__wrapped__  # pylint:disable=no-member
    if restore_as_bound_func:
        original = types.MethodType(original, instr_root)
    setattr(instr_root, instr_func_name, original)
