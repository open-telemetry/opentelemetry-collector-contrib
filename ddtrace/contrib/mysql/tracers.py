import mysql.connector

from ...utils.deprecation import deprecated


@deprecated(message='Use patching instead (see the docs).', version='1.0.0')
def get_traced_mysql_connection(*args, **kwargs):
    return mysql.connector.MySQLConnection
