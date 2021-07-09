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
import asyncio
import logging
from unittest import mock
from unittest.mock import MagicMock

import aiopg
from aiopg.utils import (  # pylint: disable=no-name-in-module
    _ContextManager,
    _PoolAcquireContextManager,
)

import opentelemetry.instrumentation.aiopg
from opentelemetry import trace as trace_api
from opentelemetry.instrumentation.aiopg import AiopgInstrumentor, wrappers
from opentelemetry.instrumentation.aiopg.aiopg_integration import (
    AiopgIntegration,
)
from opentelemetry.sdk import resources
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase


def async_call(coro):
    loop = asyncio.get_event_loop()
    return loop.run_until_complete(coro)


class TestAiopgInstrumentor(TestBase):
    def setUp(self):
        super().setUp()
        self.origin_aiopg_connect = aiopg.connect
        self.origin_aiopg_create_pool = aiopg.create_pool
        aiopg.connect = mock_connect
        aiopg.create_pool = mock_create_pool

    def tearDown(self):
        super().tearDown()
        aiopg.connect = self.origin_aiopg_connect
        aiopg.create_pool = self.origin_aiopg_create_pool
        with self.disable_logging():
            AiopgInstrumentor().uninstrument()

    def test_instrumentor_connect(self):
        AiopgInstrumentor().instrument()

        cnx = async_call(aiopg.connect(database="test"))

        cursor = async_call(cnx.cursor())

        query = "SELECT * FROM test"
        async_call(cursor.execute(query))

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        # Check version and name in span's instrumentation info
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.aiopg
        )

        # check that no spans are generated after uninstrument
        AiopgInstrumentor().uninstrument()

        cnx = async_call(aiopg.connect(database="test"))
        cursor = async_call(cnx.cursor())
        query = "SELECT * FROM test"
        cursor.execute(query)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)

    def test_instrumentor_connect_ctx_manager(self):
        async def _ctx_manager_connect():
            AiopgInstrumentor().instrument()

            async with aiopg.connect(database="test") as cnx:
                async with cnx.cursor() as cursor:
                    query = "SELECT * FROM test"
                    await cursor.execute(query)

                    spans_list = self.memory_exporter.get_finished_spans()
                    self.assertEqual(len(spans_list), 1)
                    span = spans_list[0]

                    # Check version and name in span's instrumentation info
                    self.check_span_instrumentation_info(
                        span, opentelemetry.instrumentation.aiopg
                    )

        async_call(_ctx_manager_connect())

    def test_instrumentor_create_pool(self):
        AiopgInstrumentor().instrument()

        pool = async_call(aiopg.create_pool(database="test"))
        cnx = async_call(pool.acquire())
        cursor = async_call(cnx.cursor())

        query = "SELECT * FROM test"
        async_call(cursor.execute(query))

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        # Check version and name in span's instrumentation info
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.aiopg
        )

        # check that no spans are generated after uninstrument
        AiopgInstrumentor().uninstrument()

        pool = async_call(aiopg.create_pool(database="test"))
        cnx = async_call(pool.acquire())
        cursor = async_call(cnx.cursor())
        query = "SELECT * FROM test"
        cursor.execute(query)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)

    def test_instrumentor_create_pool_ctx_manager(self):
        async def _ctx_manager_pool():
            AiopgInstrumentor().instrument()

            async with aiopg.create_pool(database="test") as pool:
                async with pool.acquire() as cnx:
                    async with cnx.cursor() as cursor:
                        query = "SELECT * FROM test"
                        await cursor.execute(query)

                        spans_list = self.memory_exporter.get_finished_spans()
                        self.assertEqual(len(spans_list), 1)
                        span = spans_list[0]

                        # Check version and name in span's instrumentation info
                        self.check_span_instrumentation_info(
                            span, opentelemetry.instrumentation.aiopg
                        )

        async_call(_ctx_manager_pool())

    def test_custom_tracer_provider_connect(self):
        resource = resources.Resource.create({})
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result

        AiopgInstrumentor().instrument(tracer_provider=tracer_provider)

        cnx = async_call(aiopg.connect(database="test"))
        cursor = async_call(cnx.cursor())
        query = "SELECT * FROM test"
        async_call(cursor.execute(query))

        spans_list = exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]
        self.assertIs(span.resource, resource)

    def test_custom_tracer_provider_create_pool(self):
        resource = resources.Resource.create({})
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result

        AiopgInstrumentor().instrument(tracer_provider=tracer_provider)

        pool = async_call(aiopg.create_pool(database="test"))
        cnx = async_call(pool.acquire())
        cursor = async_call(cnx.cursor())
        query = "SELECT * FROM test"
        async_call(cursor.execute(query))

        spans_list = exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertIs(span.resource, resource)

    def test_instrument_connection(self):
        cnx = async_call(aiopg.connect(database="test"))
        query = "SELECT * FROM test"
        cursor = async_call(cnx.cursor())
        async_call(cursor.execute(query))

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 0)

        cnx = AiopgInstrumentor().instrument_connection(cnx)
        cursor = async_call(cnx.cursor())
        async_call(cursor.execute(query))

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)

    def test_instrument_connection_after_instrument(self):
        cnx = async_call(aiopg.connect(database="test"))
        query = "SELECT * FROM test"
        cursor = async_call(cnx.cursor())
        async_call(cursor.execute(query))

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 0)

        AiopgInstrumentor().instrument()
        cnx = AiopgInstrumentor().instrument_connection(cnx)
        cursor = async_call(cnx.cursor())
        async_call(cursor.execute(query))

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)

    def test_custom_tracer_provider_instrument_connection(self):
        resource = resources.Resource.create(
            {"service.name": "db-test-service"}
        )
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result

        cnx = async_call(aiopg.connect(database="test"))

        cnx = AiopgInstrumentor().instrument_connection(
            cnx, tracer_provider=tracer_provider
        )

        cursor = async_call(cnx.cursor())
        query = "SELECT * FROM test"
        async_call(cursor.execute(query))

        spans_list = exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertEqual(
            span.resource.attributes["service.name"], "db-test-service"
        )
        self.assertIs(span.resource, resource)

    def test_uninstrument_connection(self):
        AiopgInstrumentor().instrument()
        cnx = async_call(aiopg.connect(database="test"))
        query = "SELECT * FROM test"
        cursor = async_call(cnx.cursor())
        async_call(cursor.execute(query))

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)

        cnx = AiopgInstrumentor().uninstrument_connection(cnx)
        cursor = async_call(cnx.cursor())
        async_call(cursor.execute(query))

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)


