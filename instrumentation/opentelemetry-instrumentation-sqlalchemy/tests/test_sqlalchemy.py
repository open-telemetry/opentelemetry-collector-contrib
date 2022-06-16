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

import pytest
import sqlalchemy
from sqlalchemy import create_engine

from opentelemetry import trace
from opentelemetry.instrumentation.sqlalchemy import SQLAlchemyInstrumentor
from opentelemetry.instrumentation.sqlalchemy.engine import EngineTracer
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider, export
from opentelemetry.test.test_base import TestBase


class TestSqlalchemyInstrumentation(TestBase):
    @pytest.fixture(autouse=True)
    def inject_fixtures(self, caplog):
        self.caplog = caplog  # pylint: disable=attribute-defined-outside-init

    def tearDown(self):
        super().tearDown()
        SQLAlchemyInstrumentor().uninstrument()

    def test_trace_integration(self):
        engine = create_engine("sqlite:///:memory:")
        SQLAlchemyInstrumentor().instrument(
            engine=engine,
            tracer_provider=self.tracer_provider,
        )
        cnx = engine.connect()
        cnx.execute("SELECT	1 + 1;").fetchall()
        spans = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].name, "SELECT :memory:")
        self.assertEqual(spans[0].kind, trace.SpanKind.CLIENT)

    def test_instrument_two_engines(self):
        engine_1 = create_engine("sqlite:///:memory:")
        engine_2 = create_engine("sqlite:///:memory:")

        SQLAlchemyInstrumentor().instrument(
            engines=[engine_1, engine_2],
            tracer_provider=self.tracer_provider,
        )

        cnx_1 = engine_1.connect()
        cnx_1.execute("SELECT	1 + 1;").fetchall()
        cnx_2 = engine_2.connect()
        cnx_2.execute("SELECT	1 + 1;").fetchall()

        spans = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans), 2)

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
            self.assertEqual(
                spans[0].instrumentation_scope.name,
                "opentelemetry.instrumentation.sqlalchemy",
            )

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
                engine=engine,
                tracer_provider=self.tracer_provider,
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
        self.assertEqual(
            spans[0].instrumentation_scope.name,
            "opentelemetry.instrumentation.sqlalchemy",
        )

    def test_custom_tracer_provider(self):
        provider = TracerProvider(
            resource=Resource.create(
                {
                    "service.name": "test",
                    "deployment.environment": "env",
                    "service.version": "1234",
                },
            ),
        )
        provider.add_span_processor(
            export.SimpleSpanProcessor(self.memory_exporter)
        )

        SQLAlchemyInstrumentor().instrument(tracer_provider=provider)
        from sqlalchemy import create_engine  # pylint: disable-all

        engine = create_engine("sqlite:///:memory:")
        cnx = engine.connect()
        cnx.execute("SELECT	1 + 1;").fetchall()
        spans = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].resource.attributes["service.name"], "test")
        self.assertEqual(
            spans[0].resource.attributes["deployment.environment"], "env"
        )
        self.assertEqual(
            spans[0].resource.attributes["service.version"], "1234"
        )

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
            self.assertEqual(
                spans[0].instrumentation_scope.name,
                "opentelemetry.instrumentation.sqlalchemy",
            )

        asyncio.get_event_loop().run_until_complete(run())

    def test_generate_commenter(self):
        logging.getLogger("sqlalchemy.engine").setLevel(logging.INFO)
        engine = create_engine("sqlite:///:memory:")
        SQLAlchemyInstrumentor().instrument(
            engine=engine,
            tracer_provider=self.tracer_provider,
            enable_commenter=True,
        )

        cnx = engine.connect()
        cnx.execute("SELECT	1 + 1;").fetchall()
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIn(
            EngineTracer._generate_comment(span),
            self.caplog.records[-2].getMessage(),
        )
