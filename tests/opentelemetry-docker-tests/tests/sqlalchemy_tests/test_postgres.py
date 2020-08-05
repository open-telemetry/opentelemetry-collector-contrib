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
import unittest

import psycopg2
import pytest
from sqlalchemy.exc import ProgrammingError

from opentelemetry import trace
from opentelemetry.instrumentation.sqlalchemy.engine import (
    _DB,
    _HOST,
    _PORT,
    _ROWS,
    _STMT,
)

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

    VENDOR = "postgres"
    SQL_DB = "opentelemetry-tests"
    SERVICE = "postgres"
    ENGINE_ARGS = {
        "url": "postgresql://%(user)s:%(password)s@%(host)s:%(port)s/%(dbname)s"
        % POSTGRES_CONFIG
    }

    def check_meta(self, span):
        # check database connection tags
        self.assertEqual(span.attributes.get(_HOST), POSTGRES_CONFIG["host"])
        self.assertEqual(span.attributes.get(_PORT), POSTGRES_CONFIG["port"])

    def test_engine_execute_errors(self):
        # ensures that SQL errors are reported
        with pytest.raises(ProgrammingError):
            with self.connection() as conn:
                conn.execute("SELECT * FROM a_wrong_table").fetchall()

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        # span fields
        self.assertEqual(span.name, "{}.query".format(self.VENDOR))
        self.assertEqual(span.attributes.get("service"), self.SERVICE)
        self.assertEqual(
            span.attributes.get(_STMT), "SELECT * FROM a_wrong_table"
        )
        self.assertEqual(span.attributes.get(_DB), self.SQL_DB)
        self.assertIsNone(span.attributes.get(_ROWS))
        self.check_meta(span)
        self.assertTrue(span.end_time - span.start_time > 0)
        # check the error
        self.assertIs(
            span.status.canonical_code,
            trace.status.StatusCanonicalCode.UNKNOWN,
        )
        self.assertIn("a_wrong_table", span.status.description)


class PostgresCreatorTestCase(PostgresTestCase):
    """TestCase for Postgres Engine that includes the same tests set
    of `PostgresTestCase`, but it uses a specific `creator` function.
    """

    VENDOR = "postgres"
    SQL_DB = "opentelemetry-tests"
    SERVICE = "postgres"
    ENGINE_ARGS = {
        "url": "postgresql://",
        "creator": lambda: psycopg2.connect(**POSTGRES_CONFIG),
    }
