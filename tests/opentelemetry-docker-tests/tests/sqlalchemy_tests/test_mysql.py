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

MYSQL_CONFIG = {
    "host": "127.0.0.1",
    "port": int(os.getenv("TEST_MYSQL_PORT", "3306")),
    "user": os.getenv("TEST_MYSQL_USER", "testuser"),
    "password": os.getenv("TEST_MYSQL_PASSWORD", "testpassword"),
    "database": os.getenv("TEST_MYSQL_DATABASE", "opentelemetry-tests"),
}


class MysqlConnectorTestCase(SQLAlchemyTestMixin):
    """TestCase for mysql-connector engine"""

    __test__ = True

    VENDOR = "mysql"
    SQL_DB = "opentelemetry-tests"
    SERVICE = "mysql"
    ENGINE_ARGS = {
        "url": "mysql+mysqlconnector://%(user)s:%(password)s@%(host)s:%(port)s/%(database)s"
        % MYSQL_CONFIG
    }

    def check_meta(self, span):
        # check database connection tags
        self.assertEqual(span.attributes.get(_HOST), MYSQL_CONFIG["host"])
        self.assertEqual(span.attributes.get(_PORT), MYSQL_CONFIG["port"])

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
