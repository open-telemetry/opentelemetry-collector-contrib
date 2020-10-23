import asyncio
import os

import asyncpg

from opentelemetry.instrumentation.asyncpg import AsyncPGInstrumentor
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace.status import StatusCanonicalCode

POSTGRES_HOST = os.getenv("POSTGRESQL_HOST ", "localhost")
POSTGRES_PORT = int(os.getenv("POSTGRESQL_PORT ", "5432"))
POSTGRES_DB_NAME = os.getenv("POSTGRESQL_DB_NAME ", "opentelemetry-tests")
POSTGRES_PASSWORD = os.getenv("POSTGRESQL_HOST ", "testpassword")
POSTGRES_USER = os.getenv("POSTGRESQL_HOST ", "testuser")


def async_call(coro):
    loop = asyncio.get_event_loop()
    return loop.run_until_complete(coro)


class TestFunctionalAsyncPG(TestBase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls._connection = None
        cls._cursor = None
        cls._tracer = cls.tracer_provider.get_tracer(__name__)
        AsyncPGInstrumentor().instrument(tracer_provider=cls.tracer_provider)
        cls._connection = async_call(
            asyncpg.connect(
                database=POSTGRES_DB_NAME,
                user=POSTGRES_USER,
                password=POSTGRES_PASSWORD,
                host=POSTGRES_HOST,
                port=POSTGRES_PORT,
            )
        )

    @classmethod
    def tearDownClass(cls):
        AsyncPGInstrumentor().uninstrument()

    def test_instrumented_execute_method_without_arguments(self, *_, **__):
        async_call(self._connection.execute("SELECT 42;"))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            StatusCanonicalCode.OK, spans[0].status.canonical_code
        )
        self.assertEqual(
            spans[0].attributes,
            {
                "db.type": "sql",
                "db.user": POSTGRES_USER,
                "db.instance": POSTGRES_DB_NAME,
                "db.statement": "SELECT 42;",
            },
        )

    def test_instrumented_fetch_method_without_arguments(self, *_, **__):
        async_call(self._connection.fetch("SELECT 42;"))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            spans[0].attributes,
            {
                "db.type": "sql",
                "db.user": POSTGRES_USER,
                "db.instance": POSTGRES_DB_NAME,
                "db.statement": "SELECT 42;",
            },
        )

    def test_instrumented_transaction_method(self, *_, **__):
        async def _transaction_execute():
            async with self._connection.transaction():
                await self._connection.execute("SELECT 42;")

        async_call(_transaction_execute())

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(3, len(spans))
        self.assertEqual(
            {
                "db.instance": POSTGRES_DB_NAME,
                "db.user": POSTGRES_USER,
                "db.type": "sql",
                "db.statement": "BEGIN;",
            },
            spans[0].attributes,
        )
        self.assertEqual(
            StatusCanonicalCode.OK, spans[0].status.canonical_code
        )
        self.assertEqual(
            {
                "db.instance": POSTGRES_DB_NAME,
                "db.user": POSTGRES_USER,
                "db.type": "sql",
                "db.statement": "SELECT 42;",
            },
            spans[1].attributes,
        )
        self.assertEqual(
            StatusCanonicalCode.OK, spans[1].status.canonical_code
        )
        self.assertEqual(
            {
                "db.instance": POSTGRES_DB_NAME,
                "db.user": POSTGRES_USER,
                "db.type": "sql",
                "db.statement": "COMMIT;",
            },
            spans[2].attributes,
        )
        self.assertEqual(
            StatusCanonicalCode.OK, spans[2].status.canonical_code
        )

    def test_instrumented_failed_transaction_method(self, *_, **__):
        async def _transaction_execute():
            async with self._connection.transaction():
                await self._connection.execute("SELECT 42::uuid;")

        with self.assertRaises(asyncpg.CannotCoerceError):
            async_call(_transaction_execute())

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(3, len(spans))
        self.assertEqual(
            {
                "db.instance": POSTGRES_DB_NAME,
                "db.user": POSTGRES_USER,
                "db.type": "sql",
                "db.statement": "BEGIN;",
            },
            spans[0].attributes,
        )
        self.assertEqual(
            StatusCanonicalCode.OK, spans[0].status.canonical_code
        )
        self.assertEqual(
            {
                "db.instance": POSTGRES_DB_NAME,
                "db.user": POSTGRES_USER,
                "db.type": "sql",
                "db.statement": "SELECT 42::uuid;",
            },
            spans[1].attributes,
        )
        self.assertEqual(
            StatusCanonicalCode.INVALID_ARGUMENT,
            spans[1].status.canonical_code,
        )
        self.assertEqual(
            {
                "db.instance": POSTGRES_DB_NAME,
                "db.user": POSTGRES_USER,
                "db.type": "sql",
                "db.statement": "ROLLBACK;",
            },
            spans[2].attributes,
        )
        self.assertEqual(
            StatusCanonicalCode.OK, spans[2].status.canonical_code
        )

    def test_instrumented_method_doesnt_capture_parameters(self, *_, **__):
        async_call(self._connection.execute("SELECT $1;", "1"))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            StatusCanonicalCode.OK, spans[0].status.canonical_code
        )
        self.assertEqual(
            spans[0].attributes,
            {
                "db.type": "sql",
                "db.user": POSTGRES_USER,
                # This shouldn't be set because we don't capture parameters by
                # default
                #
                # "db.statement.parameters": "('1',)",
                "db.instance": POSTGRES_DB_NAME,
                "db.statement": "SELECT $1;",
            },
        )


