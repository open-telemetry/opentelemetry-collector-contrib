"""Instrument pymysql to report MySQL queries.

``patch_all`` will automatically patch your pymysql connection to make it work.
::

    from ddtrace import Pin, patch
    from pymysql import connect

    # If not patched yet, you can patch pymysql specifically
    patch(pymysql=True)

    # This will report a span with the default settings
    conn = connect(user="alice", password="b0b", host="localhost", port=3306, database="test")
    cursor = conn.cursor()
    cursor.execute("SELECT 6*7 AS the_answer;")

    # Use a pin to specify metadata related to this connection
    Pin.override(conn, service='pymysql-users')
"""

from ...utils.importlib import require_modules


required_modules = ['pymysql']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch
        from .tracers import get_traced_pymysql_connection

        __all__ = ['get_traced_pymysql_connection', 'patch']
