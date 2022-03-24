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
This library allows tracing HTTP elasticsearch made by the
`elasticsearch <https://elasticsearch-py.readthedocs.io/en/master/>`_ library.

Usage
-----

.. code-block:: python

    from opentelemetry.instrumentation.elasticsearch import ElasticsearchInstrumentor
    import elasticsearch


    # instrument elasticsearch
    ElasticsearchInstrumentor().instrument()

    # Using elasticsearch as normal now will automatically generate spans
    es = elasticsearch.Elasticsearch()
    es.index(index='my-index', doc_type='my-type', id=1, body={'my': 'data', 'timestamp': datetime.now()})
    es.get(index='my-index', doc_type='my-type', id=1)

Elasticsearch instrumentation prefixes operation names with the string "Elasticsearch". This
can be changed to a different string by either setting the `OTEL_PYTHON_ELASTICSEARCH_NAME_PREFIX`
environment variable or by passing the prefix as an argument to the instrumentor. For example,


.. code-block:: python

    ElasticsearchInstrumentor("my-custom-prefix").instrument()


The `instrument` method accepts the following keyword args:

tracer_provider (TracerProvider) - an optional tracer provider
request_hook (Callable) - a function with extra user-defined logic to be performed before performing the request
                          this function signature is:
                          def request_hook(span: Span, method: str, url: str, kwargs)
response_hook (Callable) - a function with extra user-defined logic to be performed after performing the request
                          this function signature is:
                          def response_hook(span: Span, response: dict)

for example:

.. code: python

    from opentelemetry.instrumentation.elasticsearch import ElasticsearchInstrumentor
    import elasticsearch

    def request_hook(span, method, url, kwargs):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_request_hook", "some-value")

    def response_hook(span, response):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_response_hook", "some-value")

    # instrument elasticsearch with request and response hooks
    ElasticsearchInstrumentor().instrument(request_hook=request_hook, response_hook=response_hook)

    # Using elasticsearch as normal now will automatically generate spans,
    # including user custom attributes added from the hooks
    es = elasticsearch.Elasticsearch()
    es.index(index='my-index', doc_type='my-type', id=1, body={'my': 'data', 'timestamp': datetime.now()})
    es.get(index='my-index', doc_type='my-type', id=1)

API
---
"""

import re
from logging import getLogger
from os import environ
from typing import Collection

import elasticsearch
import elasticsearch.exceptions
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry.instrumentation.elasticsearch.package import _instruments
from opentelemetry.instrumentation.elasticsearch.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import SpanKind, get_tracer

logger = getLogger(__name__)


# Values to add as tags from the actual
# payload returned by Elasticsearch, if any.
_ATTRIBUTES_FROM_RESULT = [
    "found",
    "timed_out",
    "took",
]

_DEFAULT_OP_NAME = "request"


class ElasticsearchInstrumentor(BaseInstrumentor):
    """An instrumentor for elasticsearch
    See `BaseInstrumentor`
    """

    def __init__(self, span_name_prefix=None):
        if not span_name_prefix:
            span_name_prefix = environ.get(
                "OTEL_PYTHON_ELASTICSEARCH_NAME_PREFIX",
                "Elasticsearch",
            )
        self._span_name_prefix = span_name_prefix.strip()
        super().__init__()

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """
        Instruments elasticsarch module
        """
        tracer_provider = kwargs.get("tracer_provider")
        tracer = get_tracer(__name__, __version__, tracer_provider)
        request_hook = kwargs.get("request_hook")
        response_hook = kwargs.get("response_hook")
        _wrap(
            elasticsearch,
            "Transport.perform_request",
            _wrap_perform_request(
                tracer, self._span_name_prefix, request_hook, response_hook
            ),
        )

    def _uninstrument(self, **kwargs):
        unwrap(elasticsearch.Transport, "perform_request")


_regex_doc_url = re.compile(r"/_doc/([^/]+)")

# search api https://www.elastic.co/guide/en/elasticsearch/reference/current/search-search.html
_regex_search_url = re.compile(r"/([^/]+)/_search[/]?")


def _wrap_perform_request(
    tracer, span_name_prefix, request_hook=None, response_hook=None
):
    # pylint: disable=R0912,R0914
    def wrapper(wrapped, _, args, kwargs):
        method = url = None
        try:
            method, url, *_ = args
        except IndexError:
            logger.warning(
                "expected perform_request to receive two positional arguments. "
                "Got %d",
                len(args),
            )

        op_name = span_name_prefix + (url or method or _DEFAULT_OP_NAME)

        doc_id = None
        search_target = None

        if url:
            # TODO: This regex-based solution avoids creating an unbounded number of span names, but should be replaced by instrumenting individual Elasticsearch methods instead of Transport.perform_request()
            # A limitation of the regex is that only the '_doc' mapping type is supported. Mapping types are deprecated since Elasticsearch 7
            # https://github.com/open-telemetry/opentelemetry-python-contrib/issues/708
            match = _regex_doc_url.search(url)
            if match is not None:
                # Remove the full document ID from the URL
                doc_span = match.span()
                op_name = (
                    span_name_prefix
                    + url[: doc_span[0]]
                    + "/_doc/:id"
                    + url[doc_span[1] :]
                )
                # Put the document ID in attributes
                doc_id = match.group(1)
            match = _regex_search_url.search(url)
            if match is not None:
                op_name = span_name_prefix + "/<target>/_search"
                search_target = match.group(1)

        params = kwargs.get("params", {})
        body = kwargs.get("body", None)

        with tracer.start_as_current_span(
            op_name,
            kind=SpanKind.CLIENT,
        ) as span:

            if callable(request_hook):
                request_hook(span, method, url, kwargs)

            if span.is_recording():
                attributes = {
                    SpanAttributes.DB_SYSTEM: "elasticsearch",
                }
                if url:
                    attributes["elasticsearch.url"] = url
                if method:
                    attributes["elasticsearch.method"] = method
                if body:
                    attributes[SpanAttributes.DB_STATEMENT] = str(body)
                if params:
                    attributes["elasticsearch.params"] = str(params)
                if doc_id:
                    attributes["elasticsearch.id"] = doc_id
                if search_target:
                    attributes["elasticsearch.target"] = search_target
                for key, value in attributes.items():
                    span.set_attribute(key, value)

            rv = wrapped(*args, **kwargs)
            if isinstance(rv, dict) and span.is_recording():
                for member in _ATTRIBUTES_FROM_RESULT:
                    if member in rv:
                        span.set_attribute(
                            f"elasticsearch.{member}",
                            str(rv[member]),
                        )

            if callable(response_hook):
                response_hook(span, rv)
            return rv

    return wrapper
