"""Instrument psycopg2 to report Postgres queries.

``patch_all`` will automatically patch your psycopg2 connection to make it work.
::

    from ddtrace import Pin, patch
    import psycopg2

    # If not patched yet, you can patch psycopg2 specifically
    patch(psycopg=True)

    # This will report a span with the default settings
    db = psycopg2.connect(connection_factory=factory)
    cursor = db.cursor()
    cursor.execute("select * from users where id = 1")

    # Use a pin to specify metadata related to this connection
    Pin.override(db, service='postgres-users')
"""
from ...utils.importlib import require_modules


required_modules = ['psycopg2']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .connection import connection_factory
        from .patch import patch, patch_conn

        __all__ = ['connection_factory', 'patch', 'patch_conn']
