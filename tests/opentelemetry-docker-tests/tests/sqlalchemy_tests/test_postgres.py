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

import psycopg2
import pytest
from sqlalchemy.exc import ProgrammingError

from opentelemetry import trace
from opentelemetry.semconv.trace import SpanAttributes

from .mixins import SQLAlchemyTestMixin

POSTGRES_CONFIG = {
    "host": "127.0.0.1",
    "port": int(os.getenv("TEST_POSTGRES_PORT", "5432")),
    "user": os.getenv("TEST_POSTGRES_USER", "testuser"),
    "password": os.getenv("TEST_POSTGRES_PASSWORD", "testpassword"),
    "dbname": os.getenv("TEST_POSTGRES_DB", "opentelemetry-tests"),
}


class PostgresTestCase(SQLAlchemyTestMixin):
    """TestCase for Postgres Engine"""

    __test__ = True

    VENDOR = "postgresql"
    SQL_DB = "opentelemetry-tests"
    ENGINE_ARGS = {
        "url": "postgresql://%(user)s:%(password)s@%(host)s:%(port)s/%(dbname)s"
        % POSTGRES_CONFIG
    }

    def check_meta(self, span):
        # check database connection tags
        self.assertEqual(
            span.attributes.get(SpanAttributes.NET_PEER_NAME),
            POSTGRES_CONFIG["host"],
        )
        self.assertEqual(
            span.attributes.get(SpanAttributes.NET_PEER_PORT),
            POSTGRES_CONFIG["port"],
        )

    def test_engine_execute_errors(self):
        # ensures that SQL errors are reported
        with pytest.raises(ProgrammingError):
            with self.connection() as conn:
                conn.execute("SELECT * FROM a_wrong_table").fetchall()

        spans = self.memory_exporter.get_finished_spans()
        # one span for the connection and one for the query
        self.assertEqual(len(spans), 2)
        span = spans[1]
        # span fields
        self.assertEqual(span.name, "SELECT opentelemetry-tests")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT),
            "SELECT * FROM a_wrong_table",
        )
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_NAME), self.SQL_DB
        )
        self.check_meta(span)
        self.assertTrue(span.end_time - span.start_time > 0)
        # check the error
        self.assertIs(
            span.status.status_code,
            trace.StatusCode.ERROR,
        )
        self.assertIn("a_wrong_table", span.status.description)


class PostgresCreatorTestCase(PostgresTestCase):
    """TestCase for Postgres Engine that includes the same tests set
    of `PostgresTestCase`, but it uses a specific `creator` function.
    """

    VENDOR = "postgresql"
    SQL_DB = "opentelemetry-tests"
    ENGINE_ARGS = {
        "url": "postgresql://",
        "creator": lambda: psycopg2.connect(**POSTGRES_CONFIG),
    }
