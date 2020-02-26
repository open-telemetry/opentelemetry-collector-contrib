"""
Generic dbapi tracing code.
"""

from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...ext import SpanTypes, sql
from ...internal.logger import get_logger
from ...pin import Pin
from ...settings import config
from ...utils.formats import asbool, get_env
from ...vendor import wrapt


log = get_logger(__name__)

config._add('dbapi2', dict(
    trace_fetch_methods=asbool(get_env('dbapi2', 'trace_fetch_methods', 'false')),
))


class TracedCursor(wrapt.ObjectProxy):
    """ TracedCursor wraps a psql cursor and traces it's queries. """

    def __init__(self, cursor, pin):
        super(TracedCursor, self).__init__(cursor)
        pin.onto(self)
        name = pin.app or 'sql'
        self._self_datadog_name = '{}.query'.format(name)
        self._self_last_execute_operation = None

    def _trace_method(self, method, name, resource, extra_tags, *args, **kwargs):
        """
        Internal function to trace the call to the underlying cursor method
        :param method: The callable to be wrapped
        :param name: The name of the resulting span.
        :param resource: The sql query. Sql queries are obfuscated on the agent side.
        :param extra_tags: A dict of tags to store into the span's meta
        :param args: The args that will be passed as positional args to the wrapped method
        :param kwargs: The args that will be passed as kwargs to the wrapped method
        :return: The result of the wrapped method invocation
        """
        pin = Pin.get_from(self)
        if not pin or not pin.enabled():
            return method(*args, **kwargs)
        service = pin.service
        with pin.tracer.trace(name, service=service, resource=resource, span_type=SpanTypes.SQL) as s:
            # No reason to tag the query since it is set as the resource by the agent. See:
            # https://github.com/DataDog/datadog-trace-agent/blob/bda1ebbf170dd8c5879be993bdd4dbae70d10fda/obfuscate/sql.go#L232
            s.set_tags(pin.tags)
            s.set_tags(extra_tags)

            # set analytics sample rate if enabled but only for non-FetchTracedCursor
            if not isinstance(self, FetchTracedCursor):
                s.set_tag(
                    ANALYTICS_SAMPLE_RATE_KEY,
                    config.dbapi2.get_analytics_sample_rate()
                )

            try:
                return method(*args, **kwargs)
            finally:
                row_count = self.__wrapped__.rowcount
                s.set_metric('db.rowcount', row_count)
                # Necessary for django integration backward compatibility. Django integration used to provide its own
                # implementation of the TracedCursor, which used to store the row count into a tag instead of
                # as a metric. Such custom implementation has been replaced by this generic dbapi implementation and
                # this tag has been added since.
                if row_count and row_count >= 0:
                    s.set_tag(sql.ROWS, row_count)

    def executemany(self, query, *args, **kwargs):
        """ Wraps the cursor.executemany method"""
        self._self_last_execute_operation = query
        # Always return the result as-is
        # DEV: Some libraries return `None`, others `int`, and others the cursor objects
        #      These differences should be overriden at the integration specific layer (e.g. in `sqlite3/patch.py`)
        # FIXME[matt] properly handle kwargs here. arg names can be different
        # with different libs.
        return self._trace_method(
            self.__wrapped__.executemany, self._self_datadog_name, query, {'sql.executemany': 'true'},
            query, *args, **kwargs)

    def execute(self, query, *args, **kwargs):
        """ Wraps the cursor.execute method"""
        self._self_last_execute_operation = query

        # Always return the result as-is
        # DEV: Some libraries return `None`, others `int`, and others the cursor objects
        #      These differences should be overriden at the integration specific layer (e.g. in `sqlite3/patch.py`)
        return self._trace_method(self.__wrapped__.execute, self._self_datadog_name, query, {}, query, *args, **kwargs)

    def callproc(self, proc, args):
        """ Wraps the cursor.callproc method"""
        self._self_last_execute_operation = proc
        return self._trace_method(self.__wrapped__.callproc, self._self_datadog_name, proc, {}, proc, args)

    def __enter__(self):
        # previous versions of the dbapi didn't support context managers. let's
        # reference the func that would be called to ensure that errors
        # messages will be the same.
        self.__wrapped__.__enter__

        # and finally, yield the traced cursor.
        return self


