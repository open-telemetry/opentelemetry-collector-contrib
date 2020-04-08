from . import SpanTypes

# [TODO] Deprecated, remove when we remove AppTypes
TYPE = SpanTypes.SQL
APP_TYPE = SpanTypes.SQL

# tags
QUERY = 'sql.query'   # the query text
ROWS = 'sql.rows'     # number of rows returned by a query
DB = 'sql.db'         # the name of the database


def normalize_vendor(vendor):
    """ Return a canonical name for a type of database. """
    if not vendor:
        return 'db'  # should this ever happen?
    elif 'sqlite' in vendor:
        return 'sqlite'
    elif 'postgres' in vendor or vendor == 'psycopg2':
        return 'postgres'
    else:
        return vendor


def parse_pg_dsn(dsn):
    """
    Return a dictionary of the components of a postgres DSN.

    >>> parse_pg_dsn('user=dog port=1543 dbname=dogdata')
    {'user':'dog', 'port':'1543', 'dbname':'dogdata'}
    """
    # FIXME: replace by psycopg2.extensions.parse_dsn when available
    # https://github.com/psycopg/psycopg2/pull/321
    return {c.split('=')[0]: c.split('=')[1] for c in dsn.split() if '=' in c}
