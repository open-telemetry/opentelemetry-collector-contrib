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

from unittest import TestCase

from opentelemetry.instrumentation.asgi import ASGIGetter


class TestASGIGetter(TestCase):
    def test_get_none(self):
        getter = ASGIGetter()
        carrier = {}
        val = getter.get(carrier, "test")
        self.assertIsNone(val)

    def test_get_(self):
        getter = ASGIGetter()
        carrier = {"headers": [(b"test-key", b"val")]}
        expected_val = ["val"]
        self.assertEqual(
            getter.get(carrier, "Test-Key"),
            expected_val,
            "Should be case insensitive",
        )
        self.assertEqual(
            getter.get(carrier, "test-key"),
            expected_val,
            "Should be case insensitive",
        )
        self.assertEqual(
            getter.get(carrier, "TEST-KEY"),
            expected_val,
            "Should be case insensitive",
        )

    def test_keys(self):
        getter = ASGIGetter()
        keys = getter.keys({})
        self.assertEqual(keys, [])
