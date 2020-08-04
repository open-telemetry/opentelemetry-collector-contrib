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
SQLite instrumentation supporting `sqlite3`_, it can be enabled by
using ``SQLite3Instrumentor``.

.. _sqlite3: https://docs.python.org/3/library/sqlite3.html

Usage
-----

.. code:: python

    import sqlite3
    from opentelemetry import trace
    from opentelemetry.trace import TracerProvider
    from opentelemetry.instrumentation.sqlite3 import SQLite3Instrumentor

    trace.set_tracer_provider(TracerProvider())

    SQLite3Instrumentor().instrument()

    cnx = sqlite3.connect('example.db')
    cursor = cnx.cursor()
    cursor.execute("INSERT INTO test (testField) VALUES (123)")
    cursor.close()
    cnx.close()

API
---
"""

import sqlite3

from opentelemetry.instrumentation import dbapi
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.sqlite3.version import __version__
from opentelemetry.trace import get_tracer


class SQLite3Instrumentor(BaseInstrumentor):
    # No useful attributes of sqlite3 connection object
    _CONNECTION_ATTRIBUTES = {}

    _DATABASE_COMPONENT = "sqlite3"
    _DATABASE_TYPE = "sql"

    def _instrument(self, **kwargs):
        """Integrate with SQLite3 Python library.
        https://docs.python.org/3/library/sqlite3.html
        """
        tracer_provider = kwargs.get("tracer_provider")

        dbapi.wrap_connect(
            __name__,
            sqlite3,
            "connect",
            self._DATABASE_COMPONENT,
            self._DATABASE_TYPE,
            self._CONNECTION_ATTRIBUTES,
            version=__version__,
            tracer_provider=tracer_provider,
        )

    def _uninstrument(self, **kwargs):
        """"Disable SQLite3 instrumentation"""
        dbapi.unwrap_connect(sqlite3, "connect")

    # pylint:disable=no-self-use
    def instrument_connection(self, connection):
        """Enable instrumentation in a SQLite connection.

        Args:
            connection: The connection to instrument.

        Returns:
            An instrumented connection.
        """
        tracer = get_tracer(__name__, __version__)

        return dbapi.instrument_connection(
            tracer,
            connection,
            self._DATABASE_COMPONENT,
            self._DATABASE_TYPE,
            self._CONNECTION_ATTRIBUTES,
        )

    def uninstrument_connection(self, connection):
        """Disable instrumentation in a SQLite connection.

        Args:
            connection: The connection to uninstrument.

        Returns:
            An uninstrumented connection.
        """
        return dbapi.uninstrument_connection(connection)
