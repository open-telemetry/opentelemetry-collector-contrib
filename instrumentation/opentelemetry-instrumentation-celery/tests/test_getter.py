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

from unittest import TestCase, mock

from opentelemetry.instrumentation.celery import CeleryGetter


class TestCeleryGetter(TestCase):
    def test_get_none(self):
        getter = CeleryGetter()
        carrier = {}
        val = getter.get(carrier, "test")
        self.assertIsNone(val)

    def test_get_str(self):
        mock_obj = mock.Mock()
        getter = CeleryGetter()
        mock_obj.test = "val"
        val = getter.get(mock_obj, "test")
        self.assertEqual(val, ("val",))

    def test_get_iter(self):
        mock_obj = mock.Mock()
        getter = CeleryGetter()
        mock_obj.test = ["val"]
        val = getter.get(mock_obj, "test")
        self.assertEqual(val, ["val"])

    def test_keys(self):
        getter = CeleryGetter()
        keys = getter.keys({})
        self.assertEqual(keys, [])
