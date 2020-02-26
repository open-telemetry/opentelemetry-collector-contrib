from django.db import connections

# project
from ...ext import sql as sqlx
from ...internal.logger import get_logger
from ...pin import Pin

from .conf import settings
from ..dbapi import TracedCursor as DbApiTracedCursor

log = get_logger(__name__)

CURSOR_ATTR = '_datadog_original_cursor'
ALL_CONNS_ATTR = '_datadog_original_connections_all'


def patch_db(tracer):
    if hasattr(connections, ALL_CONNS_ATTR):
        log.debug('db already patched')
        return
    setattr(connections, ALL_CONNS_ATTR, connections.all)

    def all_connections(self):
        conns = getattr(self, ALL_CONNS_ATTR)()
        for conn in conns:
            patch_conn(tracer, conn)
        return conns

    connections.all = all_connections.__get__(connections, type(connections))


def unpatch_db():
    for c in connections.all():
        unpatch_conn(c)

    all_connections = getattr(connections, ALL_CONNS_ATTR, None)
    if all_connections is None:
        log.debug('nothing to do, the db is not patched')
        return
    connections.all = all_connections
    delattr(connections, ALL_CONNS_ATTR)


def patch_conn(tracer, conn):
    if hasattr(conn, CURSOR_ATTR):
        return

    setattr(conn, CURSOR_ATTR, conn.cursor)

    def cursor():
        database_prefix = (
            '{}-'.format(settings.DEFAULT_DATABASE_PREFIX)
            if settings.DEFAULT_DATABASE_PREFIX else ''
        )
        alias = getattr(conn, 'alias', 'default')
        service = '{}{}{}'.format(database_prefix, alias, 'db')
        vendor = getattr(conn, 'vendor', 'db')
        prefix = sqlx.normalize_vendor(vendor)
        tags = {
            'django.db.vendor': vendor,
            'django.db.alias': alias,
        }

        pin = Pin(service, tags=tags, tracer=tracer, app=prefix)
        return DbApiTracedCursor(conn._datadog_original_cursor(), pin)

    conn.cursor = cursor


def unpatch_conn(conn):
    cursor = getattr(conn, CURSOR_ATTR, None)
    if cursor is None:
        log.debug('nothing to do, the connection is not patched')
        return
    conn.cursor = cursor
    delattr(conn, CURSOR_ATTR)