class FetchTracedCursor(TracedCursor):
    """
    Sub-class of :class:`TracedCursor` that also instruments `fetchone`, `fetchall`, and `fetchmany` methods.

    We do not trace these functions by default since they can get very noisy (e.g. `fetchone` with 100k rows).
    """
    def fetchone(self, *args, **kwargs):
        """ Wraps the cursor.fetchone method"""
        span_name = '{}.{}'.format(self._self_datadog_name, 'fetchone')
        return self._trace_method(self.__wrapped__.fetchone, span_name, self._self_last_execute_operation, {},
                                  *args, **kwargs)

    def fetchall(self, *args, **kwargs):
        """ Wraps the cursor.fetchall method"""
        span_name = '{}.{}'.format(self._self_datadog_name, 'fetchall')
        return self._trace_method(self.__wrapped__.fetchall, span_name, self._self_last_execute_operation, {},
                                  *args, **kwargs)

    def fetchmany(self, *args, **kwargs):
        """ Wraps the cursor.fetchmany method"""
        span_name = '{}.{}'.format(self._self_datadog_name, 'fetchmany')
        # We want to trace the information about how many rows were requested. Note that this number may be larger
        # the number of rows actually returned if less then requested are available from the query.
        size_tag_key = 'db.fetch.size'
        if 'size' in kwargs:
            extra_tags = {size_tag_key: kwargs.get('size')}
        elif len(args) == 1 and isinstance(args[0], int):
            extra_tags = {size_tag_key: args[0]}
        else:
            default_array_size = getattr(self.__wrapped__, 'arraysize', None)
            extra_tags = {size_tag_key: default_array_size} if default_array_size else {}

        return self._trace_method(self.__wrapped__.fetchmany, span_name, self._self_last_execute_operation, extra_tags,
                                  *args, **kwargs)


class TracedConnection(wrapt.ObjectProxy):
    """ TracedConnection wraps a Connection with tracing code. """

    def __init__(self, conn, pin=None, cursor_cls=None):
        # Set default cursor class if one was not provided
        if not cursor_cls:
            # Do not trace `fetch*` methods by default
            cursor_cls = TracedCursor
            if config.dbapi2.trace_fetch_methods:
                cursor_cls = FetchTracedCursor

        super(TracedConnection, self).__init__(conn)
        name = _get_vendor(conn)
        self._self_datadog_name = '{}.connection'.format(name)
        db_pin = pin or Pin(service=name, app=name)
        db_pin.onto(self)
        # wrapt requires prefix of `_self` for attributes that are only in the
        # proxy (since some of our source objects will use `__slots__`)
        self._self_cursor_cls = cursor_cls

    def _trace_method(self, method, name, extra_tags, *args, **kwargs):
        pin = Pin.get_from(self)
        if not pin or not pin.enabled():
            return method(*args, **kwargs)
        service = pin.service

        with pin.tracer.trace(name, service=service) as s:
            s.set_tags(pin.tags)
            s.set_tags(extra_tags)

            return method(*args, **kwargs)

    def cursor(self, *args, **kwargs):
        cursor = self.__wrapped__.cursor(*args, **kwargs)
        pin = Pin.get_from(self)
        if not pin:
            return cursor
        return self._self_cursor_cls(cursor, pin)

    def commit(self, *args, **kwargs):
        span_name = '{}.{}'.format(self._self_datadog_name, 'commit')
        return self._trace_method(self.__wrapped__.commit, span_name, {}, *args, **kwargs)

    def rollback(self, *args, **kwargs):
        span_name = '{}.{}'.format(self._self_datadog_name, 'rollback')
        return self._trace_method(self.__wrapped__.rollback, span_name, {}, *args, **kwargs)


def _get_vendor(conn):
    """ Return the vendor (e.g postgres, mysql) of the given
        database.
    """
    try:
        name = _get_module_name(conn)
    except Exception:
        log.debug('couldnt parse module name', exc_info=True)
        name = 'sql'
    return sql.normalize_vendor(name)


def _get_module_name(conn):
    return conn.__class__.__module__.split('.')[0]
