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
The trace integration with aiopg based on dbapi integration,
where replaced sync wrap methods to async

Usage
-----

.. code-block:: python

    from opentelemetry import trace
    from opentelemetry.instrumentation.aiopg import trace_integration
    from opentelemetry.trace import TracerProvider

    trace.set_tracer_provider(TracerProvider())

    trace_integration(aiopg.connection, "_connect", "postgresql", "sql")

API
---
"""
import logging
import typing

import aiopg
import wrapt

from opentelemetry.instrumentation.aiopg.aiopg_integration import (
    AiopgIntegration,
    get_traced_connection_proxy,
)
from opentelemetry.instrumentation.aiopg.version import __version__
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.trace import TracerProvider

logger = logging.getLogger(__name__)


def trace_integration(
    database_component: str,
    database_type: str = "",
    connection_attributes: typing.Dict = None,
    tracer_provider: typing.Optional[TracerProvider] = None,
):
    """Integrate with aiopg library.
    based on dbapi integration, where replaced sync wrap methods to async

    Args:
        database_component: Database driver name or
            database name "postgreSQL".
        database_type: The Database type. For any SQL database, "sql".
        connection_attributes: Attribute names for database, port, host and
            user in Connection object.
        tracer_provider: The :class:`opentelemetry.trace.TracerProvider` to
            use. If ommited the current configured one is used.
    """

    wrap_connect(
        __name__,
        database_component,
        database_type,
        connection_attributes,
        __version__,
        tracer_provider,
    )


def wrap_connect(
    name: str,
    database_component: str,
    database_type: str = "",
    connection_attributes: typing.Dict = None,
    version: str = "",
    tracer_provider: typing.Optional[TracerProvider] = None,
):
    """Integrate with aiopg library.
    https://github.com/aio-libs/aiopg

    Args:
        name: Name of opentelemetry extension for aiopg.
        database_component: Database driver name
            or database name "postgreSQL".
        database_type: The Database type. For any SQL database, "sql".
        connection_attributes: Attribute names for database, port, host and
            user in Connection object.
        version: Version of opentelemetry extension for aiopg.
        tracer_provider: The :class:`opentelemetry.trace.TracerProvider` to
            use. If ommited the current configured one is used.
    """

    # pylint: disable=unused-argument
    async def wrap_connect_(
        wrapped: typing.Callable[..., typing.Any],
        instance: typing.Any,
        args: typing.Tuple[typing.Any, typing.Any],
        kwargs: typing.Dict[typing.Any, typing.Any],
    ):
        db_integration = AiopgIntegration(
            name,
            database_component,
            database_type=database_type,
            connection_attributes=connection_attributes,
            version=version,
            tracer_provider=tracer_provider,
        )
        return await db_integration.wrapped_connection(wrapped, args, kwargs)

    try:
        wrapt.wrap_function_wrapper(aiopg, "connect", wrap_connect_)
    except Exception as ex:  # pylint: disable=broad-except
        logger.warning("Failed to integrate with aiopg. %s", str(ex))


def unwrap_connect():
    """Disable integration with aiopg library.
    https://github.com/aio-libs/aiopg
    """

    unwrap(aiopg, "connect")


def instrument_connection(
    name: str,
    connection,
    database_component: str,
    database_type: str = "",
    connection_attributes: typing.Dict = None,
    version: str = "",
    tracer_provider: typing.Optional[TracerProvider] = None,
):
    """Enable instrumentation in a database connection.

    Args:
        name: Name of opentelemetry extension for aiopg.
        connection: The connection to instrument.
        database_component: Database driver name or database name "postgreSQL".
        database_type: The Database type. For any SQL database, "sql".
        connection_attributes: Attribute names for database, port, host and
            user in a connection object.
        version: Version of opentelemetry extension for aiopg.
        tracer_provider: The :class:`opentelemetry.trace.TracerProvider` to
            use. If ommited the current configured one is used.

    Returns:
        An instrumented connection.
    """
    db_integration = AiopgIntegration(
        name,
        database_component,
        database_type,
        connection_attributes=connection_attributes,
        version=version,
        tracer_provider=tracer_provider,
    )
    db_integration.get_connection_attributes(connection)
    return get_traced_connection_proxy(connection, db_integration)


def uninstrument_connection(connection):
    """Disable instrumentation in a database connection.

    Args:
        connection: The connection to uninstrument.

    Returns:
        An uninstrumented connection.
    """
    if isinstance(connection, wrapt.ObjectProxy):
        return connection.__wrapped__

    logger.warning("Connection is not instrumented")
    return connection


def wrap_create_pool(
    name: str,
    database_component: str,
    database_type: str = "",
    connection_attributes: typing.Dict = None,
    version: str = "",
    tracer_provider: typing.Optional[TracerProvider] = None,
):
    # pylint: disable=unused-argument
    async def wrap_create_pool_(
        wrapped: typing.Callable[..., typing.Any],
        instance: typing.Any,
        args: typing.Tuple[typing.Any, typing.Any],
        kwargs: typing.Dict[typing.Any, typing.Any],
    ):
        db_integration = AiopgIntegration(
            name,
            database_component,
            database_type,
            connection_attributes=connection_attributes,
            version=version,
            tracer_provider=tracer_provider,
        )
        return await db_integration.wrapped_pool(wrapped, args, kwargs)

    try:
        wrapt.wrap_function_wrapper(aiopg, "create_pool", wrap_create_pool_)
    except Exception as ex:  # pylint: disable=broad-except
        logger.warning("Failed to integrate with DB API. %s", str(ex))


def unwrap_create_pool():
    """Disable integration with aiopg library.
    https://github.com/aio-libs/aiopg
    """
    unwrap(aiopg, "create_pool")
