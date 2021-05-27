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
This library allows tracing PostgreSQL queries made by the
`asyncpg <https://magicstack.github.io/asyncpg/current/>`_ library.

Usage
-----

.. code-block:: python

    import asyncpg
    from opentelemetry.instrumentation.asyncpg import AsyncPGInstrumentor

    # You can optionally pass a custom TracerProvider to AsyncPGInstrumentor.instrument()
    AsyncPGInstrumentor().instrument()
    conn = await asyncpg.connect(user='user', password='password',
                                 database='database', host='127.0.0.1')
    values = await conn.fetch('''SELECT 42;''')

API
---
"""

from typing import Collection

import asyncpg
import wrapt
from asyncpg import exceptions

from opentelemetry import trace
from opentelemetry.instrumentation.asyncpg.package import _instruments
from opentelemetry.instrumentation.asyncpg.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.semconv.trace import (
    DbSystemValues,
    NetTransportValues,
    SpanAttributes,
)
from opentelemetry.trace import SpanKind
from opentelemetry.trace.status import Status, StatusCode


def _hydrate_span_from_args(connection, query, parameters) -> dict:
    """Get network and database attributes from connection."""
    span_attributes = {
        SpanAttributes.DB_SYSTEM: DbSystemValues.POSTGRESQL.value
    }

    # connection contains _params attribute which is a namedtuple ConnectionParameters.
    # https://github.com/MagicStack/asyncpg/blob/master/asyncpg/connection.py#L68

    params = getattr(connection, "_params", None)
    dbname = getattr(params, "database", None)
    if dbname:
        span_attributes[SpanAttributes.DB_NAME] = dbname
    user = getattr(params, "user", None)
    if user:
        span_attributes[SpanAttributes.DB_USER] = user

    # connection contains _addr attribute which is either a host/port tuple, or unix socket string
    # https://magicstack.github.io/asyncpg/current/_modules/asyncpg/connection.html
    addr = getattr(connection, "_addr", None)
    if isinstance(addr, tuple):
        span_attributes[SpanAttributes.NET_PEER_NAME] = addr[0]
        span_attributes[SpanAttributes.NET_PEER_PORT] = addr[1]
        span_attributes[
            SpanAttributes.NET_TRANSPORT
        ] = NetTransportValues.IP_TCP.value
    elif isinstance(addr, str):
        span_attributes[SpanAttributes.NET_PEER_NAME] = addr
        span_attributes[
            SpanAttributes.NET_TRANSPORT
        ] = NetTransportValues.UNIX.value

    if query is not None:
        span_attributes[SpanAttributes.DB_STATEMENT] = query

    if parameters is not None and len(parameters) > 0:
        span_attributes["db.statement.parameters"] = str(parameters)

    return span_attributes


class AsyncPGInstrumentor(BaseInstrumentor):
    def __init__(self, capture_parameters=False):
        super().__init__()
        self.capture_parameters = capture_parameters
        self._tracer = None

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        tracer_provider = kwargs.get("tracer_provider")
        self._tracer = trace.get_tracer(__name__, __version__, tracer_provider)

        for method in [
            "Connection.execute",
            "Connection.executemany",
            "Connection.fetch",
            "Connection.fetchval",
            "Connection.fetchrow",
        ]:
            wrapt.wrap_function_wrapper(
                "asyncpg.connection", method, self._do_execute
            )

    def _uninstrument(self, **__):
        for method in [
            "execute",
            "executemany",
            "fetch",
            "fetchval",
            "fetchrow",
        ]:
            unwrap(asyncpg.Connection, method)

    async def _do_execute(self, func, instance, args, kwargs):

        exception = None
        params = getattr(instance, "_params", {})
        name = args[0] if args[0] else params.get("database", "postgresql")

        with self._tracer.start_as_current_span(
            name, kind=SpanKind.CLIENT
        ) as span:
            if span.is_recording():
                span_attributes = _hydrate_span_from_args(
                    instance,
                    args[0],
                    args[1:] if self.capture_parameters else None,
                )
                for attribute, value in span_attributes.items():
                    span.set_attribute(attribute, value)

            try:
                result = await func(*args, **kwargs)
            except Exception as exc:  # pylint: disable=W0703
                exception = exc
                raise
            finally:
                if span.is_recording() and exception is not None:
                    span.set_status(Status(StatusCode.ERROR))

        return result
