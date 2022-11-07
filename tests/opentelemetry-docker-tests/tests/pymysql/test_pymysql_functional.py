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

import os

import pymysql as pymy

from opentelemetry import trace as trace_api
from opentelemetry.instrumentation.pymysql import PyMySQLInstrumentor
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase

MYSQL_USER = os.getenv("MYSQL_USER", "testuser")
MYSQL_PASSWORD = os.getenv("MYSQL_PASSWORD", "testpassword")
MYSQL_HOST = os.getenv("MYSQL_HOST", "localhost")
MYSQL_PORT = int(os.getenv("MYSQL_PORT", "3306"))
MYSQL_DB_NAME = os.getenv("MYSQL_DB_NAME", "opentelemetry-tests")


class TestFunctionalPyMysql(TestBase):
    def setUp(self):
        super().setUp()
        self._tracer = self.tracer_provider.get_tracer(__name__)
        PyMySQLInstrumentor().instrument()
        self._connection = pymy.connect(
            user=MYSQL_USER,
            password=MYSQL_PASSWORD,
            host=MYSQL_HOST,
            port=MYSQL_PORT,
            database=MYSQL_DB_NAME,
        )
        self._cursor = self._connection.cursor()

    def tearDown(self):
        self._cursor.close()
        self._connection.close()
        PyMySQLInstrumentor().uninstrument()
        super().tearDown()

    def validate_spans(self, span_name):
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 2)
        for span in spans:
            if span.name == "rootSpan":
                root_span = span
            else:
                db_span = span
            self.assertIsInstance(span.start_time, int)
            self.assertIsInstance(span.end_time, int)
        self.assertIsNotNone(root_span)
        self.assertIsNotNone(db_span)
        self.assertEqual(root_span.name, "rootSpan")
        self.assertEqual(db_span.name, span_name)
        self.assertIsNotNone(db_span.parent)
        self.assertIs(db_span.parent, root_span.get_span_context())
        self.assertIs(db_span.kind, trace_api.SpanKind.CLIENT)
        self.assertEqual(db_span.attributes[SpanAttributes.DB_SYSTEM], "mysql")
        self.assertEqual(
            db_span.attributes[SpanAttributes.DB_NAME], MYSQL_DB_NAME
        )
        self.assertEqual(
            db_span.attributes[SpanAttributes.DB_USER], MYSQL_USER
        )
        self.assertEqual(
            db_span.attributes[SpanAttributes.NET_PEER_NAME], MYSQL_HOST
        )
        self.assertEqual(
            db_span.attributes[SpanAttributes.NET_PEER_PORT], MYSQL_PORT
        )

    def test_execute(self):
        """Should create a child span for execute"""
        stmt = "CREATE TABLE IF NOT EXISTS test (id INT)"
        with self._tracer.start_as_current_span("rootSpan"):
            self._cursor.execute(stmt)
        self.validate_spans("CREATE")

    def test_execute_with_cursor_context_manager(self):
        """Should create a child span for execute with cursor context"""
        stmt = "CREATE TABLE IF NOT EXISTS test (id INT)"
        with self._tracer.start_as_current_span("rootSpan"):
            with self._connection.cursor() as cursor:
                cursor.execute(stmt)
        self.validate_spans("CREATE")

    def test_executemany(self):
        """Should create a child span for executemany"""
        stmt = "INSERT INTO test (id) VALUES (%s)"
        with self._tracer.start_as_current_span("rootSpan"):
            data = (("1",), ("2",), ("3",))
            self._cursor.executemany(stmt, data)
        self.validate_spans("INSERT")

    def test_callproc(self):
        """Should create a child span for callproc"""
        with self._tracer.start_as_current_span("rootSpan"), self.assertRaises(
            Exception
        ):
            self._cursor.callproc("test", ())
            self.validate_spans("test")

    def test_commit(self):
        stmt = "INSERT INTO test (id) VALUES (%s)"
        with self._tracer.start_as_current_span("rootSpan"):
            data = (("4",), ("5",), ("6",))
            self._cursor.executemany(stmt, data)
            self._connection.commit()
        self.validate_spans("INSERT")

    def test_rollback(self):
        stmt = "INSERT INTO test (id) VALUES (%s)"
        with self._tracer.start_as_current_span("rootSpan"):
            data = (("7",), ("8",), ("9",))
            self._cursor.executemany(stmt, data)
            self._connection.rollback()
        self.validate_spans("INSERT")
