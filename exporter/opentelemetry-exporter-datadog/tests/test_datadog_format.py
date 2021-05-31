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
from unittest.mock import Mock, patch

from opentelemetry import trace as trace_api
from opentelemetry.context import Context
from opentelemetry.exporter.datadog import constants, propagator
from opentelemetry.sdk import trace
from opentelemetry.sdk.trace.id_generator import RandomIdGenerator
from opentelemetry.trace import get_current_span, set_span_in_context

FORMAT = propagator.DatadogFormat()


class TestDatadogFormat(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        id_generator = RandomIdGenerator()
        cls.serialized_trace_id = propagator.format_trace_id(
            id_generator.generate_trace_id()
        )
        cls.serialized_parent_id = propagator.format_span_id(
            id_generator.generate_span_id()
        )
        cls.serialized_origin = "origin-service"

    def test_extract_malformed_headers_to_explicit_ctx(self):
        """Test with no Datadog headers"""
        orig_ctx = Context({"k1": "v1"})
        malformed_trace_id_key = FORMAT.TRACE_ID_KEY + "-x"
        malformed_parent_id_key = FORMAT.PARENT_ID_KEY + "-x"
        context = FORMAT.extract(
            {
                malformed_trace_id_key: self.serialized_trace_id,
                malformed_parent_id_key: self.serialized_parent_id,
            },
            orig_ctx,
        )
        self.assertDictEqual(orig_ctx, context)

    def test_extract_malformed_headers_to_implicit_ctx(self):
        malformed_trace_id_key = FORMAT.TRACE_ID_KEY + "-x"
        malformed_parent_id_key = FORMAT.PARENT_ID_KEY + "-x"
        context = FORMAT.extract(
            {
                malformed_trace_id_key: self.serialized_trace_id,
                malformed_parent_id_key: self.serialized_parent_id,
            }
        )
        self.assertDictEqual(Context(), context)

    def test_extract_missing_trace_id_to_explicit_ctx(self):
        """If a trace id is missing, populate an invalid trace id."""
        orig_ctx = Context({"k1": "v1"})
        carrier = {FORMAT.PARENT_ID_KEY: self.serialized_parent_id}

        ctx = FORMAT.extract(carrier, orig_ctx)
        self.assertDictEqual(orig_ctx, ctx)

    def test_extract_missing_trace_id_to_implicit_ctx(self):
        carrier = {FORMAT.PARENT_ID_KEY: self.serialized_parent_id}

        ctx = FORMAT.extract(carrier)
        self.assertDictEqual(Context(), ctx)

    def test_extract_missing_parent_id_to_explicit_ctx(self):
        """If a parent id is missing, populate an invalid trace id."""
        orig_ctx = Context({"k1": "v1"})
        carrier = {FORMAT.TRACE_ID_KEY: self.serialized_trace_id}

        ctx = FORMAT.extract(carrier, orig_ctx)
        self.assertDictEqual(orig_ctx, ctx)

    def test_extract_missing_parent_id_to_implicit_ctx(self):
        carrier = {FORMAT.TRACE_ID_KEY: self.serialized_trace_id}

        ctx = FORMAT.extract(carrier)
        self.assertDictEqual(Context(), ctx)

    def test_context_propagation(self):
        """Test the propagation of Datadog headers."""
        parent_span_context = get_current_span(
            FORMAT.extract(
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
                RandomIdGenerator().generate_span_id(),
                is_remote=False,
                trace_flags=parent_span_context.trace_flags,
                trace_state=parent_span_context.trace_state,
            ),
            parent=parent_span_context,
        )

        child_carrier = {}
        child_context = set_span_in_context(child)
        FORMAT.inject(child_carrier, context=child_context)

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
                RandomIdGenerator().generate_span_id(),
                is_remote=False,
                trace_flags=parent_span_context.trace_flags,
                trace_state=parent_span_context.trace_state,
            ),
            parent=parent_span_context,
        )

        child_carrier = {}
        child_context = set_span_in_context(child)
        FORMAT.inject(child_carrier, context=child_context)

        self.assertEqual(
            child_carrier[FORMAT.SAMPLING_PRIORITY_KEY],
            str(constants.AUTO_REJECT),
        )

    @patch("opentelemetry.exporter.datadog.propagator.get_current_span")
    def test_fields(self, mock_get_current_span):
        """Make sure the fields attribute returns the fields used in inject"""

        tracer = trace.TracerProvider().get_tracer("sdk_tracer_provider")

        mock_setter = Mock()

        mock_get_current_span.configure_mock(
            **{
                "return_value": Mock(
                    **{
                        "get_span_context.return_value": None,
                        "context.trace_flags": 0,
                        "context.trace_id": 1,
                        "context.trace_state": {constants.DD_ORIGIN: 0},
                    }
                )
            }
        )

        with tracer.start_as_current_span("parent"):
            with tracer.start_as_current_span("child"):
                FORMAT.inject({}, setter=mock_setter)

        inject_fields = set()

        for call in mock_setter.mock_calls:
            inject_fields.add(call[1][1])

        self.assertEqual(FORMAT.fields, inject_fields)
