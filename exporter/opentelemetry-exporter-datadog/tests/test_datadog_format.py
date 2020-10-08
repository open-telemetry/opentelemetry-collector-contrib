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

from opentelemetry import trace as trace_api
from opentelemetry.exporter.datadog import constants, propagator
from opentelemetry.sdk import trace
from opentelemetry.trace import get_current_span, set_span_in_context

FORMAT = propagator.DatadogFormat()


def get_as_list(dict_object, key):
    value = dict_object.get(key)
    return [value] if value is not None else []


class TestDatadogFormat(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        ids_generator = trace_api.RandomIdsGenerator()
        cls.serialized_trace_id = propagator.format_trace_id(
            ids_generator.generate_trace_id()
        )
        cls.serialized_parent_id = propagator.format_span_id(
            ids_generator.generate_span_id()
        )
        cls.serialized_origin = "origin-service"

    def test_malformed_headers(self):
        """Test with no Datadog headers"""
        malformed_trace_id_key = FORMAT.TRACE_ID_KEY + "-x"
        malformed_parent_id_key = FORMAT.PARENT_ID_KEY + "-x"
        context = get_current_span(
            FORMAT.extract(
                get_as_list,
                {
                    malformed_trace_id_key: self.serialized_trace_id,
                    malformed_parent_id_key: self.serialized_parent_id,
                },
            )
        ).get_span_context()

        self.assertNotEqual(context.trace_id, int(self.serialized_trace_id))
        self.assertNotEqual(context.span_id, int(self.serialized_parent_id))
        self.assertFalse(context.is_remote)

    def test_missing_trace_id(self):
        """If a trace id is missing, populate an invalid trace id."""
        carrier = {
            FORMAT.PARENT_ID_KEY: self.serialized_parent_id,
        }

        ctx = FORMAT.extract(get_as_list, carrier)
        span_context = get_current_span(ctx).get_span_context()
        self.assertEqual(span_context.trace_id, trace_api.INVALID_TRACE_ID)

    def test_missing_parent_id(self):
        """If a parent id is missing, populate an invalid trace id."""
        carrier = {
            FORMAT.TRACE_ID_KEY: self.serialized_trace_id,
        }

        ctx = FORMAT.extract(get_as_list, carrier)
        span_context = get_current_span(ctx).get_span_context()
        self.assertEqual(span_context.span_id, trace_api.INVALID_SPAN_ID)

    def test_context_propagation(self):
        """Test the propagation of Datadog headers."""
        parent_span_context = get_current_span(
            FORMAT.extract(
                get_as_list,
                {
                    FORMAT.TRACE_ID_KEY: self.serialized_trace_id,
                    FORMAT.PARENT_ID_KEY: self.serialized_parent_id,
                    FORMAT.SAMPLING_PRIORITY_KEY: str(constants.AUTO_KEEP),
                    FORMAT.ORIGIN_KEY: self.serialized_origin,
                },
            )
        ).get_span_context()

        self.assertEqual(
            parent_span_context.trace_id, int(self.serialized_trace_id)
        )
        self.assertEqual(
            parent_span_context.span_id, int(self.serialized_parent_id)
        )
        self.assertEqual(parent_span_context.trace_flags, constants.AUTO_KEEP)
        self.assertEqual(
            parent_span_context.trace_state.get(constants.DD_ORIGIN),
            self.serialized_origin,
        )
        self.assertTrue(parent_span_context.is_remote)

        child = trace._Span(
            "child",
            trace_api.SpanContext(
                parent_span_context.trace_id,
                trace_api.RandomIdsGenerator().generate_span_id(),
                is_remote=False,
                trace_flags=parent_span_context.trace_flags,
                trace_state=parent_span_context.trace_state,
            ),
            parent=parent_span_context,
        )

        child_carrier = {}
        child_context = set_span_in_context(child)
        FORMAT.inject(dict.__setitem__, child_carrier, context=child_context)

        self.assertEqual(
            child_carrier[FORMAT.TRACE_ID_KEY], self.serialized_trace_id
        )
        self.assertEqual(
            child_carrier[FORMAT.PARENT_ID_KEY], str(child.context.span_id)
        )
        self.assertEqual(
            child_carrier[FORMAT.SAMPLING_PRIORITY_KEY],
            str(constants.AUTO_KEEP),
        )
        self.assertEqual(
            child_carrier.get(FORMAT.ORIGIN_KEY), self.serialized_origin
        )

    def test_sampling_priority_auto_reject(self):
        """Test sampling priority rejected."""
        parent_span_context = get_current_span(
            FORMAT.extract(
                get_as_list,
                {
                    FORMAT.TRACE_ID_KEY: self.serialized_trace_id,
                    FORMAT.PARENT_ID_KEY: self.serialized_parent_id,
                    FORMAT.SAMPLING_PRIORITY_KEY: str(constants.AUTO_REJECT),
                },
            )
        ).get_span_context()

        self.assertEqual(
            parent_span_context.trace_flags, constants.AUTO_REJECT
        )

        child = trace._Span(
            "child",
            trace_api.SpanContext(
                parent_span_context.trace_id,
                trace_api.RandomIdsGenerator().generate_span_id(),
                is_remote=False,
                trace_flags=parent_span_context.trace_flags,
                trace_state=parent_span_context.trace_state,
            ),
            parent=parent_span_context,
        )

        child_carrier = {}
        child_context = set_span_in_context(child)
        FORMAT.inject(dict.__setitem__, child_carrier, context=child_context)

        self.assertEqual(
            child_carrier[FORMAT.SAMPLING_PRIORITY_KEY],
            str(constants.AUTO_REJECT),
        )
