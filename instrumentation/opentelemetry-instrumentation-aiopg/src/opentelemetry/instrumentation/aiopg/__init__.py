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
The integration with PostgreSQL supports the aiopg library,
it can be enabled by using ``AiopgInstrumentor``.

.. aiopg: https://github.com/aio-libs/aiopg

Usage
-----

.. code-block:: python

    import aiopg
    from opentelemetry.instrumentation.aiopg import AiopgInstrumentor

    AiopgInstrumentor().instrument()

    cnx = await aiopg.connect(database='Database')
    cursor = await cnx.cursor()
    await cursor.execute("INSERT INTO test (testField) VALUES (123)")
    cursor.close()
    cnx.close()

    pool = await aiopg.create_pool(database='Database')
    cnx = await pool.acquire()
    cursor = await cnx.cursor()
    await cursor.execute("INSERT INTO test (testField) VALUES (123)")
    cursor.close()
    cnx.close()

API
---
"""

from typing import Collection

from opentelemetry.instrumentation.aiopg import wrappers
from opentelemetry.instrumentation.aiopg.package import _instruments
from opentelemetry.instrumentation.aiopg.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor


class AiopgInstrumentor(BaseInstrumentor):
    _CONNECTION_ATTRIBUTES = {
        "database": "info.dbname",
        "port": "info.port",
        "host": "info.host",
        "user": "info.user",
    }

    _DATABASE_SYSTEM = "postgresql"

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Integrate with PostgreSQL aiopg library.
        aiopg: https://github.com/aio-libs/aiopg
        """

        tracer_provider = kwargs.get("tracer_provider")

        wrappers.wrap_connect(
            __name__,
            self._DATABASE_SYSTEM,
            self._CONNECTION_ATTRIBUTES,
            version=__version__,
            tracer_provider=tracer_provider,
        )

        wrappers.wrap_create_pool(
            __name__,
            self._DATABASE_SYSTEM,
            self._CONNECTION_ATTRIBUTES,
            version=__version__,
            tracer_provider=tracer_provider,
        )

    # pylint:disable=no-self-use
    def _uninstrument(self, **kwargs):
        """ "Disable aiopg instrumentation"""
        wrappers.unwrap_connect()
        wrappers.unwrap_create_pool()

    # pylint:disable=no-self-use
    def instrument_connection(self, connection, tracer_provider=None):
        """Enable instrumentation in a aiopg connection.

        Args:
            connection: The connection to instrument.
            tracer_provider: The optional tracer provider to use. If omitted
                the current globally configured one is used.

        Returns:
            An instrumented connection.
        """
        return wrappers.instrument_connection(
            __name__,
            connection,
            self._DATABASE_SYSTEM,
            self._CONNECTION_ATTRIBUTES,
            version=__version__,
            tracer_provider=tracer_provider,
        )

    def uninstrument_connection(self, connection):
        """Disable instrumentation in a aiopg connection.

        Args:
            connection: The connection to uninstrument.

        Returns:
            An uninstrumented connection.
        """
        return wrappers.uninstrument_connection(connection)
