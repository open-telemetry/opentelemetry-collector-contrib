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

import sqlalchemy

from opentelemetry import trace
from opentelemetry.instrumentation.sqlalchemy import SQLAlchemyInstrumentor
from opentelemetry.test.test_base import TestBase

POSTGRES_CONFIG = {
    "host": "127.0.0.1",
    "port": int(os.getenv("TEST_POSTGRES_PORT", "5432")),
    "user": os.getenv("TEST_POSTGRES_USER", "testuser"),
    "password": os.getenv("TEST_POSTGRES_PASSWORD", "testpassword"),
    "dbname": os.getenv("TEST_POSTGRES_DB", "opentelemetry-tests"),
}


class SQLAlchemyInstrumentTestCase(TestBase):
    """TestCase that checks if the engine is properly traced
    when the `instrument()` method is used.
    """

    def setUp(self):
        super().setUp()
        # create a traced engine with the given arguments
        SQLAlchemyInstrumentor().instrument()
        dsn = (
            "postgresql://%(user)s:%(password)s@%(host)s:%(port)s/%(dbname)s"
            % POSTGRES_CONFIG
        )
        self.engine = sqlalchemy.create_engine(dsn)

        # prepare a connection
        self.conn = self.engine.connect()

    def tearDown(self):
        # clear the database and dispose the engine
        self.conn.close()
        self.engine.dispose()
        SQLAlchemyInstrumentor().uninstrument()
        super().tearDown()

    def test_engine_traced(self):
        # ensures that the engine is traced
        rows = self.conn.execute("SELECT").fetchall()
        self.assertEqual(len(rows), 1)

        spans = self.memory_exporter.get_finished_spans()
        # trace composition
        self.assertEqual(len(spans), 2)
        span = spans[1]
        # check subset of span fields
        self.assertEqual(span.name, "SELECT opentelemetry-tests")
        self.assertIs(span.status.status_code, trace.StatusCode.UNSET)
        self.assertGreater((span.end_time - span.start_time), 0)
