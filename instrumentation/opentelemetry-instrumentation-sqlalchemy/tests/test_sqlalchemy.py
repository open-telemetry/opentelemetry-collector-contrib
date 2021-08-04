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
from unittest import mock

import pytest
import sqlalchemy
from sqlalchemy import create_engine

from opentelemetry import trace
from opentelemetry.instrumentation.sqlalchemy import SQLAlchemyInstrumentor
from opentelemetry.test.test_base import TestBase


class TestSqlalchemyInstrumentation(TestBase):
    def tearDown(self):
        super().tearDown()
        SQLAlchemyInstrumentor().uninstrument()

    def test_trace_integration(self):
        engine = create_engine("sqlite:///:memory:")
        SQLAlchemyInstrumentor().instrument(
            engine=engine, tracer_provider=self.tracer_provider,
        )
        cnx = engine.connect()
        cnx.execute("SELECT	1 + 1;").fetchall()
        spans = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].name, "SELECT :memory:")
        self.assertEqual(spans[0].kind, trace.SpanKind.CLIENT)

    @pytest.mark.skipif(
        not sqlalchemy.__version__.startswith("1.4"),
        reason="only run async tests for 1.4",
    )
    def test_async_trace_integration(self):
        async def run():
            from sqlalchemy.ext.asyncio import (  # pylint: disable-all
                create_async_engine,
            )

            engine = create_async_engine("sqlite+aiosqlite:///:memory:")
            SQLAlchemyInstrumentor().instrument(
                engine=engine.sync_engine, tracer_provider=self.tracer_provider
            )
            async with engine.connect() as cnx:
                await cnx.execute(sqlalchemy.text("SELECT	1 + 1;"))
            spans = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(spans), 1)
            self.assertEqual(spans[0].name, "SELECT :memory:")
            self.assertEqual(spans[0].kind, trace.SpanKind.CLIENT)

        asyncio.get_event_loop().run_until_complete(run())

    def test_not_recording(self):
        mock_tracer = mock.Mock()
        mock_span = mock.Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with mock.patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            engine = create_engine("sqlite:///:memory:")
            SQLAlchemyInstrumentor().instrument(
                engine=engine, tracer_provider=self.tracer_provider,
            )
            cnx = engine.connect()
            cnx.execute("SELECT	1 + 1;").fetchall()
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_create_engine_wrapper(self):
        SQLAlchemyInstrumentor().instrument()
        from sqlalchemy import create_engine  # pylint: disable-all

        engine = create_engine("sqlite:///:memory:")
        cnx = engine.connect()
        cnx.execute("SELECT	1 + 1;").fetchall()
        spans = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].name, "SELECT :memory:")
        self.assertEqual(spans[0].kind, trace.SpanKind.CLIENT)

    @pytest.mark.skipif(
        not sqlalchemy.__version__.startswith("1.4"),
        reason="only run async tests for 1.4",
    )
    def test_create_async_engine_wrapper(self):
        async def run():
            SQLAlchemyInstrumentor().instrument()
            from sqlalchemy.ext.asyncio import (  # pylint: disable-all
                create_async_engine,
            )

            engine = create_async_engine("sqlite+aiosqlite:///:memory:")
            async with engine.connect() as cnx:
                await cnx.execute(sqlalchemy.text("SELECT	1 + 1;"))
            spans = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(spans), 1)
            self.assertEqual(spans[0].name, "SELECT :memory:")
            self.assertEqual(spans[0].kind, trace.SpanKind.CLIENT)

        asyncio.get_event_loop().run_until_complete(run())
