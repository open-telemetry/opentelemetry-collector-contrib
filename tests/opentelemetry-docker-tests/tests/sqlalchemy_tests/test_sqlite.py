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

import pytest
from sqlalchemy.exc import OperationalError

from opentelemetry import trace
from opentelemetry.semconv.trace import SpanAttributes

from .mixins import SQLAlchemyTestMixin


class SQLiteTestCase(SQLAlchemyTestMixin):
    """TestCase for the SQLite engine"""

    __test__ = True

    VENDOR = "sqlite"
    SQL_DB = ":memory:"
    ENGINE_ARGS = {"url": "sqlite:///:memory:"}

    def test_engine_execute_errors(self):
        # ensures that SQL errors are reported
        stmt = "SELECT * FROM a_wrong_table"
        with pytest.raises(OperationalError):
            with self.connection() as conn:
                conn.execute(stmt).fetchall()

        spans = self.memory_exporter.get_finished_spans()
        # one span for the connection and one span for the query
        self.assertEqual(len(spans), 2)
        span = spans[1]
        # span fields
        self.assertEqual(span.name, "SELECT :memory:")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT),
            "SELECT * FROM a_wrong_table",
        )
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_NAME), self.SQL_DB
        )
        self.assertTrue((span.end_time - span.start_time) > 0)
        # check the error
        self.assertIs(
            span.status.status_code,
            trace.StatusCode.ERROR,
        )
        self.assertEqual(
            span.status.description, "no such table: a_wrong_table"
        )
