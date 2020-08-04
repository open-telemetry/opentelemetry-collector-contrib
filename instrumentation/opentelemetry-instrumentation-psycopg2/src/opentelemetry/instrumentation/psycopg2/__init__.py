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
The integration with PostgreSQL supports the `Psycopg`_ library, it can be enabled by
using ``Psycopg2Instrumentor``.

.. _Psycopg: http://initd.org/psycopg/

Usage
-----

.. code-block:: python

    import psycopg2
    from opentelemetry import trace
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.instrumentation.psycopg2 import Psycopg2Instrumentor

    trace.set_tracer_provider(TracerProvider())

    Psycopg2Instrumentor().instrument()

    cnx = psycopg2.connect(database='Database')
    cursor = cnx.cursor()
    cursor.execute("INSERT INTO test (testField) VALUES (123)")
    cursor.close()
    cnx.close()

API
---
"""

import psycopg2

from opentelemetry.instrumentation import dbapi
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.psycopg2.version import __version__
from opentelemetry.trace import get_tracer


class Psycopg2Instrumentor(BaseInstrumentor):
    _CONNECTION_ATTRIBUTES = {
        "database": "info.dbname",
        "port": "info.port",
        "host": "info.host",
        "user": "info.user",
    }

    _DATABASE_COMPONENT = "postgresql"
    _DATABASE_TYPE = "sql"

    def _instrument(self, **kwargs):
        """Integrate with PostgreSQL Psycopg library.
           Psycopg: http://initd.org/psycopg/
        """

        tracer_provider = kwargs.get("tracer_provider")

        dbapi.wrap_connect(
            __name__,
            psycopg2,
            "connect",
            self._DATABASE_COMPONENT,
            self._DATABASE_TYPE,
            self._CONNECTION_ATTRIBUTES,
            version=__version__,
            tracer_provider=tracer_provider,
        )

    def _uninstrument(self, **kwargs):
        """"Disable Psycopg2 instrumentation"""
        dbapi.unwrap_connect(psycopg2, "connect")

    # pylint:disable=no-self-use
    def instrument_connection(self, connection):
        """Enable instrumentation in a Psycopg2 connection.

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
        """Disable instrumentation in a Psycopg2 connection.

        Args:
            connection: The connection to uninstrument.

        Returns:
            An uninstrumented connection.
        """
        return dbapi.uninstrument_connection(connection)
