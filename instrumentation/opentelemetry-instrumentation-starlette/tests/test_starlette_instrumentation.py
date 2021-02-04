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
from unittest.mock import patch

from starlette import applications
from starlette.responses import PlainTextResponse
from starlette.routing import Route
from starlette.testclient import TestClient

import opentelemetry.instrumentation.starlette as otel_starlette
from opentelemetry.test.test_base import TestBase
from opentelemetry.util.http import get_excluded_urls


class TestStarletteManualInstrumentation(TestBase):
    def _create_app(self):
        app = self._create_starlette_app()
        self._instrumentor.instrument_app(app)
        return app

    def setUp(self):
        super().setUp()
        self.env_patch = patch.dict(
            "os.environ",
            {"OTEL_PYTHON_STARLETTE_EXCLUDED_URLS": "/exclude/123,healthzz"},
        )
        self.env_patch.start()
        self.exclude_patch = patch(
            "opentelemetry.instrumentation.starlette._excluded_urls",
            get_excluded_urls("STARLETTE"),
        )
        self.exclude_patch.start()
        self._instrumentor = otel_starlette.StarletteInstrumentor()
        self._app = self._create_app()
        self._client = TestClient(self._app)

    def tearDown(self):
        super().tearDown()
        self.env_patch.stop()
        self.exclude_patch.stop()

    def test_basic_starlette_call(self):
        self._client.get("/foobar")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        for span in spans:
            self.assertIn("/foobar", span.name)

    def test_starlette_route_attribute_added(self):
        """Ensure that starlette routes are used as the span name."""
        self._client.get("/user/123")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)
        for span in spans:
            self.assertIn("/user/{username}", span.name)
        self.assertEqual(
            spans[-1].attributes["http.route"], "/user/{username}"
        )
        # ensure that at least one attribute that is populated by
        # the asgi instrumentation is successfully feeding though.
        self.assertEqual(spans[-1].attributes["http.flavor"], "1.1")

    def test_starlette_excluded_urls(self):
        """Ensure that givem starlette routes are excluded."""
        self._client.get("/healthzz")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    @staticmethod
    def _create_starlette_app():
        def home(_):
            return PlainTextResponse("hi")

        def health(_):
            return PlainTextResponse("ok")

        app = applications.Starlette(
            routes=[
                Route("/foobar", home),
                Route("/user/{username}", home),
                Route("/healthzz", health),
            ]
        )
        return app


class TestAutoInstrumentation(TestStarletteManualInstrumentation):
    """Test the auto-instrumented variant

    Extending the manual instrumentation as most test cases apply
    to both.
    """

    def _create_app(self):
        # instrumentation is handled by the instrument call
        self._instrumentor.instrument()
        return self._create_starlette_app()

    def tearDown(self):
        self._instrumentor.uninstrument()
        super().tearDown()


class TestAutoInstrumentationLogic(unittest.TestCase):
    def test_instrumentation(self):
        """Verify that instrumentation methods are instrumenting and
        removing as expected.
        """
        instrumentor = otel_starlette.StarletteInstrumentor()
        original = applications.Starlette
        instrumentor.instrument()
        try:
            instrumented = applications.Starlette
            self.assertIsNot(original, instrumented)
        finally:
            instrumentor.uninstrument()

        should_be_original = applications.Starlette
        self.assertIs(original, should_be_original)
