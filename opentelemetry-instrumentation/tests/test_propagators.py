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

# pylint: disable=protected-access

import unittest

from opentelemetry import trace
from opentelemetry.instrumentation import propagators
from opentelemetry.instrumentation.propagators import (
    DictHeaderSetter,
    TraceResponsePropagator,
    get_global_response_propagator,
    set_global_response_propagator,
)
from opentelemetry.test.test_base import TestBase


class TestGlobals(TestBase):
    def test_get_set(self):
        original = propagators._RESPONSE_PROPAGATOR

        propagators._RESPONSE_PROPAGATOR = None
        self.assertIsNone(get_global_response_propagator())

        prop = TraceResponsePropagator()
        set_global_response_propagator(prop)
        self.assertIs(prop, get_global_response_propagator())

        propagators._RESPONSE_PROPAGATOR = original


class TestDictHeaderSetter(unittest.TestCase):
    def test_simple(self):
        setter = DictHeaderSetter()
        carrier = {}
        setter.set(carrier, "kk", "vv")
        self.assertIn("kk", carrier)
        self.assertEqual(carrier["kk"], "vv")

    def test_append(self):
        setter = DictHeaderSetter()
        carrier = {"kk": "old"}
        setter.set(carrier, "kk", "vv")
        self.assertIn("kk", carrier)
        self.assertEqual(carrier["kk"], "old, vv")


class TestTraceResponsePropagator(TestBase):
    def test_inject(self):
        span = trace.NonRecordingSpan(
            trace.SpanContext(
                trace_id=1,
                span_id=2,
                is_remote=False,
                trace_flags=trace.DEFAULT_TRACE_OPTIONS,
                trace_state=trace.DEFAULT_TRACE_STATE,
            ),
        )

        ctx = trace.set_span_in_context(span)
        prop = TraceResponsePropagator()
        carrier = {}
        prop.inject(carrier, ctx)
        self.assertEqual(
            carrier["Access-Control-Expose-Headers"], "traceresponse"
        )
        self.assertEqual(
            carrier["traceresponse"],
            "00-00000000000000000000000000000001-0000000000000002-00",
        )
