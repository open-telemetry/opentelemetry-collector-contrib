# Copyright 2020, OpenTelemetry Authors
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

import sqlite3

from opentelemetry import trace as trace_api
from opentelemetry.instrumentation.sqlite3 import SQLite3Instrumentor
from opentelemetry.test.test_base import TestBase


class TestSQLite3(TestBase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls._connection = None
        cls._cursor = None
        cls._tracer = cls.tracer_provider.get_tracer(__name__)
        SQLite3Instrumentor().instrument(tracer_provider=cls.tracer_provider)
        cls._connection = sqlite3.connect(":memory:")
        cls._cursor = cls._connection.cursor()

    @classmethod
    def tearDownClass(cls):
        if cls._cursor:
            cls._cursor.close()
        if cls._connection:
            cls._connection.close()

    def validate_spans(self, span_name):
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 2)
        for span in spans:
            if span.name == "rootSpan":
                root_span = span
            else:
                child_span = span
            self.assertIsInstance(span.start_time, int)
            self.assertIsInstance(span.end_time, int)
        self.assertIsNotNone(root_span)
        self.assertIsNotNone(child_span)
        self.assertEqual(root_span.name, "rootSpan")
        self.assertEqual(child_span.name, span_name)
        self.assertIsNotNone(child_span.parent)
        self.assertIs(child_span.parent, root_span.get_span_context())
        self.assertIs(child_span.kind, trace_api.SpanKind.CLIENT)

    def test_execute(self):
        """Should create a child span for execute method"""
        stmt = "CREATE TABLE IF NOT EXISTS test (id integer)"
        with self._tracer.start_as_current_span("rootSpan"):
            self._cursor.execute(stmt)
        self.validate_spans("CREATE")

    def test_executemany(self):
        """Should create a child span for executemany"""
        stmt = "INSERT INTO test (id) VALUES (?)"
        with self._tracer.start_as_current_span("rootSpan"):
            data = [("1",), ("2",), ("3",)]
            self._cursor.executemany(stmt, data)
        self.validate_spans("INSERT")

    def test_callproc(self):
        """Should create a child span for callproc"""
        with self._tracer.start_as_current_span("rootSpan"), self.assertRaises(
            Exception
        ):
            self._cursor.callproc("test", ())
            self.validate_spans("test")
