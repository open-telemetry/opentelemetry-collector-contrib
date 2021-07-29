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
from werkzeug.wrappers import BaseResponse

from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.test.test_base import TestBase
from opentelemetry.test.wsgitestutil import WsgiTestBase

# pylint: disable=import-error
from .base_test import InstrumentationTest


class TestAutomatic(InstrumentationTest, TestBase, WsgiTestBase):
    def setUp(self):
        super().setUp()

        FlaskInstrumentor().instrument()

        self.app = flask.Flask(__name__)

        self._common_initialization()

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            FlaskInstrumentor().uninstrument()

    def test_uninstrument(self):
        # pylint: disable=access-member-before-definition
        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        FlaskInstrumentor().uninstrument()
        self.app = flask.Flask(__name__)

        self.app.route("/hello/<int:helloid>")(self._hello_endpoint)
        # pylint: disable=attribute-defined-outside-init
        self.client = Client(self.app, BaseResponse)

        resp = self.client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

    def test_exluded_urls_explicit(self):
        FlaskInstrumentor().uninstrument()
        FlaskInstrumentor().instrument(excluded_urls="/hello/456")

        self.app = flask.Flask(__name__)
        self.app.route("/hello/<int:helloid>")(self._hello_endpoint)
        client = Client(self.app, BaseResponse)

        resp = client.get("/hello/123")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 123"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)

        resp = client.get("/hello/456")
        self.assertEqual(200, resp.status_code)
        self.assertEqual([b"Hello: 456"], list(resp.response))
        span_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(span_list), 1)
