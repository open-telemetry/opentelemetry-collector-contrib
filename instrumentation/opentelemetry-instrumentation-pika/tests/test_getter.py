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

from opentelemetry.instrumentation.pika.utils import _PikaGetter


class TestPikaGetter(TestCase):
    def setUp(self) -> None:
        self.getter = _PikaGetter()

    def test_get_none(self) -> None:
        carrier = {}
        value = self.getter.get(carrier, "test")
        self.assertIsNone(value)

    def test_get_value(self) -> None:
        key = "test"
        value = "value"
        carrier = {key: value}
        val = self.getter.get(carrier, key)
        self.assertEqual(val, [value])

    def test_keys(self):
        keys = self.getter.keys({})
        self.assertEqual(keys, [])
