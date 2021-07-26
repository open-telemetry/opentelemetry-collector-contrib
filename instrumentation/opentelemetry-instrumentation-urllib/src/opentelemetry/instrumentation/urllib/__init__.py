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

Hooks
*******

The urllib instrumentation supports extending tracing behavior with the help of
request and response hooks. These are functions that are called back by the instrumentation
right after a Span is created for a request and right before the span is finished processing a response respectively.
The hooks can be configured as follows:

..code:: python

    # `request_obj` is an instance of urllib.request.Request
    def request_hook(span, request_obj):
        pass

    # `request_obj` is an instance of urllib.request.Request
    # `response` is an instance of http.client.HTTPResponse
    def response_hook(span, request_obj, response)
        pass

    URLLibInstrumentor.instrument(
        request_hook=request_hook, response_hook=response_hook)
    )

API
---
"""

import functools
import types
import typing

# from urllib import response
from http import client
from typing import Collection
from urllib.request import (  # pylint: disable=no-name-in-module,import-error
    OpenerDirector,
    Request,
)

from opentelemetry import context
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.urllib.package import _instruments
from opentelemetry.instrumentation.urllib.version import __version__
from opentelemetry.instrumentation.utils import (
    _SUPPRESS_INSTRUMENTATION_KEY,
    http_status_to_status_code,
)
from opentelemetry.propagate import inject
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import Span, SpanKind, get_tracer
from opentelemetry.trace.status import Status
from opentelemetry.util.http import remove_url_credentials

# A key to a context variable to avoid creating duplicate spans when instrumenting
# both, Session.request and Session.send, since Session.request calls into Session.send
_SUPPRESS_HTTP_INSTRUMENTATION_KEY = context.create_key(
    "suppress_http_instrumentation"
)

_RequestHookT = typing.Optional[typing.Callable[[Span, Request], None]]
_ResponseHookT = typing.Optional[
    typing.Callable[[Span, Request, client.HTTPResponse], None]
]


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
                ``request_hook``: An optional callback invoked that is invoked right after a span is created.
                ``response_hook``: An optional callback which is invoked right before the span is finished processing a response
        """
        tracer_provider = kwargs.get("tracer_provider")
        tracer = get_tracer(__name__, __version__, tracer_provider)
        _instrument(
            tracer,
            request_hook=kwargs.get("request_hook"),
            response_hook=kwargs.get("response_hook"),
        )

    def _uninstrument(self, **kwargs):
        _uninstrument()

    def uninstrument_opener(
        self, opener: OpenerDirector
    ):  # pylint: disable=no-self-use
        """uninstrument_opener a specific instance of urllib.request.OpenerDirector"""
        _uninstrument_from(opener, restore_as_bound_func=True)


def _instrument(
    tracer,
    request_hook: _RequestHookT = None,
    response_hook: _ResponseHookT = None,
):
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
        if context.get_value(
            _SUPPRESS_INSTRUMENTATION_KEY
        ) or context.get_value(_SUPPRESS_HTTP_INSTRUMENTATION_KEY):
            return call_wrapped()

        method = request.get_method().upper()
        url = request.full_url

        span_name = "HTTP {}".format(method).strip()

        url = remove_url_credentials(url)

        labels = {
            SpanAttributes.HTTP_METHOD: method,
            SpanAttributes.HTTP_URL: url,
        }

        with tracer.start_as_current_span(
            span_name, kind=SpanKind.CLIENT
        ) as span:
            exception = None
            if callable(request_hook):
                request_hook(span, request)
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

            if callable(response_hook):
                response_hook(span, request, result)

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
