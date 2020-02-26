import pymysql.connections

from ...utils.deprecation import deprecated


@deprecated(message='Use patching instead (see the docs).', version='1.0.0')
def get_traced_pymysql_connection(*args, **kwargs):
    return pymysql.connections.Connection
