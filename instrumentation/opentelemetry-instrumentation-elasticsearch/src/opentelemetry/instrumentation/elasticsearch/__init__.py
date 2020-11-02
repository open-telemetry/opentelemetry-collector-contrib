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

    from opentelemetry import trace
    from opentelemetry.instrumentation.elasticsearch import ElasticSearchInstrumentor
    from opentelemetry.sdk.trace import TracerProvider
    import elasticsearch

    trace.set_tracer_provider(TracerProvider())

    # instrument elasticsearch
    ElasticSearchInstrumentor().instrument(tracer_provider=trace.get_tracer_provider())

    # Using elasticsearch as normal now will automatically generate spans
    es = elasticsearch.Elasticsearch()
    es.index(index='my-index', doc_type='my-type', id=1, body={'my': 'data', 'timestamp': datetime.now()})
    es.get(index='my-index', doc_type='my-type', id=1)

API
---

Elasticsearch instrumentation prefixes operation names with the string "Elasticsearch". This
can be changed to a different string by either setting the `OTEL_PYTHON_ELASTICSEARCH_NAME_PREFIX`
environment variable or by passing the prefix as an argument to the instrumentor. For example,


.. code-block:: python

    ElasticSearchInstrumentor("my-custom-prefix").instrument()
"""

import functools
import types
from logging import getLogger
from os import environ

import elasticsearch
import elasticsearch.exceptions
from wrapt import ObjectProxy
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry import context, propagators, trace
from opentelemetry.instrumentation.elasticsearch.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.trace import SpanKind, get_tracer
from opentelemetry.trace.status import Status, StatusCode

logger = getLogger(__name__)


# Values to add as tags from the actual
# payload returned by Elasticsearch, if any.
_ATTRIBUTES_FROM_RESULT = [
    "found",
    "timed_out",
    "took",
]

_DEFALT_OP_NAME = "request"


class ElasticsearchInstrumentor(BaseInstrumentor):
    """An instrumentor for elasticsearch
    See `BaseInstrumentor`
    """

    def __init__(self, span_name_prefix=None):
        if not span_name_prefix:
            span_name_prefix = environ.get(
                "OTEL_PYTHON_ELASTICSEARCH_NAME_PREFIX", "Elasticsearch",
            )
        self._span_name_prefix = span_name_prefix.strip()
        super().__init__()

    def _instrument(self, **kwargs):
        """
        Instruments elasticsarch module
        """
        tracer_provider = kwargs.get("tracer_provider")
        tracer = get_tracer(__name__, __version__, tracer_provider)
        _wrap(
            elasticsearch,
            "Transport.perform_request",
            _wrap_perform_request(tracer, self._span_name_prefix),
        )

    def _uninstrument(self, **kwargs):
        unwrap(elasticsearch.Transport, "perform_request")


def _wrap_perform_request(tracer, span_name_prefix):
    # pylint: disable=R0912
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

        op_name = span_name_prefix + (url or method or _DEFALT_OP_NAME)
        params = kwargs.get("params", {})
        body = kwargs.get("body", None)

        with tracer.start_as_current_span(
            op_name, kind=SpanKind.CLIENT,
        ) as span:
            if span.is_recording():
                attributes = {
                    "component": "elasticsearch-py",
                    "db.type": "elasticsearch",
                }
                if url:
                    attributes["elasticsearch.url"] = url
                if method:
                    attributes["elasticsearch.method"] = method
                if body:
                    attributes["db.statement"] = str(body)
                if params:
                    attributes["elasticsearch.params"] = str(params)
                for key, value in attributes.items():
                    span.set_attribute(key, value)
            try:
                rv = wrapped(*args, **kwargs)
                if isinstance(rv, dict) and span.is_recording():
                    for member in _ATTRIBUTES_FROM_RESULT:
                        if member in rv:
                            span.set_attribute(
                                "elasticsearch.{0}".format(member),
                                str(rv[member]),
                            )
                return rv
            except Exception as ex:  # pylint: disable=broad-except
                if span.is_recording():
                    span.set_status(Status(StatusCode.ERROR, str(ex)))
                raise ex

    return wrapper
