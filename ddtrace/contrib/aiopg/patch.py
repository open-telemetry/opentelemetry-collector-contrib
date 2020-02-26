# 3p
import asyncio

import aiopg.connection
import psycopg2.extensions
from ddtrace.vendor import wrapt

from .connection import AIOTracedConnection
from ..psycopg.patch import _patch_extensions, \
    _unpatch_extensions, patch_conn as psycopg_patch_conn
from ...utils.wrappers import unwrap as _u


def patch():
    """ Patch monkey patches psycopg's connection function
        so that the connection's functions are traced.
    """
    if getattr(aiopg, '_datadog_patch', False):
        return
    setattr(aiopg, '_datadog_patch', True)

    wrapt.wrap_function_wrapper(aiopg.connection, '_connect', patched_connect)
    _patch_extensions(_aiopg_extensions)  # do this early just in case


def unpatch():
    if getattr(aiopg, '_datadog_patch', False):
        setattr(aiopg, '_datadog_patch', False)
        _u(aiopg.connection, '_connect')
        _unpatch_extensions(_aiopg_extensions)


@asyncio.coroutine
def patched_connect(connect_func, _, args, kwargs):
    conn = yield from connect_func(*args, **kwargs)
    return psycopg_patch_conn(conn, traced_conn_cls=AIOTracedConnection)


def _extensions_register_type(func, _, args, kwargs):
    def _unroll_args(obj, scope=None):
        return obj, scope
    obj, scope = _unroll_args(*args, **kwargs)

    # register_type performs a c-level check of the object
    # type so we must be sure to pass in the actual db connection
    if scope and isinstance(scope, wrapt.ObjectProxy):
        scope = scope.__wrapped__._conn

    return func(obj, scope) if scope else func(obj)


# extension hooks
_aiopg_extensions = [
    (psycopg2.extensions.register_type,
     psycopg2.extensions, 'register_type',
     _extensions_register_type),
]
