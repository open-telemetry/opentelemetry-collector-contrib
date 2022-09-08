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

import flask
from werkzeug.test import Client
from werkzeug.wrappers import Response

from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.test.wsgitestutil import WsgiTestBase

# pylint: disable=import-error
from .base_test import InstrumentationTest


class TestSQLCommenter(InstrumentationTest, WsgiTestBase):
    def setUp(self):
        super().setUp()
        FlaskInstrumentor().instrument()
        self.app = flask.Flask(__name__)
        self._common_initialization()

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FlaskInstrumentor().uninstrument()

    def test_sqlcommenter_enabled_default(self):

        self.app = flask.Flask(__name__)
        self.app.route("/sqlcommenter")(self._sqlcommenter_endpoint)
        client = Client(self.app, Response)

        resp = client.get("/sqlcommenter")
        self.assertEqual(200, resp.status_code)
        self.assertRegex(
            list(resp.response)[0].strip(),
            b'{"controller":"_sqlcommenter_endpoint","framework":"flask:(.*)","route":"/sqlcommenter"}',
        )

    def test_sqlcommenter_enabled_with_configurations(self):
        FlaskInstrumentor().uninstrument()
        FlaskInstrumentor().instrument(
            enable_commenter=True, commenter_options={"route": False}
        )

        self.app = flask.Flask(__name__)
        self.app.route("/sqlcommenter")(self._sqlcommenter_endpoint)
        client = Client(self.app, Response)

        resp = client.get("/sqlcommenter")
        self.assertEqual(200, resp.status_code)
        self.assertRegex(
            list(resp.response)[0].strip(),
            b'{"controller":"_sqlcommenter_endpoint","framework":"flask:(.*)"}',
        )

    def test_sqlcommenter_disabled(self):
        FlaskInstrumentor().uninstrument()
        FlaskInstrumentor().instrument(enable_commenter=False)

        self.app = flask.Flask(__name__)
        self.app.route("/sqlcommenter")(self._sqlcommenter_endpoint)
        client = Client(self.app, Response)

        resp = client.get("/sqlcommenter")
        self.assertEqual(200, resp.status_code)
        self.assertEqual(list(resp.response)[0].strip(), b"{}")
