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
# type: ignore

from logging import WARNING
from unittest import TestCase

from opentelemetry.instrumentation.instrumentor import BaseInstrumentor


class TestInstrumentor(TestCase):
    class Instrumentor(BaseInstrumentor):
        def _instrument(self, **kwargs):
            return "instrumented"

        def _uninstrument(self, **kwargs):
            return "uninstrumented"

        def instrumentation_dependencies(self):
            return []

    def test_protect(self):
        instrumentor = self.Instrumentor()

        with self.assertLogs(level=WARNING):
            self.assertIs(instrumentor.uninstrument(), None)

        self.assertEqual(instrumentor.instrument(), "instrumented")

        with self.assertLogs(level=WARNING):
            self.assertIs(instrumentor.instrument(), None)

        self.assertEqual(instrumentor.uninstrument(), "uninstrumented")

        with self.assertLogs(level=WARNING):
            self.assertIs(instrumentor.uninstrument(), None)

    def test_singleton(self):
        self.assertIs(self.Instrumentor(), self.Instrumentor())
