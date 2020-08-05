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

import unittest

import pytest
from sqlalchemy.exc import OperationalError

from opentelemetry import trace
from opentelemetry.instrumentation.sqlalchemy.engine import _DB, _ROWS, _STMT

from .mixins import SQLAlchemyTestMixin


class SQLiteTestCase(SQLAlchemyTestMixin):
    """TestCase for the SQLite engine"""

    __test__ = True

    VENDOR = "sqlite"
    SQL_DB = ":memory:"
    SERVICE = "sqlite"
    ENGINE_ARGS = {"url": "sqlite:///:memory:"}

    def test_engine_execute_errors(self):
        # ensures that SQL errors are reported
        with pytest.raises(OperationalError):
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
        self.assertTrue((span.end_time - span.start_time) > 0)
        # check the error
        self.assertIs(
            span.status.canonical_code,
            trace.status.StatusCanonicalCode.UNKNOWN,
        )
        self.assertEqual(
            span.status.description, "no such table: a_wrong_table"
        )
