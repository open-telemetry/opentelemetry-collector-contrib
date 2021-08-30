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

from unittest import mock

import pymysql

import opentelemetry.instrumentation.pymysql
from opentelemetry.instrumentation.pymysql import PyMySQLInstrumentor
from opentelemetry.sdk import resources
from opentelemetry.test.test_base import TestBase


class TestPyMysqlIntegration(TestBase):
    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            PyMySQLInstrumentor().uninstrument()

    @mock.patch("pymysql.connect")
    # pylint: disable=unused-argument
    def test_instrumentor(self, mock_connect):
        PyMySQLInstrumentor().instrument()

        cnx = pymysql.connect(database="test")
        cursor = cnx.cursor()
        query = "SELECT * FROM test"
        cursor.execute(query)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.pymysql
        )

        # check that no spans are generated after uninstrument
        PyMySQLInstrumentor().uninstrument()

        cnx = pymysql.connect(database="test")
        cursor = cnx.cursor()
        query = "SELECT * FROM test"
        cursor.execute(query)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)

    @mock.patch("pymysql.connect")
    # pylint: disable=unused-argument
    def test_custom_tracer_provider(self, mock_connect):
        resource = resources.Resource.create({})
        result = self.create_tracer_provider(resource=resource)
        tracer_provider, exporter = result

        PyMySQLInstrumentor().instrument(tracer_provider=tracer_provider)

        cnx = pymysql.connect(database="test")
        cursor = cnx.cursor()
        query = "SELECT * FROM test"
        cursor.execute(query)

        spans_list = exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertIs(span.resource, resource)

    @mock.patch("pymysql.connect")
    # pylint: disable=unused-argument
    def test_instrument_connection(self, mock_connect):
        cnx = pymysql.connect(database="test")
        query = "SELECT * FROM test"
        cursor = cnx.cursor()
        cursor.execute(query)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 0)

        cnx = PyMySQLInstrumentor().instrument_connection(cnx)
        cursor = cnx.cursor()
        cursor.execute(query)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)

    @mock.patch("pymysql.connect")
    # pylint: disable=unused-argument
    def test_uninstrument_connection(self, mock_connect):
        PyMySQLInstrumentor().instrument()
        cnx = pymysql.connect(database="test")
        query = "SELECT * FROM test"
        cursor = cnx.cursor()
        cursor.execute(query)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)

        cnx = PyMySQLInstrumentor().uninstrument_connection(cnx)
        cursor = cnx.cursor()
        cursor.execute(query)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