class TestAiopgIntegration(TestBase):
    def setUp(self):
        super().setUp()
        self.tracer = self.tracer_provider.get_tracer(__name__)

    def test_span_succeeded(self):
        connection_props = {
            "database": "testdatabase",
            "server_host": "testhost",
            "server_port": 123,
            "user": "testuser",
        }
        connection_attributes = {
            "database": "database",
            "port": "server_port",
            "host": "server_host",
            "user": "user",
        }
        db_integration = AiopgIntegration(
            self.tracer,
            "testcomponent",
            connection_attributes,
            capture_parameters=True,
        )
        mock_connection = async_call(
            db_integration.wrapped_connection(
                mock_connect, {}, connection_props
            )
        )
        cursor = async_call(mock_connection.cursor())
        async_call(cursor.execute("Test query", ("param1Value", False)))
        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]
        self.assertEqual(span.name, "Test")
        self.assertIs(span.kind, trace_api.SpanKind.CLIENT)

        self.assertEqual(
            span.attributes[SpanAttributes.DB_SYSTEM], "testcomponent"
        )
        self.assertEqual(
            span.attributes[SpanAttributes.DB_NAME], "testdatabase"
        )
        self.assertEqual(
            span.attributes[SpanAttributes.DB_STATEMENT], "Test query"
        )
        self.assertEqual(
            span.attributes["db.statement.parameters"],
            "('param1Value', False)",
        )
        self.assertEqual(span.attributes[SpanAttributes.DB_USER], "testuser")
        self.assertEqual(
            span.attributes[SpanAttributes.NET_PEER_NAME], "testhost"
        )
        self.assertEqual(span.attributes[SpanAttributes.NET_PEER_PORT], 123)
        self.assertIs(span.status.status_code, trace_api.StatusCode.UNSET)

    def test_span_not_recording(self):
        connection_props = {
            "database": "testdatabase",
            "server_host": "testhost",
            "server_port": 123,
            "user": "testuser",
        }
        connection_attributes = {
            "database": "database",
            "port": "server_port",
            "host": "server_host",
            "user": "user",
        }
        mock_tracer = mock.Mock()
        mock_span = mock.Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        db_integration = AiopgIntegration(
            mock_tracer, "testcomponent", connection_attributes
        )
        mock_connection = async_call(
            db_integration.wrapped_connection(
                mock_connect, {}, connection_props
            )
        )
        cursor = async_call(mock_connection.cursor())
        async_call(cursor.execute("Test query", ("param1Value", False)))
        self.assertFalse(mock_span.is_recording())
        self.assertTrue(mock_span.is_recording.called)
        self.assertFalse(mock_span.set_attribute.called)
        self.assertFalse(mock_span.set_status.called)

    def test_span_failed(self):
        db_integration = AiopgIntegration(self.tracer, "testcomponent")
        mock_connection = async_call(
            db_integration.wrapped_connection(mock_connect, {}, {})
        )
        cursor = async_call(mock_connection.cursor())
        with self.assertRaises(Exception):
            async_call(cursor.execute("Test query", throw_exception=True))

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]
        self.assertEqual(
            span.attributes[SpanAttributes.DB_STATEMENT], "Test query"
        )
        self.assertIs(span.status.status_code, trace_api.StatusCode.ERROR)
        self.assertEqual(span.status.description, "Exception: Test Exception")

    def test_executemany(self):
        db_integration = AiopgIntegration(self.tracer, "testcomponent")
        mock_connection = async_call(
            db_integration.wrapped_connection(mock_connect, {}, {})
        )
        cursor = async_call(mock_connection.cursor())
        async_call(cursor.executemany("Test query"))
        spans_list = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]
        self.assertEqual(
            span.attributes[SpanAttributes.DB_STATEMENT], "Test query"
        )

    def test_callproc(self):
        db_integration = AiopgIntegration(self.tracer, "testcomponent")
        mock_connection = async_call(
            db_integration.wrapped_connection(mock_connect, {}, {})
        )
        cursor = async_call(mock_connection.cursor())
        async_call(cursor.callproc("Test stored procedure"))
        spans_list = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]
        self.assertEqual(
            span.attributes[SpanAttributes.DB_STATEMENT],
            "Test stored procedure",
        )

    def test_wrap_connect(self):
        aiopg_mock = AiopgMock()
        with mock.patch("aiopg.connect", aiopg_mock.connect):
            wrappers.wrap_connect(self.tracer, "-")
            connection = async_call(aiopg.connect())
            self.assertEqual(aiopg_mock.connect_call_count, 1)
            self.assertIsInstance(connection.__wrapped__, mock.Mock)

    def test_unwrap_connect(self):
        wrappers.wrap_connect(self.tracer, "-")
        aiopg_mock = AiopgMock()
        with mock.patch("aiopg.connect", aiopg_mock.connect):
            connection = async_call(aiopg.connect())
            self.assertEqual(aiopg_mock.connect_call_count, 1)
            wrappers.unwrap_connect()
            connection = async_call(aiopg.connect())
            self.assertEqual(aiopg_mock.connect_call_count, 2)
            self.assertIsInstance(connection, mock.Mock)

    def test_wrap_create_pool(self):
        async def check_connection(pool):
            async with pool.acquire() as connection:
                self.assertEqual(aiopg_mock.create_pool_call_count, 1)
                self.assertIsInstance(
                    connection.__wrapped__, AiopgConnectionMock
                )

        aiopg_mock = AiopgMock()
        with mock.patch("aiopg.create_pool", aiopg_mock.create_pool):
            wrappers.wrap_create_pool(self.tracer, "-")
            pool = async_call(aiopg.create_pool())
            async_call(check_connection(pool))

    def test_unwrap_create_pool(self):
        async def check_connection(pool):
            async with pool.acquire() as connection:
                self.assertEqual(aiopg_mock.create_pool_call_count, 2)
                self.assertIsInstance(connection, AiopgConnectionMock)

        aiopg_mock = AiopgMock()
        with mock.patch("aiopg.create_pool", aiopg_mock.create_pool):
            wrappers.wrap_create_pool(self.tracer, "-")
            pool = async_call(aiopg.create_pool())
            self.assertEqual(aiopg_mock.create_pool_call_count, 1)

            wrappers.unwrap_create_pool()
            pool = async_call(aiopg.create_pool())
            async_call(check_connection(pool))

    def test_instrument_connection(self):
        connection = mock.Mock()
        # Avoid get_attributes failing because can't concatenate mock
        connection.database = "-"
        connection2 = wrappers.instrument_connection(
            self.tracer, connection, "-"
        )
        self.assertIs(connection2.__wrapped__, connection)

    def test_uninstrument_connection(self):
        connection = mock.Mock()
        # Set connection.database to avoid a failure because mock can't
        # be concatenated
        connection.database = "-"
        connection2 = wrappers.instrument_connection(
            self.tracer, connection, "-"
        )
        self.assertIs(connection2.__wrapped__, connection)

        connection3 = wrappers.uninstrument_connection(connection2)
        self.assertIs(connection3, connection)

        with self.assertLogs(level=logging.WARNING):
            connection4 = wrappers.uninstrument_connection(connection)
        self.assertIs(connection4, connection)


