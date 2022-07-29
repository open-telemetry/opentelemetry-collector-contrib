# Copyright 2020, OpenTelemetry Authors
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

import psycopg2
from test_psycopg_functional import (
    POSTGRES_DB_NAME,
    POSTGRES_HOST,
    POSTGRES_PASSWORD,
    POSTGRES_PORT,
    POSTGRES_USER,
)

from opentelemetry.instrumentation.psycopg2 import Psycopg2Instrumentor
from opentelemetry.test.test_base import TestBase


class TestFunctionalPsycopg(TestBase):
    def setUp(self):
        super().setUp()
        self._tracer = self.tracer_provider.get_tracer(__name__)
        Psycopg2Instrumentor().instrument(enable_commenter=True)
        self._connection = psycopg2.connect(
            dbname=POSTGRES_DB_NAME,
            user=POSTGRES_USER,
            password=POSTGRES_PASSWORD,
            host=POSTGRES_HOST,
            port=POSTGRES_PORT,
        )
        self._connection.set_session(autocommit=True)
        self._cursor = self._connection.cursor()

    def tearDown(self):
        self._cursor.close()
        self._connection.close()
        Psycopg2Instrumentor().uninstrument()
        super().tearDown()

    def test_commenter_enabled(self):
        self._cursor.execute("SELECT  1;")
        self.assertRegex(
            self._cursor.query.decode("ascii"),
            r"SELECT  1 /\*db_driver='psycopg2(.*)',dbapi_level='\d.\d',dbapi_threadsafety=\d,driver_paramstyle=(.*),libpq_version=\d*,traceparent='\d{1,2}-[a-zA-Z0-9_]{32}-[a-zA-Z0-9_]{16}-\d{1,2}'\*/;",
        )
