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
`urllib3 <https://urllib3.readthedocs.io/>`_ library.

Usage
-----
.. code-block:: python

    import urllib3
    from opentelemetry.instrumentation.urllib3 import URLLib3Instrumentor

    def strip_query_params(url: str) -> str:
        return url.split("?")[0]

    URLLib3Instrumentor().instrument(
        # Remove all query params from the URL attribute on the span.
        url_filter=strip_query_params,
    )

    http = urllib3.PoolManager()
    response = http.request("GET", "https://www.example.org/")

Hooks
*******

The urllib3 instrumentation supports extending tracing behavior with the help of
request and response hooks. These are functions that are called back by the instrumentation
right after a Span is created for a request and right before the span is finished processing a response respectively.
The hooks can be configured as follows:

.. code:: python

    # `request` is an instance of urllib3.connectionpool.HTTPConnectionPool
    def request_hook(span, request):
        pass

    # `request` is an instance of urllib3.connectionpool.HTTPConnectionPool
    # `response` is an instance of urllib3.response.HTTPResponse
    def response_hook(span, request, response):
        pass

    URLLib3Instrumentor.instrument(
        request_hook=request_hook, response_hook=response_hook)
    )

API
---
"""

import contextlib
import typing
from typing import Collection

import urllib3.connectionpool
import wrapt

from opentelemetry import context
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.urllib3.package import _instruments
from opentelemetry.instrumentation.urllib3.version import __version__
from opentelemetry.instrumentation.utils import (
    _SUPPRESS_INSTRUMENTATION_KEY,
    http_status_to_status_code,
    unwrap,
)
from opentelemetry.propagate import inject
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import Span, SpanKind, get_tracer
from opentelemetry.trace.status import Status

# A key to a context variable to avoid creating duplicate spans when instrumenting
# both, Session.request and Session.send, since Session.request calls into Session.send
_SUPPRESS_HTTP_INSTRUMENTATION_KEY = context.create_key(
    "suppress_http_instrumentation"
)

_UrlFilterT = typing.Optional[typing.Callable[[str], str]]
_RequestHookT = typing.Optional[
    typing.Callable[[Span, urllib3.connectionpool.HTTPConnectionPool], None]
]
_ResponseHookT = typing.Optional[
    typing.Callable[
        [
            Span,
            urllib3.connectionpool.HTTPConnectionPool,
            urllib3.response.HTTPResponse,
        ],
        None,
    ]
]

_URL_OPEN_ARG_TO_INDEX_MAPPING = {
    "method": 0,
    "url": 1,
}


class URLLib3Instrumentor(BaseInstrumentor):
    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Instruments the urllib3 module

        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global.
                ``request_hook``: An optional callback that is invoked right after a span is created.
                ``response_hook``: An optional callback which is invoked right before the span is finished processing a response.
                ``url_filter``: A callback to process the requested URL prior
                    to adding it as a span attribute.
        """
        tracer_provider = kwargs.get("tracer_provider")
        tracer = get_tracer(__name__, __version__, tracer_provider)
        _instrument(
            tracer,
            request_hook=kwargs.get("request_hook"),
            response_hook=kwargs.get("response_hook"),
            url_filter=kwargs.get("url_filter"),
        )

    def _uninstrument(self, **kwargs):
        _uninstrument()


def _instrument(
    tracer,
    request_hook: _RequestHookT = None,
    response_hook: _ResponseHookT = None,
    url_filter: _UrlFilterT = None,
):
    def instrumented_urlopen(wrapped, instance, args, kwargs):
        if _is_instrumentation_suppressed():
            return wrapped(*args, **kwargs)

        method = _get_url_open_arg("method", args, kwargs).upper()
        url = _get_url(instance, args, kwargs, url_filter)
        headers = _prepare_headers(kwargs)

        span_name = "HTTP {}".format(method.strip())
        span_attributes = {
            SpanAttributes.HTTP_METHOD: method,
            SpanAttributes.HTTP_URL: url,
        }

        with tracer.start_as_current_span(
            span_name, kind=SpanKind.CLIENT, attributes=span_attributes
        ) as span:
            if callable(request_hook):
                request_hook(span, instance)
            inject(headers)

            with _suppress_further_instrumentation():
                response = wrapped(*args, **kwargs)

            _apply_response(span, response)
            if callable(response_hook):
                response_hook(span, instance, response)
            return response

    wrapt.wrap_function_wrapper(
        urllib3.connectionpool.HTTPConnectionPool,
        "urlopen",
        instrumented_urlopen,
    )


def _get_url_open_arg(name: str, args: typing.List, kwargs: typing.Mapping):
    arg_idx = _URL_OPEN_ARG_TO_INDEX_MAPPING.get(name)
    if arg_idx is not None:
        try:
            return args[arg_idx]
        except IndexError:
            pass
    return kwargs.get(name)


def _get_url(
    instance: urllib3.connectionpool.HTTPConnectionPool,
    args: typing.List,
    kwargs: typing.Mapping,
    url_filter: _UrlFilterT,
) -> str:
    url_or_path = _get_url_open_arg("url", args, kwargs)
    if not url_or_path.startswith("/"):
        url = url_or_path
    else:
        url = instance.scheme + "://" + instance.host
        if _should_append_port(instance.scheme, instance.port):
            url += ":" + str(instance.port)
        url += url_or_path

    if url_filter:
        return url_filter(url)
    return url


def _should_append_port(scheme: str, port: typing.Optional[int]) -> bool:
    if not port:
        return False
    if scheme == "http" and port == 80:
        return False
    if scheme == "https" and port == 443:
        return False
    return True


def _prepare_headers(urlopen_kwargs: typing.Dict) -> typing.Dict:
    headers = urlopen_kwargs.get("headers")

    # avoid modifying original headers on inject
    headers = headers.copy() if headers is not None else {}
    urlopen_kwargs["headers"] = headers

    return headers


def _apply_response(span: Span, response: urllib3.response.HTTPResponse):
    if not span.is_recording():
        return

    span.set_attribute(SpanAttributes.HTTP_STATUS_CODE, response.status)
    span.set_status(Status(http_status_to_status_code(response.status)))


def _is_instrumentation_suppressed() -> bool:
    return bool(
        context.get_value(_SUPPRESS_INSTRUMENTATION_KEY)
        or context.get_value(_SUPPRESS_HTTP_INSTRUMENTATION_KEY)
    )


@contextlib.contextmanager
def _suppress_further_instrumentation():
    token = context.attach(
        context.set_value(_SUPPRESS_HTTP_INSTRUMENTATION_KEY, True)
    )
    try:
        yield
    finally:
        context.detach(token)


def _uninstrument():
    unwrap(urllib3.connectionpool.HTTPConnectionPool, "urlopen")
