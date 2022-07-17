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

import pytest
from sqlalchemy.exc import ProgrammingError

from opentelemetry import trace
from opentelemetry.semconv.trace import SpanAttributes

from .mixins import Player, SQLAlchemyTestMixin

MSSQL_CONFIG = {
    "host": "127.0.0.1",
    "port": int(os.getenv("TEST_MSSQL_PORT", "1433")),
    "user": os.getenv("TEST_MSSQL_USER", "sa"),
    "password": os.getenv("TEST_MSSQL_PASSWORD", "yourStrong(!)Password"),
    "database": os.getenv("TEST_MSSQL_DATABASE", "opentelemetry-tests"),
    "driver": os.getenv("TEST_MSSQL_DRIVER", "ODBC+Driver+17+for+SQL+Server"),
}


class MssqlConnectorTestCase(SQLAlchemyTestMixin):
    """TestCase for pyodbc engine"""

    __test__ = True

    VENDOR = "mssql"
    SQL_DB = "opentelemetry-tests"
    ENGINE_ARGS = {
        "url": "mssql+pyodbc://%(user)s:%(password)s@%(host)s:%(port)s/%(database)s?driver=%(driver)s"
        % MSSQL_CONFIG
    }

    def check_meta(self, span):
        # check database connection tags
        self.assertEqual(
            span.attributes.get(SpanAttributes.NET_PEER_NAME),
            MSSQL_CONFIG["host"],
        )
        self.assertEqual(
            span.attributes.get(SpanAttributes.NET_PEER_PORT),
            MSSQL_CONFIG["port"],
        )
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_NAME),
            MSSQL_CONFIG["database"],
        )
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_USER), MSSQL_CONFIG["user"]
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

    def test_orm_insert(self):
        # ensures that the ORM session is traced
        wayne = Player(id=1, name="wayne")
        self.session.add(wayne)
        self.session.commit()

        spans = self.memory_exporter.get_finished_spans()
        # connect, identity insert on before the insert, insert, and identity insert off after the insert
        self.assertEqual(len(spans), 4)
        span = spans[2]
        self._check_span(span, "INSERT")
        self.assertIn(
            "INSERT INTO players",
            span.attributes.get(SpanAttributes.DB_STATEMENT),
        )
        self.check_meta(span)
