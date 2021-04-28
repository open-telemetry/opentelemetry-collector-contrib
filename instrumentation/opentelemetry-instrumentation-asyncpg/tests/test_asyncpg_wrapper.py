from asyncpg import Connection

from opentelemetry.instrumentation.asyncpg import AsyncPGInstrumentor
from opentelemetry.test.test_base import TestBase


class TestAsyncPGInstrumentation(TestBase):
    def test_duplicated_instrumentation(self):
        AsyncPGInstrumentor().instrument()
        AsyncPGInstrumentor().instrument()
        AsyncPGInstrumentor().instrument()
        AsyncPGInstrumentor().uninstrument()
        for method_name in ["execute", "fetch"]:
            method = getattr(Connection, method_name, None)
            self.assertFalse(
                hasattr(method, "_opentelemetry_ext_asyncpg_applied")
            )

    def test_duplicated_uninstrumentation(self):
        AsyncPGInstrumentor().instrument()
        AsyncPGInstrumentor().uninstrument()
        AsyncPGInstrumentor().uninstrument()
        AsyncPGInstrumentor().uninstrument()
        for method_name in ["execute", "fetch"]:
            method = getattr(Connection, method_name, None)
            self.assertFalse(
                hasattr(method, "_opentelemetry_ext_asyncpg_applied")
            )