class TestFunctionalAsyncPG_CaptureParameters(TestBase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls._connection = None
        cls._cursor = None
        cls._tracer = cls.tracer_provider.get_tracer(__name__)
        AsyncPGInstrumentor(capture_parameters=True).instrument(
            tracer_provider=cls.tracer_provider
        )
        cls._connection = async_call(
            asyncpg.connect(
                database=POSTGRES_DB_NAME,
                user=POSTGRES_USER,
                password=POSTGRES_PASSWORD,
                host=POSTGRES_HOST,
                port=POSTGRES_PORT,
            )
        )

    @classmethod
    def tearDownClass(cls):
        AsyncPGInstrumentor().uninstrument()

    def test_instrumented_execute_method_with_arguments(self, *_, **__):
        async_call(self._connection.execute("SELECT $1;", "1"))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            StatusCanonicalCode.OK, spans[0].status.canonical_code
        )
        self.assertEqual(
            spans[0].attributes,
            {
                "db.type": "sql",
                "db.user": POSTGRES_USER,
                "db.statement.parameters": "('1',)",
                "db.instance": POSTGRES_DB_NAME,
                "db.statement": "SELECT $1;",
            },
        )

    def test_instrumented_fetch_method_with_arguments(self, *_, **__):
        async_call(self._connection.fetch("SELECT $1;", "1"))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            spans[0].attributes,
            {
                "db.type": "sql",
                "db.user": POSTGRES_USER,
                "db.statement.parameters": "('1',)",
                "db.instance": POSTGRES_DB_NAME,
                "db.statement": "SELECT $1;",
            },
        )

    def test_instrumented_executemany_method_with_arguments(self, *_, **__):
        async_call(self._connection.executemany("SELECT $1;", [["1"], ["2"]]))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            {
                "db.type": "sql",
                "db.statement": "SELECT $1;",
                "db.statement.parameters": "([['1'], ['2']],)",
                "db.user": POSTGRES_USER,
                "db.instance": POSTGRES_DB_NAME,
            },
            spans[0].attributes,
        )

    def test_instrumented_execute_interface_error_method(self, *_, **__):
        with self.assertRaises(asyncpg.InterfaceError):
            async_call(self._connection.execute("SELECT 42;", 1, 2, 3))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            spans[0].attributes,
            {
                "db.type": "sql",
                "db.instance": POSTGRES_DB_NAME,
                "db.user": POSTGRES_USER,
                "db.statement.parameters": "(1, 2, 3)",
                "db.statement": "SELECT 42;",
            },
        )
