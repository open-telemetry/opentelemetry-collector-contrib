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
import os

from sqlalchemy.event import listen  # pylint: disable=no-name-in-module

from opentelemetry import trace
from opentelemetry.instrumentation.sqlalchemy.version import __version__
from opentelemetry.instrumentation.utils import (
    _generate_opentelemetry_traceparent,
    _generate_sql_comment,
)
from opentelemetry.semconv.trace import NetTransportValues, SpanAttributes
from opentelemetry.trace import Span
from opentelemetry.trace.status import Status, StatusCode


def _normalize_vendor(vendor):
    """Return a canonical name for a type of database."""
    if not vendor:
        return "db"  # should this ever happen?

    if "sqlite" in vendor:
        return "sqlite"

    if "postgres" in vendor or vendor == "psycopg2":
        return "postgresql"

    return vendor


def _get_tracer(engine, tracer_provider=None):
    return trace.get_tracer(
        _normalize_vendor(engine.name),
        __version__,
        tracer_provider=tracer_provider,
    )


def _wrap_create_async_engine(tracer_provider=None):
    # pylint: disable=unused-argument
    def _wrap_create_async_engine_internal(func, module, args, kwargs):
        """Trace the SQLAlchemy engine, creating an `EngineTracer`
        object that will listen to SQLAlchemy events.
        """
        engine = func(*args, **kwargs)
        EngineTracer(_get_tracer(engine, tracer_provider), engine.sync_engine)
        return engine

    return _wrap_create_async_engine_internal


def _wrap_create_engine(tracer_provider=None):
    # pylint: disable=unused-argument
    def _wrap_create_engine_internal(func, module, args, kwargs):
        """Trace the SQLAlchemy engine, creating an `EngineTracer`
        object that will listen to SQLAlchemy events.
        """
        engine = func(*args, **kwargs)
        EngineTracer(_get_tracer(engine, tracer_provider), engine)
        return engine

    return _wrap_create_engine_internal


class EngineTracer:
    def __init__(self, tracer, engine, enable_commenter=False):
        self.tracer = tracer
        self.engine = engine
        self.vendor = _normalize_vendor(engine.name)
        self.enable_commenter = enable_commenter

        listen(
            engine, "before_cursor_execute", self._before_cur_exec, retval=True
        )
        listen(engine, "after_cursor_execute", _after_cur_exec)
        listen(engine, "handle_error", _handle_error)

    def _operation_name(self, db_name, statement):
        parts = []
        if isinstance(statement, str):
            # otel spec recommends against parsing SQL queries. We are not trying to parse SQL
            # but simply truncating the statement to the first word. This covers probably >95%
            # use cases and uses the SQL statement in span name correctly as per the spec.
            # For some very special cases it might not record the correct statement if the SQL
            # dialect is too weird but in any case it shouldn't break anything.
            parts.append(statement.split()[0])
        if db_name:
            parts.append(db_name)
        if not parts:
            return self.vendor
        return " ".join(parts)

    # pylint: disable=unused-argument
    def _before_cur_exec(
        self, conn, cursor, statement, params, context, executemany
    ):
        attrs, found = _get_attributes_from_url(conn.engine.url)
        if not found:
            attrs = _get_attributes_from_cursor(self.vendor, cursor, attrs)

        db_name = attrs.get(SpanAttributes.DB_NAME, "")
        span = self.tracer.start_span(
            self._operation_name(db_name, statement),
            kind=trace.SpanKind.CLIENT,
        )
        with trace.use_span(span, end_on_exit=False):
            if span.is_recording():
                span.set_attribute(SpanAttributes.DB_STATEMENT, statement)
                span.set_attribute(SpanAttributes.DB_SYSTEM, self.vendor)
                for key, value in attrs.items():
                    span.set_attribute(key, value)

        context._otel_span = span
        if self.enable_commenter:
            statement = statement + EngineTracer._generate_comment(span=span)

        return statement, params

    @staticmethod
    def _generate_comment(span: Span) -> str:
        span_context = span.get_span_context()
        meta = {}
        if span_context.is_valid:
            meta.update(_generate_opentelemetry_traceparent(span))
        return _generate_sql_comment(**meta)


# pylint: disable=unused-argument
def _after_cur_exec(conn, cursor, statement, params, context, executemany):
    span = getattr(context, "_otel_span", None)
    if span is None:
        return

    span.end()


def _handle_error(context):
    span = getattr(context.execution_context, "_otel_span", None)
    if span is None:
        return

    if span.is_recording():
        span.set_status(
            Status(
                StatusCode.ERROR,
                str(context.original_exception),
            )
        )
    span.end()


def _get_attributes_from_url(url):
    """Set connection tags from the url. return true if successful."""
    attrs = {}
    if url.host:
        attrs[SpanAttributes.NET_PEER_NAME] = url.host
    if url.port:
        attrs[SpanAttributes.NET_PEER_PORT] = url.port
    if url.database:
        attrs[SpanAttributes.DB_NAME] = url.database
    if url.username:
        attrs[SpanAttributes.DB_USER] = url.username
    return attrs, bool(url.host)


def _get_attributes_from_cursor(vendor, cursor, attrs):
    """Attempt to set db connection attributes by introspecting the cursor."""
    if vendor == "postgresql":
        info = getattr(getattr(cursor, "connection", None), "info", None)
        if not info:
            return attrs

        attrs[SpanAttributes.DB_NAME] = info.dbname
        is_unix_socket = info.host and info.host.startswith("/")

        if is_unix_socket:
            attrs[SpanAttributes.NET_TRANSPORT] = NetTransportValues.UNIX.value
            if info.port:
                # postgresql enforces this pattern on all socket names
                attrs[SpanAttributes.NET_PEER_NAME] = os.path.join(
                    info.host, f".s.PGSQL.{info.port}"
                )
        else:
            attrs[
                SpanAttributes.NET_TRANSPORT
            ] = NetTransportValues.IP_TCP.value
            attrs[SpanAttributes.NET_PEER_NAME] = info.host
            if info.port:
                attrs[SpanAttributes.NET_PEER_PORT] = int(info.port)
    return attrs
