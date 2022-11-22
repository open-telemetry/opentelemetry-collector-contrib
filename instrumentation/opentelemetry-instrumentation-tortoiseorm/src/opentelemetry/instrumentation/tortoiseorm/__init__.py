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
Instrument `tortoise-orm`_ to report SQL queries.

Usage
-----

.. code:: python

    from fastapi import FastAPI
    from tortoise.contrib.fastapi import register_tortoise
    from opentelemetry.sdk.resources import SERVICE_NAME, Resource
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.instrumentation.tortoiseorm import TortoiseORMInstrumentor

    app = FastAPI()
    tracer = TracerProvider(resource=Resource({SERVICE_NAME: "FastAPI"}))
    TortoiseORMInstrumentor().instrument(tracer_provider=tracer)

    register_tortoise(
        app,
        db_url="sqlite://sample.db",
        modules={"models": ["example_app.db_models"]}
    )

API
---
"""
from typing import Collection

import wrapt

from opentelemetry import trace
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.tortoiseorm.package import _instruments
from opentelemetry.instrumentation.tortoiseorm.version import __version__
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.semconv.trace import DbSystemValues, SpanAttributes
from opentelemetry.trace import SpanKind
from opentelemetry.trace.status import Status, StatusCode

try:
    import tortoise.backends.asyncpg.client

    TORTOISE_POSTGRES_SUPPORT = True
except ModuleNotFoundError:
    TORTOISE_POSTGRES_SUPPORT = False

try:
    import tortoise.backends.mysql.client

    TORTOISE_MYSQL_SUPPORT = True
except ModuleNotFoundError:
    TORTOISE_MYSQL_SUPPORT = False

try:
    import tortoise.backends.sqlite.client

    TORTOISE_SQLITE_SUPPORT = True
except ModuleNotFoundError:
    TORTOISE_SQLITE_SUPPORT = False

import tortoise.contrib.pydantic.base


class TortoiseORMInstrumentor(BaseInstrumentor):
    """An instrumentor for Tortoise-ORM
    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Instruments Tortoise ORM backend methods.
        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global
                ``capture_parameters``: set to True to capture SQL query parameters
        Returns:
            None
        """
        tracer_provider = kwargs.get("tracer_provider")
        # pylint: disable=attribute-defined-outside-init
        self._tracer = trace.get_tracer(__name__, __version__, tracer_provider)
        self.capture_parameters = kwargs.get("capture_parameters", False)
        if TORTOISE_SQLITE_SUPPORT:
            funcs = [
                "SqliteClient.execute_many",
                "SqliteClient.execute_query",
                "SqliteClient.execute_insert",
                "SqliteClient.execute_query_dict",
                "SqliteClient.execute_script",
            ]
            for func in funcs:
                wrapt.wrap_function_wrapper(
                    "tortoise.backends.sqlite.client",
                    func,
                    self._do_execute,
                )

        if TORTOISE_POSTGRES_SUPPORT:
            funcs = [
                "AsyncpgDBClient.execute_many",
                "AsyncpgDBClient.execute_query",
                "AsyncpgDBClient.execute_insert",
                "AsyncpgDBClient.execute_query_dict",
                "AsyncpgDBClient.execute_script",
            ]
            for func in funcs:
                wrapt.wrap_function_wrapper(
                    "tortoise.backends.asyncpg.client",
                    func,
                    self._do_execute,
                )

        if TORTOISE_MYSQL_SUPPORT:
            funcs = [
                "MySQLClient.execute_many",
                "MySQLClient.execute_query",
                "MySQLClient.execute_insert",
                "MySQLClient.execute_query_dict",
                "MySQLClient.execute_script",
            ]
            for func in funcs:
                wrapt.wrap_function_wrapper(
                    "tortoise.backends.mysql.client",
                    func,
                    self._do_execute,
                )
        wrapt.wrap_function_wrapper(
            "tortoise.contrib.pydantic.base",
            "PydanticModel.from_queryset",
            self._from_queryset,
        )
        wrapt.wrap_function_wrapper(
            "tortoise.contrib.pydantic.base",
            "PydanticModel.from_queryset_single",
            self._from_queryset,
        )
        wrapt.wrap_function_wrapper(
            "tortoise.contrib.pydantic.base",
            "PydanticListModel.from_queryset",
            self._from_queryset,
        )

    def _uninstrument(self, **kwargs):
        if TORTOISE_SQLITE_SUPPORT:
            unwrap(
                tortoise.backends.sqlite.client.SqliteClient, "execute_query"
            )
            unwrap(
                tortoise.backends.sqlite.client.SqliteClient, "execute_many"
            )
            unwrap(
                tortoise.backends.sqlite.client.SqliteClient, "execute_insert"
            )
            unwrap(
                tortoise.backends.sqlite.client.SqliteClient,
                "execute_query_dict",
            )
            unwrap(
                tortoise.backends.sqlite.client.SqliteClient, "execute_script"
            )
        if TORTOISE_MYSQL_SUPPORT:
            unwrap(tortoise.backends.mysql.client.MySQLClient, "execute_query")
            unwrap(tortoise.backends.mysql.client.MySQLClient, "execute_many")
            unwrap(
                tortoise.backends.mysql.client.MySQLClient, "execute_insert"
            )
            unwrap(
                tortoise.backends.mysql.client.MySQLClient,
                "execute_query_dict",
            )
            unwrap(
                tortoise.backends.mysql.client.MySQLClient, "execute_script"
            )
        if TORTOISE_POSTGRES_SUPPORT:
            unwrap(
                tortoise.backends.asyncpg.client.AsyncpgDBClient,
                "execute_query",
            )
            unwrap(
                tortoise.backends.asyncpg.client.AsyncpgDBClient,
                "execute_many",
            )
            unwrap(
                tortoise.backends.asyncpg.client.AsyncpgDBClient,
                "execute_insert",
            )
            unwrap(
                tortoise.backends.asyncpg.client.AsyncpgDBClient,
                "execute_query_dict",
            )
            unwrap(
                tortoise.backends.asyncpg.client.AsyncpgDBClient,
                "execute_script",
            )
        unwrap(tortoise.contrib.pydantic.base.PydanticModel, "from_queryset")
        unwrap(
            tortoise.contrib.pydantic.base.PydanticModel,
            "from_queryset_single",
        )
        unwrap(
            tortoise.contrib.pydantic.base.PydanticListModel, "from_queryset"
        )

    def _hydrate_span_from_args(self, connection, query, parameters) -> dict:
        """Get network and database attributes from connection."""
        span_attributes = {}
        capabilities = getattr(connection, "capabilities", None)
        if capabilities is not None:
            if capabilities.dialect == "sqlite":
                span_attributes[
                    SpanAttributes.DB_SYSTEM
                ] = DbSystemValues.SQLITE.value
            elif capabilities.dialect == "postgres":
                span_attributes[
                    SpanAttributes.DB_SYSTEM
                ] = DbSystemValues.POSTGRESQL.value
            elif capabilities.dialect == "mysql":
                span_attributes[
                    SpanAttributes.DB_SYSTEM
                ] = DbSystemValues.MYSQL.value
        dbname = getattr(connection, "filename", None)
        if dbname:
            span_attributes[SpanAttributes.DB_NAME] = dbname
        dbname = getattr(connection, "database", None)
        if dbname:
            span_attributes[SpanAttributes.DB_NAME] = dbname
        if query is not None:
            span_attributes[SpanAttributes.DB_STATEMENT] = query
        user = getattr(connection, "user", None)
        if user:
            span_attributes[SpanAttributes.DB_USER] = user
        host = getattr(connection, "host", None)
        if host:
            span_attributes[SpanAttributes.NET_PEER_NAME] = host
        port = getattr(connection, "port", None)
        if port:
            span_attributes[SpanAttributes.NET_PEER_PORT] = port

        if self.capture_parameters:
            if parameters is not None and len(parameters) > 0:
                span_attributes["db.statement.parameters"] = str(parameters)

        return span_attributes

    async def _do_execute(self, func, instance, args, kwargs):

        exception = None
        name = args[0].split()[0]

        with self._tracer.start_as_current_span(
            name, kind=SpanKind.CLIENT
        ) as span:
            if span.is_recording():
                span_attributes = self._hydrate_span_from_args(
                    instance,
                    args[0],
                    args[1:],
                )
                for attribute, value in span_attributes.items():
                    span.set_attribute(attribute, value)

            try:
                result = await func(*args, **kwargs)
            except Exception as exc:  # pylint: disable=W0703
                exception = exc
                raise
            finally:
                if span.is_recording() and exception is not None:
                    span.set_status(Status(StatusCode.ERROR))

        return result

    async def _from_queryset(self, func, modelcls, args, kwargs):

        exception = None
        name = f"pydantic.{func.__name__}"

        with self._tracer.start_as_current_span(
            name, kind=SpanKind.INTERNAL
        ) as span:
            if span.is_recording():
                span_attributes = {}

                model_config = getattr(modelcls, "Config", None)
                if model_config:
                    model_title = getattr(modelcls.Config, "title")
                    if model_title:
                        span_attributes["pydantic.model"] = model_title

                for attribute, value in span_attributes.items():
                    span.set_attribute(attribute, value)

            try:
                result = await func(*args, **kwargs)
            except Exception as exc:  # pylint: disable=W0703
                exception = exc
                raise
            finally:
                if span.is_recording() and exception is not None:
                    span.set_status(Status(StatusCode.ERROR))

        return result
