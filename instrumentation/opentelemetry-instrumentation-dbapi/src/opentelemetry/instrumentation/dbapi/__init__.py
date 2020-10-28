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
The trace integration with Database API supports libraries that follow the
Python Database API Specification v2.0.
`<https://www.python.org/dev/peps/pep-0249/>`_

Usage
-----

.. code-block:: python

    import mysql.connector
    import pyodbc

    from opentelemetry import trace
    from opentelemetry.instrumentation.dbapi import trace_integration
    from opentelemetry.trace import TracerProvider

    trace.set_tracer_provider(TracerProvider())

    # Ex: mysql.connector
    trace_integration(mysql.connector, "connect", "mysql", "sql")
    # Ex: pyodbc
    trace_integration(pyodbc, "Connection", "odbc", "sql")

API
---
"""

import functools
import logging
import typing

import wrapt

from opentelemetry import trace as trace_api
from opentelemetry.instrumentation.dbapi.version import __version__
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.trace import SpanKind, TracerProvider, get_tracer
from opentelemetry.trace.status import Status, StatusCode

logger = logging.getLogger(__name__)


def trace_integration(
    connect_module: typing.Callable[..., typing.Any],
    connect_method_name: str,
    database_component: str,
    database_type: str = "",
    connection_attributes: typing.Dict = None,
    tracer_provider: typing.Optional[TracerProvider] = None,
):
    """Integrate with DB API library.
        https://www.python.org/dev/peps/pep-0249/

        Args:
            connect_module: Module name where connect method is available.
            connect_method_name: The connect method name.
            database_component: Database driver name or database name "JDBI",
                "jdbc", "odbc", "postgreSQL".
            database_type: The Database type. For any SQL database, "sql".
            connection_attributes: Attribute names for database, port, host and
                user in Connection object.
            tracer_provider: The :class:`opentelemetry.trace.TracerProvider` to
                use. If ommited the current configured one is used.
    """
    wrap_connect(
        __name__,
        connect_module,
        connect_method_name,
        database_component,
        database_type,
        connection_attributes,
        version=__version__,
        tracer_provider=tracer_provider,
    )


def wrap_connect(
    name: str,
    connect_module: typing.Callable[..., typing.Any],
    connect_method_name: str,
    database_component: str,
    database_type: str = "",
    connection_attributes: typing.Dict = None,
    version: str = "",
    tracer_provider: typing.Optional[TracerProvider] = None,
):
    """Integrate with DB API library.
        https://www.python.org/dev/peps/pep-0249/

        Args:
            tracer: The :class:`opentelemetry.trace.Tracer` to use.
            connect_module: Module name where connect method is available.
            connect_method_name: The connect method name.
            database_component: Database driver name or database name "JDBI",
                "jdbc", "odbc", "postgreSQL".
            database_type: The Database type. For any SQL database, "sql".
            connection_attributes: Attribute names for database, port, host and
                user in Connection object.
    """

    # pylint: disable=unused-argument
    def wrap_connect_(
        wrapped: typing.Callable[..., typing.Any],
        instance: typing.Any,
        args: typing.Tuple[typing.Any, typing.Any],
        kwargs: typing.Dict[typing.Any, typing.Any],
    ):
        db_integration = DatabaseApiIntegration(
            name,
            database_component,
            database_type=database_type,
            connection_attributes=connection_attributes,
            version=version,
            tracer_provider=tracer_provider,
        )
        return db_integration.wrapped_connection(wrapped, args, kwargs)

    try:
        wrapt.wrap_function_wrapper(
            connect_module, connect_method_name, wrap_connect_
        )
    except Exception as ex:  # pylint: disable=broad-except
        logger.warning("Failed to integrate with DB API. %s", str(ex))


def unwrap_connect(
    connect_module: typing.Callable[..., typing.Any], connect_method_name: str,
):
    """Disable integration with DB API library.
        https://www.python.org/dev/peps/pep-0249/

        Args:
            connect_module: Module name where the connect method is available.
            connect_method_name: The connect method name.
    """
    unwrap(connect_module, connect_method_name)


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
        tracer: The :class:`opentelemetry.trace.Tracer` to use.
        connection: The connection to instrument.
        database_component: Database driver name or database name "JDBI",
            "jdbc", "odbc", "postgreSQL".
        database_type: The Database type. For any SQL database, "sql".
        connection_attributes: Attribute names for database, port, host and
            user in a connection object.

    Returns:
        An instrumented connection.
    """
    db_integration = DatabaseApiIntegration(
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


class DatabaseApiIntegration:
    def __init__(
        self,
        name: str,
        database_component: str,
        database_type: str = "sql",
        connection_attributes=None,
        version: str = "",
        tracer_provider: typing.Optional[TracerProvider] = None,
    ):
        self.connection_attributes = connection_attributes
        if self.connection_attributes is None:
            self.connection_attributes = {
                "database": "database",
                "port": "port",
                "host": "host",
                "user": "user",
            }
        self._name = name
        self._version = version
        self._tracer_provider = tracer_provider
        self.database_component = database_component
        self.database_type = database_type
        self.connection_props = {}
        self.span_attributes = {}
        self.name = ""
        self.database = ""

    def get_tracer(self):
        return get_tracer(
            self._name,
            instrumenting_library_version=self._version,
            tracer_provider=self._tracer_provider,
        )

    def wrapped_connection(
        self,
        connect_method: typing.Callable[..., typing.Any],
        args: typing.Tuple[typing.Any, typing.Any],
        kwargs: typing.Dict[typing.Any, typing.Any],
    ):
        """Add object proxy to connection object.
        """
        connection = connect_method(*args, **kwargs)
        self.get_connection_attributes(connection)
        return get_traced_connection_proxy(connection, self)

    def get_connection_attributes(self, connection):
        # Populate span fields using connection
        for key, value in self.connection_attributes.items():
            # Allow attributes nested in connection object
            attribute = functools.reduce(
                lambda attribute, attribute_value: getattr(
                    attribute, attribute_value, None
                ),
                value.split("."),
                connection,
            )
            if attribute:
                self.connection_props[key] = attribute
        self.name = self.database_component
        self.database = self.connection_props.get("database", "")
        if self.database:
            # PyMySQL encodes names with utf-8
            if hasattr(self.database, "decode"):
                self.database = self.database.decode(errors="ignore")
            self.name += "." + self.database
        user = self.connection_props.get("user")
        if user is not None:
            self.span_attributes["db.user"] = str(user)
        host = self.connection_props.get("host")
        if host is not None:
            self.span_attributes["net.peer.name"] = host
        port = self.connection_props.get("port")
        if port is not None:
            self.span_attributes["net.peer.port"] = port


def get_traced_connection_proxy(
    connection, db_api_integration, *args, **kwargs
):
    # pylint: disable=abstract-method
    class TracedConnectionProxy(wrapt.ObjectProxy):
        # pylint: disable=unused-argument
        def __init__(self, connection, *args, **kwargs):
            wrapt.ObjectProxy.__init__(self, connection)

        def cursor(self, *args, **kwargs):
            return get_traced_cursor_proxy(
                self.__wrapped__.cursor(*args, **kwargs), db_api_integration
            )

        def __enter__(self):
            self.__wrapped__.__enter__()
            return self

        def __exit__(self, *args, **kwargs):
            self.__wrapped__.__exit__(*args, **kwargs)

    return TracedConnectionProxy(connection, *args, **kwargs)


class TracedCursor:
    def __init__(self, db_api_integration: DatabaseApiIntegration):
        self._db_api_integration = db_api_integration

    def _populate_span(
        self, span: trace_api.Span, *args: typing.Tuple[typing.Any, typing.Any]
    ):
        if not span.is_recording():
            return
        statement = args[0] if args else ""
        span.set_attribute(
            "component", self._db_api_integration.database_component
        )
        span.set_attribute("db.type", self._db_api_integration.database_type)
        span.set_attribute("db.instance", self._db_api_integration.database)
        span.set_attribute("db.statement", statement)

        for (
            attribute_key,
            attribute_value,
        ) in self._db_api_integration.span_attributes.items():
            span.set_attribute(attribute_key, attribute_value)

        if len(args) > 1:
            span.set_attribute("db.statement.parameters", str(args[1]))

    def traced_execution(
        self,
        query_method: typing.Callable[..., typing.Any],
        *args: typing.Tuple[typing.Any, typing.Any],
        **kwargs: typing.Dict[typing.Any, typing.Any]
    ):

        with self._db_api_integration.get_tracer().start_as_current_span(
            self._db_api_integration.name, kind=SpanKind.CLIENT
        ) as span:
            self._populate_span(span, *args)
            try:
                result = query_method(*args, **kwargs)
                return result
            except Exception as ex:  # pylint: disable=broad-except
                if span.is_recording():
                    span.set_status(Status(StatusCode.ERROR, str(ex)))
                raise ex


def get_traced_cursor_proxy(cursor, db_api_integration, *args, **kwargs):
    _traced_cursor = TracedCursor(db_api_integration)

    # pylint: disable=abstract-method
    class TracedCursorProxy(wrapt.ObjectProxy):

        # pylint: disable=unused-argument
        def __init__(self, cursor, *args, **kwargs):
            wrapt.ObjectProxy.__init__(self, cursor)

        def execute(self, *args, **kwargs):
            return _traced_cursor.traced_execution(
                self.__wrapped__.execute, *args, **kwargs
            )

        def executemany(self, *args, **kwargs):
            return _traced_cursor.traced_execution(
                self.__wrapped__.executemany, *args, **kwargs
            )

        def callproc(self, *args, **kwargs):
            return _traced_cursor.traced_execution(
                self.__wrapped__.callproc, *args, **kwargs
            )

        def __enter__(self):
            self.__wrapped__.__enter__()
            return self

        def __exit__(self, *args, **kwargs):
            self.__wrapped__.__exit__(*args, **kwargs)

    return TracedCursorProxy(cursor, *args, **kwargs)
