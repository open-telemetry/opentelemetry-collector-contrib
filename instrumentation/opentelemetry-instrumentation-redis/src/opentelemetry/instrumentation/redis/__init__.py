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
#
"""
Instrument `redis`_ to report Redis queries.

There are two options for instrumenting code. The first option is to use the
``opentelemetry-instrumentation`` executable which will automatically
instrument your Redis client. The second is to programmatically enable
instrumentation via the following code:

.. _redis: https://pypi.org/project/redis/

Usage
-----

.. code:: python

    from opentelemetry.instrumentation.redis import RedisInstrumentor
    import redis


    # Instrument redis
    RedisInstrumentor().instrument()

    # This will report a span with the default settings
    client = redis.StrictRedis(host="localhost", port=6379)
    client.get("my-key")

The `instrument` method accepts the following keyword args:

tracer_provider (TracerProvider) - an optional tracer provider

request_hook (Callable) - a function with extra user-defined logic to be performed before performing the request
this function signature is:  def request_hook(span: Span, instance: redis.connection.Connection, args, kwargs) -> None

response_hook (Callable) - a function with extra user-defined logic to be performed after performing the request
this function signature is: def response_hook(span: Span, instance: redis.connection.Connection, response) -> None

for example:

.. code: python

    from opentelemetry.instrumentation.redis import RedisInstrumentor
    import redis

    def request_hook(span, instance, args, kwargs):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_request_hook", "some-value")

    def response_hook(span, instance, response):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_response_hook", "some-value")

    # Instrument redis with hooks
    RedisInstrumentor().instrument(request_hook=request_hook, response_hook=response_hook)

    # This will report a span with the default settings and the custom attributes added from the hooks
    client = redis.StrictRedis(host="localhost", port=6379)
    client.get("my-key")

API
---
"""
import typing
from typing import Any, Collection

import redis
from wrapt import wrap_function_wrapper

from opentelemetry import trace
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.redis.package import _instruments
from opentelemetry.instrumentation.redis.util import (
    _extract_conn_attributes,
    _format_command_args,
)
from opentelemetry.instrumentation.redis.version import __version__
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import Span

_DEFAULT_SERVICE = "redis"

_RequestHookT = typing.Optional[
    typing.Callable[
        [Span, redis.connection.Connection, typing.List, typing.Dict], None
    ]
]
_ResponseHookT = typing.Optional[
    typing.Callable[[Span, redis.connection.Connection, Any], None]
]


def _set_connection_attributes(span, conn):
    if not span.is_recording():
        return
    for key, value in _extract_conn_attributes(
        conn.connection_pool.connection_kwargs
    ).items():
        span.set_attribute(key, value)


def _instrument(
    tracer,
    request_hook: _RequestHookT = None,
    response_hook: _ResponseHookT = None,
):
    def _traced_execute_command(func, instance, args, kwargs):
        query = _format_command_args(args)
        name = ""
        if len(args) > 0 and args[0]:
            name = args[0]
        else:
            name = instance.connection_pool.connection_kwargs.get("db", 0)
        with tracer.start_as_current_span(
            name, kind=trace.SpanKind.CLIENT
        ) as span:
            if span.is_recording():
                span.set_attribute(SpanAttributes.DB_STATEMENT, query)
                _set_connection_attributes(span, instance)
                span.set_attribute("db.redis.args_length", len(args))
            if callable(request_hook):
                request_hook(span, instance, args, kwargs)
            response = func(*args, **kwargs)
            if callable(response_hook):
                response_hook(span, instance, response)
            return response

    def _traced_execute_pipeline(func, instance, args, kwargs):
        cmds = [_format_command_args(c) for c, _ in instance.command_stack]
        resource = "\n".join(cmds)

        span_name = " ".join([args[0] for args, _ in instance.command_stack])

        with tracer.start_as_current_span(
            span_name, kind=trace.SpanKind.CLIENT
        ) as span:
            if span.is_recording():
                span.set_attribute(SpanAttributes.DB_STATEMENT, resource)
                _set_connection_attributes(span, instance)
                span.set_attribute(
                    "db.redis.pipeline_length", len(instance.command_stack)
                )
            response = func(*args, **kwargs)
            if callable(response_hook):
                response_hook(span, instance, response)
            return response

    pipeline_class = (
        "BasePipeline" if redis.VERSION < (3, 0, 0) else "Pipeline"
    )
    redis_class = "StrictRedis" if redis.VERSION < (3, 0, 0) else "Redis"

    wrap_function_wrapper(
        "redis", f"{redis_class}.execute_command", _traced_execute_command
    )
    wrap_function_wrapper(
        "redis.client",
        f"{pipeline_class}.execute",
        _traced_execute_pipeline,
    )
    wrap_function_wrapper(
        "redis.client",
        f"{pipeline_class}.immediate_execute_command",
        _traced_execute_command,
    )


class RedisInstrumentor(BaseInstrumentor):
    """An instrumentor for Redis
    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Instruments the redis module

        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global.
                ``response_hook``: An optional callback which is invoked right before the span is finished processing a response.
        """
        tracer_provider = kwargs.get("tracer_provider")
        tracer = trace.get_tracer(
            __name__, __version__, tracer_provider=tracer_provider
        )
        _instrument(
            tracer,
            request_hook=kwargs.get("request_hook"),
            response_hook=kwargs.get("response_hook"),
        )

    def _uninstrument(self, **kwargs):
        if redis.VERSION < (3, 0, 0):
            unwrap(redis.StrictRedis, "execute_command")
            unwrap(redis.StrictRedis, "pipeline")
            unwrap(redis.Redis, "pipeline")
            unwrap(
                redis.client.BasePipeline,  # pylint:disable=no-member
                "execute",
            )
            unwrap(
                redis.client.BasePipeline,  # pylint:disable=no-member
                "immediate_execute_command",
            )
        else:
            unwrap(redis.Redis, "execute_command")
            unwrap(redis.Redis, "pipeline")
            unwrap(redis.client.Pipeline, "execute")
            unwrap(redis.client.Pipeline, "immediate_execute_command")
