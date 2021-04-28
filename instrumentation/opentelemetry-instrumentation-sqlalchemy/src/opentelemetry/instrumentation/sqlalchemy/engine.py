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

from threading import local
from weakref import WeakKeyDictionary

from sqlalchemy.event import listen  # pylint: disable=no-name-in-module

from opentelemetry import trace
from opentelemetry.instrumentation.sqlalchemy.version import __version__
from opentelemetry.semconv.trace import SpanAttributes
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


# pylint: disable=unused-argument
def _wrap_create_engine(func, module, args, kwargs):
    """Trace the SQLAlchemy engine, creating an `EngineTracer`
    object that will listen to SQLAlchemy events.
    """
    engine = func(*args, **kwargs)
    EngineTracer(_get_tracer(engine), engine)
    return engine


class EngineTracer:
    def __init__(self, tracer, engine):
        self.tracer = tracer
        self.engine = engine
        self.vendor = _normalize_vendor(engine.name)
        self.cursor_mapping = WeakKeyDictionary()
        self.local = local()

        listen(engine, "before_cursor_execute", self._before_cur_exec)
        listen(engine, "after_cursor_execute", self._after_cur_exec)
        listen(engine, "handle_error", self._handle_error)

    @property
    def current_thread_span(self):
        return getattr(self.local, "current_span", None)

    @current_thread_span.setter
    def current_thread_span(self, span):
        setattr(self.local, "current_span", span)

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
    def _before_cur_exec(self, conn, cursor, statement, *args):
        attrs, found = _get_attributes_from_url(conn.engine.url)
        if not found:
            attrs = _get_attributes_from_cursor(self.vendor, cursor, attrs)

        db_name = attrs.get(SpanAttributes.DB_NAME, "")
        span = self.tracer.start_span(
            self._operation_name(db_name, statement),
            kind=trace.SpanKind.CLIENT,
        )
        self.current_thread_span = self.cursor_mapping[cursor] = span
        with trace.use_span(span, end_on_exit=False):
            if span.is_recording():
                span.set_attribute(SpanAttributes.DB_STATEMENT, statement)
                span.set_attribute(SpanAttributes.DB_SYSTEM, self.vendor)
                for key, value in attrs.items():
                    span.set_attribute(key, value)

    # pylint: disable=unused-argument
    def _after_cur_exec(self, conn, cursor, statement, *args):
        span = self.cursor_mapping.get(cursor, None)
        if span is None:
            return

        span.end()

    def _handle_error(self, context):
        span = self.current_thread_span
        if span is None:
            return

        try:
            if span.is_recording():
                span.set_status(
                    Status(StatusCode.ERROR, str(context.original_exception),)
                )
        finally:
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
        # pylint: disable=import-outside-toplevel
        from psycopg2.extensions import parse_dsn

        if hasattr(cursor, "connection") and hasattr(cursor.connection, "dsn"):
            dsn = getattr(cursor.connection, "dsn", None)
            if dsn:
                data = parse_dsn(dsn)
                attrs[SpanAttributes.DB_NAME] = data.get("dbname")
                attrs[SpanAttributes.NET_PEER_NAME] = data.get("host")
                attrs[SpanAttributes.NET_PEER_PORT] = int(data.get("port"))
    return attrs
