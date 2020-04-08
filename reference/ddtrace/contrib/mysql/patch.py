# 3p
from ddtrace.vendor import wrapt
import mysql.connector

# project
from ddtrace import Pin
from ddtrace.contrib.dbapi import TracedConnection
from ...ext import net, db


CONN_ATTR_BY_TAG = {
    net.TARGET_HOST: 'server_host',
    net.TARGET_PORT: 'server_port',
    db.USER: 'user',
    db.NAME: 'database',
}


def patch():
    wrapt.wrap_function_wrapper('mysql.connector', 'connect', _connect)
    # `Connect` is an alias for `connect`, patch it too
    if hasattr(mysql.connector, 'Connect'):
        mysql.connector.Connect = mysql.connector.connect


def unpatch():
    if isinstance(mysql.connector.connect, wrapt.ObjectProxy):
        mysql.connector.connect = mysql.connector.connect.__wrapped__
        if hasattr(mysql.connector, 'Connect'):
            mysql.connector.Connect = mysql.connector.connect


def _connect(func, instance, args, kwargs):
    conn = func(*args, **kwargs)
    return patch_conn(conn)


def patch_conn(conn):

    tags = {t: getattr(conn, a) for t, a in CONN_ATTR_BY_TAG.items() if getattr(conn, a, '') != ''}
    pin = Pin(service='mysql', app='mysql', tags=tags)

    # grab the metadata from the conn
    wrapped = TracedConnection(conn, pin=pin)
    pin.onto(wrapped)
    return wrapped