# pylint: disable=unused-argument
async def mock_connect(*args, **kwargs):
    database = kwargs.get("database")
    server_host = kwargs.get("server_host")
    server_port = kwargs.get("server_port")
    user = kwargs.get("user")
    return MockConnection(database, server_port, server_host, user)


# pylint: disable=unused-argument
async def mock_create_pool(*args, **kwargs):
    database = kwargs.get("database")
    server_host = kwargs.get("server_host")
    server_port = kwargs.get("server_port")
    user = kwargs.get("user")
    return MockPool(database, server_port, server_host, user)


class MockPool:
    def __init__(self, database, server_port, server_host, user):
        self.database = database
        self.server_port = server_port
        self.server_host = server_host
        self.user = user

    async def release(self, conn):
        return conn

    def acquire(self):
        """Acquire free connection from the pool."""
        coro = self._acquire()
        return _PoolAcquireContextManager(coro, self)

    async def _acquire(self):
        connect = await mock_connect(
            self.database, self.server_port, self.server_host, self.user
        )
        return connect

    def close(self):
        pass

    async def wait_closed(self):
        pass


class MockPsycopg2Connection:
    def __init__(self, database, server_port, server_host, user):
        self.database = database
        self.server_port = server_port
        self.server_host = server_host
        self.user = user


