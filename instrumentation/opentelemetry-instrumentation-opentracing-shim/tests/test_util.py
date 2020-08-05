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

import time
import unittest

from opentelemetry.instrumentation.opentracing_shim import util
from opentelemetry.util import time_ns


class TestUtil(unittest.TestCase):
    def test_event_name_from_kv(self):
        # Test basic behavior.
        event_name = "send HTTP request"
        res = util.event_name_from_kv({"event": event_name, "foo": "bar"})
        self.assertEqual(res, event_name)

        # Test None.
        res = util.event_name_from_kv(None)
        self.assertEqual(res, util.DEFAULT_EVENT_NAME)

        # Test empty dict.
        res = util.event_name_from_kv({})
        self.assertEqual(res, util.DEFAULT_EVENT_NAME)

        # Test missing `event` field.
        res = util.event_name_from_kv({"foo": "bar"})
        self.assertEqual(res, util.DEFAULT_EVENT_NAME)

    def test_time_seconds_to_ns(self):
        time_seconds = time.time()
        result = util.time_seconds_to_ns(time_seconds)

        self.assertEqual(result, int(time_seconds * 1e9))

    def test_time_seconds_from_ns(self):
        time_nanoseconds = time_ns()
        result = util.time_seconds_from_ns(time_nanoseconds)

        self.assertEqual(result, time_nanoseconds / 1e9)

    def test_time_conversion_precision(self):
        """Verify time conversion from seconds to nanoseconds and vice versa is
        accurate enough.
        """

        time_seconds = 1570484241.9501917
        time_nanoseconds = util.time_seconds_to_ns(time_seconds)
        result = util.time_seconds_from_ns(time_nanoseconds)

        # Tolerate inaccuracies of less than a microsecond.
        # TODO: Put a link to an explanation in the docs.
        # TODO: This seems to work consistently, but we should find out the
        # biggest possible loss of precision.
        self.assertAlmostEqual(result, time_seconds, places=6)
