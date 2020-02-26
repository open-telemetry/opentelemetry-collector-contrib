"""
To trace sqlalchemy queries, add instrumentation to the engine class or
instance you are using::

    from ddtrace import tracer
    from ddtrace.contrib.sqlalchemy import trace_engine
    from sqlalchemy import create_engine

    engine = create_engine('sqlite:///:memory:')
    trace_engine(engine, tracer, 'my-database')

    engine.connect().execute('select count(*) from users')
"""
# 3p
from sqlalchemy.event import listen

# project
import ddtrace

from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...ext import SpanTypes, sql as sqlx, net as netx
from ...pin import Pin
from ...settings import config


def trace_engine(engine, tracer=None, service=None):
    """
    Add tracing instrumentation to the given sqlalchemy engine or instance.

    :param sqlalchemy.Engine engine: a SQLAlchemy engine class or instance
    :param ddtrace.Tracer tracer: a tracer instance. will default to the global
    :param str service: the name of the service to trace.
    """
    tracer = tracer or ddtrace.tracer  # by default use global
    EngineTracer(tracer, service, engine)


def _wrap_create_engine(func, module, args, kwargs):
    """Trace the SQLAlchemy engine, creating an `EngineTracer`
    object that will listen to SQLAlchemy events. A PIN object
    is attached to the engine instance so that it can be
    used later.
    """
    # the service name is set to `None` so that the engine
    # name is used by default; users can update this setting
    # using the PIN object
    engine = func(*args, **kwargs)
    EngineTracer(ddtrace.tracer, None, engine)
    return engine


class EngineTracer(object):

    def __init__(self, tracer, service, engine):
        self.tracer = tracer
        self.engine = engine
        self.vendor = sqlx.normalize_vendor(engine.name)
        self.service = service or self.vendor
        self.name = '%s.query' % self.vendor

        # attach the PIN
        Pin(
            app=self.vendor,
            tracer=tracer,
            service=self.service
        ).onto(engine)

        listen(engine, 'before_cursor_execute', self._before_cur_exec)
        listen(engine, 'after_cursor_execute', self._after_cur_exec)
        listen(engine, 'dbapi_error', self._dbapi_error)

    def _before_cur_exec(self, conn, cursor, statement, *args):
        pin = Pin.get_from(self.engine)
        if not pin or not pin.enabled():
            # don't trace the execution
            return

        span = pin.tracer.trace(
            self.name,
            service=pin.service,
            span_type=SpanTypes.SQL,
            resource=statement,
        )

        if not _set_tags_from_url(span, conn.engine.url):
            _set_tags_from_cursor(span, self.vendor, cursor)

        # set analytics sample rate
        sample_rate = config.sqlalchemy.get_analytics_sample_rate()
        if sample_rate is not None:
            span.set_tag(ANALYTICS_SAMPLE_RATE_KEY, sample_rate)

    def _after_cur_exec(self, conn, cursor, statement, *args):
        pin = Pin.get_from(self.engine)
        if not pin or not pin.enabled():
            # don't trace the execution
            return

        span = pin.tracer.current_span()
        if not span:
            return

        try:
            if cursor and cursor.rowcount >= 0:
                span.set_tag(sqlx.ROWS, cursor.rowcount)
        finally:
            span.finish()

    def _dbapi_error(self, conn, cursor, statement, *args):
        pin = Pin.get_from(self.engine)
        if not pin or not pin.enabled():
            # don't trace the execution
            return

        span = pin.tracer.current_span()
        if not span:
            return

        try:
            span.set_traceback()
        finally:
            span.finish()


def _set_tags_from_url(span, url):
    """ set connection tags from the url. return true if successful. """
    if url.host:
        span.set_tag(netx.TARGET_HOST, url.host)
    if url.port:
        span.set_tag(netx.TARGET_PORT, url.port)
    if url.database:
        span.set_tag(sqlx.DB, url.database)

    return bool(span.get_tag(netx.TARGET_HOST))


def _set_tags_from_cursor(span, vendor, cursor):
    """ attempt to set db connection tags by introspecting the cursor. """
    if 'postgres' == vendor:
        if hasattr(cursor, 'connection') and hasattr(cursor.connection, 'dsn'):
            dsn = getattr(cursor.connection, 'dsn', None)
            if dsn:
                d = sqlx.parse_pg_dsn(dsn)
                span.set_tag(sqlx.DB, d.get('dbname'))
                span.set_tag(netx.TARGET_HOST, d.get('host'))
                span.set_tag(netx.TARGET_PORT, d.get('port'))