class MockConnection:
    def __init__(self, database, server_port, server_host, user):
        self._conn = MockPsycopg2Connection(
            database, server_port, server_host, user
        )

    # pylint: disable=no-self-use
    def cursor(self):
        coro = self._cursor()
        return _ContextManager(coro)  # pylint: disable=no-value-for-parameter

    async def _cursor(self):
        return MockCursor()

    def close(self):
        pass


class MockCursor:
    # pylint: disable=unused-argument, no-self-use
    async def execute(self, query, params=None, throw_exception=False):
        if throw_exception:
            raise Exception("Test Exception")

    # pylint: disable=unused-argument, no-self-use
    async def executemany(self, query, params=None, throw_exception=False):
        if throw_exception:
            raise Exception("Test Exception")

    # pylint: disable=unused-argument, no-self-use
    async def callproc(self, query, params=None, throw_exception=False):
        if throw_exception:
            raise Exception("Test Exception")

    def close(self):
        pass


class AiopgConnectionMock:
    _conn = MagicMock()

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        pass

    async def __aenter__(self):
        return MagicMock()


class AiopgPoolMock:
    async def release(self, conn):
        return conn

    def acquire(self):
        coro = self._acquire()
        return _PoolAcquireContextManager(coro, self)

    async def _acquire(self):
        return AiopgConnectionMock()


class AiopgMock:
    def __init__(self):
        self.connect_call_count = 0
        self.create_pool_call_count = 0

    async def connect(self, *args, **kwargs):
        self.connect_call_count += 1
        return MagicMock()

    async def create_pool(self, *args, **kwargs):
        self.create_pool_call_count += 1
        return AiopgPoolMock()
