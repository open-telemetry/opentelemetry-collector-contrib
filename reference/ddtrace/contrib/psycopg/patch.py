# 3p
import psycopg2
from ddtrace.vendor import wrapt

# project
from ddtrace import Pin, config
from ddtrace.contrib import dbapi
from ddtrace.ext import sql, net, db

# Original connect method
_connect = psycopg2.connect

# psycopg2 versions can end in `-betaN` where `N` is a number
# in such cases we simply skip version specific patching
PSYCOPG2_VERSION = (0, 0, 0)

try:
    PSYCOPG2_VERSION = tuple(map(int, psycopg2.__version__.split()[0].split('.')))
except Exception:
    pass

if PSYCOPG2_VERSION >= (2, 7):
    from psycopg2.sql import Composable


def patch():
    """ Patch monkey patches psycopg's connection function
        so that the connection's functions are traced.
    """
    if getattr(psycopg2, '_datadog_patch', False):
        return
    setattr(psycopg2, '_datadog_patch', True)

    wrapt.wrap_function_wrapper(psycopg2, 'connect', patched_connect)
    _patch_extensions(_psycopg2_extensions)  # do this early just in case


def unpatch():
    if getattr(psycopg2, '_datadog_patch', False):
        setattr(psycopg2, '_datadog_patch', False)
        psycopg2.connect = _connect


class Psycopg2TracedCursor(dbapi.TracedCursor):
    """ TracedCursor for psycopg2 """
    def _trace_method(self, method, name, resource, extra_tags, *args, **kwargs):
        # treat psycopg2.sql.Composable resource objects as strings
        if PSYCOPG2_VERSION >= (2, 7) and isinstance(resource, Composable):
            resource = resource.as_string(self.__wrapped__)

        return super(Psycopg2TracedCursor, self)._trace_method(method, name, resource, extra_tags, *args, **kwargs)


class Psycopg2FetchTracedCursor(Psycopg2TracedCursor, dbapi.FetchTracedCursor):
    """ FetchTracedCursor for psycopg2 """


class Psycopg2TracedConnection(dbapi.TracedConnection):
    """ TracedConnection wraps a Connection with tracing code. """

    def __init__(self, conn, pin=None, cursor_cls=None):
        if not cursor_cls:
            # Do not trace `fetch*` methods by default
            cursor_cls = Psycopg2TracedCursor
            if config.dbapi2.trace_fetch_methods:
                cursor_cls = Psycopg2FetchTracedCursor

        super(Psycopg2TracedConnection, self).__init__(conn, pin, cursor_cls=cursor_cls)


def patch_conn(conn, traced_conn_cls=Psycopg2TracedConnection):
    """ Wrap will patch the instance so that its queries are traced."""
    # ensure we've patched extensions (this is idempotent) in
    # case we're only tracing some connections.
    _patch_extensions(_psycopg2_extensions)

    c = traced_conn_cls(conn)

    # fetch tags from the dsn
    dsn = sql.parse_pg_dsn(conn.dsn)
    tags = {
        net.TARGET_HOST: dsn.get('host'),
        net.TARGET_PORT: dsn.get('port'),
        db.NAME: dsn.get('dbname'),
        db.USER: dsn.get('user'),
        'db.application': dsn.get('application_name'),
    }

    Pin(
        service='postgres',
        app='postgres',
        tags=tags).onto(c)

    return c


def _patch_extensions(_extensions):
    # we must patch extensions all the time (it's pretty harmless) so split
    # from global patching of connections. must be idempotent.
    for _, module, func, wrapper in _extensions:
        if not hasattr(module, func) or isinstance(getattr(module, func), wrapt.ObjectProxy):
            continue
        wrapt.wrap_function_wrapper(module, func, wrapper)


def _unpatch_extensions(_extensions):
    # we must patch extensions all the time (it's pretty harmless) so split
    # from global patching of connections. must be idempotent.
    for original, module, func, _ in _extensions:
        setattr(module, func, original)


#
# monkeypatch targets
#

def patched_connect(connect_func, _, args, kwargs):
    conn = connect_func(*args, **kwargs)
    return patch_conn(conn)


def _extensions_register_type(func, _, args, kwargs):
    def _unroll_args(obj, scope=None):
        return obj, scope
    obj, scope = _unroll_args(*args, **kwargs)

    # register_type performs a c-level check of the object
    # type so we must be sure to pass in the actual db connection
    if scope and isinstance(scope, wrapt.ObjectProxy):
        scope = scope.__wrapped__

    return func(obj, scope) if scope else func(obj)


def _extensions_quote_ident(func, _, args, kwargs):
    def _unroll_args(obj, scope=None):
        return obj, scope
    obj, scope = _unroll_args(*args, **kwargs)

    # register_type performs a c-level check of the object
    # type so we must be sure to pass in the actual db connection
    if scope and isinstance(scope, wrapt.ObjectProxy):
        scope = scope.__wrapped__

    return func(obj, scope) if scope else func(obj)


def _extensions_adapt(func, _, args, kwargs):
    adapt = func(*args, **kwargs)
    if hasattr(adapt, 'prepare'):
        return AdapterWrapper(adapt)
    return adapt


class AdapterWrapper(wrapt.ObjectProxy):
    def prepare(self, *args, **kwargs):
        func = self.__wrapped__.prepare
        if not args:
            return func(*args, **kwargs)
        conn = args[0]

        # prepare performs a c-level check of the object type so
        # we must be sure to pass in the actual db connection
        if isinstance(conn, wrapt.ObjectProxy):
            conn = conn.__wrapped__

        return func(conn, *args[1:], **kwargs)


# extension hooks
_psycopg2_extensions = [
    (psycopg2.extensions.register_type,
     psycopg2.extensions, 'register_type',
     _extensions_register_type),
    (psycopg2._psycopg.register_type,
     psycopg2._psycopg, 'register_type',
     _extensions_register_type),
    (psycopg2.extensions.adapt,
     psycopg2.extensions, 'adapt',
     _extensions_adapt),
]

# `_json` attribute is only available for psycopg >= 2.5
if getattr(psycopg2, '_json', None):
    _psycopg2_extensions += [
        (psycopg2._json.register_type,
         psycopg2._json, 'register_type',
         _extensions_register_type),
    ]

# `quote_ident` attribute is only available for psycopg >= 2.7
if getattr(psycopg2, 'extensions', None) and getattr(psycopg2.extensions,
                                                     'quote_ident', None):
    _psycopg2_extensions += [
        (psycopg2.extensions.quote_ident, psycopg2.extensions, 'quote_ident', _extensions_quote_ident),
    ]
