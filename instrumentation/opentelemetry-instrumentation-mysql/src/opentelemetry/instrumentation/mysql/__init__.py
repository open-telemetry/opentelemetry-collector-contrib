# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
MySQL instrumentation supporting `mysql-connector`_, it can be enabled by
using ``MySQLInstrumentor``.

.. _mysql-connector: https://pypi.org/project/mysql-connector/

Usage
-----

.. code:: python

    import mysql.connector
    from opentelemetry.instrumentation.mysql import MySQLInstrumentor

    MySQLInstrumentor().instrument()

    cnx = mysql.connector.connect(database="MySQL_Database")
    cursor = cnx.cursor()
    cursor.execute("INSERT INTO test (testField) VALUES (123)")
    cursor.close()
    cnx.close()

API
---
"""

from typing import Collection

import mysql.connector

from opentelemetry.instrumentation import dbapi
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.mysql.package import _instruments
from opentelemetry.instrumentation.mysql.version import __version__


class MySQLInstrumentor(BaseInstrumentor):
    _CONNECTION_ATTRIBUTES = {
        "database": "database",
        "port": "server_port",
        "host": "server_host",
        "user": "user",
    }

    _DATABASE_SYSTEM = "mysql"

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Integrate with MySQL Connector/Python library.
        https://dev.mysql.com/doc/connector-python/en/
        """
        tracer_provider = kwargs.get("tracer_provider")

        dbapi.wrap_connect(
            __name__,
            mysql.connector,
            "connect",
            self._DATABASE_SYSTEM,
            self._CONNECTION_ATTRIBUTES,
            version=__version__,
            tracer_provider=tracer_provider,
        )

    def _uninstrument(self, **kwargs):
        """ "Disable MySQL instrumentation"""
        dbapi.unwrap_connect(mysql.connector, "connect")

    # pylint:disable=no-self-use
    def instrument_connection(self, connection, tracer_provider=None):
        """Enable instrumentation in a MySQL connection.

        Args:
            connection: The connection to instrument.
            tracer_provider: The optional tracer provider to use. If omitted
                the current globally configured one is used.

        Returns:
            An instrumented connection.
        """
        return dbapi.instrument_connection(
            __name__,
            connection,
            self._DATABASE_SYSTEM,
            self._CONNECTION_ATTRIBUTES,
            version=__version__,
            tracer_provider=tracer_provider,
        )

    def uninstrument_connection(self, connection):
        """Disable instrumentation in a MySQL connection.

        Args:
            connection: The connection to uninstrument.

        Returns:
            An uninstrumented connection.
        """
        return dbapi.uninstrument_connection(connection)
