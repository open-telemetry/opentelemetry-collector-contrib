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

API
---
"""

from typing import Collection

import redis
from wrapt import ObjectProxy, wrap_function_wrapper

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

_DEFAULT_SERVICE = "redis"


def _set_connection_attributes(span, conn):
    if not span.is_recording():
        return
    for key, value in _extract_conn_attributes(
        conn.connection_pool.connection_kwargs
    ).items():
        span.set_attribute(key, value)


def _traced_execute_command(func, instance, args, kwargs):
    tracer = getattr(redis, "_opentelemetry_tracer")
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
        return func(*args, **kwargs)


def _traced_execute_pipeline(func, instance, args, kwargs):
    tracer = getattr(redis, "_opentelemetry_tracer")

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
        return func(*args, **kwargs)


class RedisInstrumentor(BaseInstrumentor):
    """An instrumentor for Redis
    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        tracer_provider = kwargs.get("tracer_provider")
        setattr(
            redis,
            "_opentelemetry_tracer",
            trace.get_tracer(
                __name__, __version__, tracer_provider=tracer_provider,
            ),
        )

        if redis.VERSION < (3, 0, 0):
            wrap_function_wrapper(
                "redis", "StrictRedis.execute_command", _traced_execute_command
            )
            wrap_function_wrapper(
                "redis.client",
                "BasePipeline.execute",
                _traced_execute_pipeline,
            )
            wrap_function_wrapper(
                "redis.client",
                "BasePipeline.immediate_execute_command",
                _traced_execute_command,
            )
        else:
            wrap_function_wrapper(
                "redis", "Redis.execute_command", _traced_execute_command
            )
            wrap_function_wrapper(
                "redis.client", "Pipeline.execute", _traced_execute_pipeline
            )
            wrap_function_wrapper(
                "redis.client",
                "Pipeline.immediate_execute_command",
                _traced_execute_command,
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
