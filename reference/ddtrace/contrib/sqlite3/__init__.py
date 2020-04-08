"""Instrument sqlite3 to report SQLite queries.

``patch_all`` will automatically patch your sqlite3 connection to make it work.
::

    from ddtrace import Pin, patch
    import sqlite3

    # If not patched yet, you can patch sqlite3 specifically
    patch(sqlite3=True)

    # This will report a span with the default settings
    db = sqlite3.connect(":memory:")
    cursor = db.cursor()
    cursor.execute("select * from users where id = 1")

    # Use a pin to specify metadata related to this connection
    Pin.override(db, service='sqlite-users')
"""
from .connection import connection_factory
from .patch import patch

__all__ = ['connection_factory', 'patch']
