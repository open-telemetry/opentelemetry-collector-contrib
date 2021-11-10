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

from opentelemetry.baggage import get_all, set_baggage
from opentelemetry.context import Context
from opentelemetry.propagators.ot_trace import (
    OT_BAGGAGE_PREFIX,
    OT_SAMPLED_HEADER,
    OT_SPAN_ID_HEADER,
    OT_TRACE_ID_HEADER,
    OTTracePropagator,
)
from opentelemetry.sdk.trace import _Span
from opentelemetry.trace import (
    INVALID_TRACE_ID,
    SpanContext,
    TraceFlags,
    set_span_in_context,
)
from opentelemetry.trace.propagation import get_current_span


class TestOTTracePropagator(TestCase):

    ot_trace_propagator = OTTracePropagator()

    def carrier_inject(self, trace_id, span_id, is_remote, trace_flags):

        carrier = {}

        self.ot_trace_propagator.inject(
            carrier,
            set_span_in_context(
                _Span(
                    "child",
                    context=SpanContext(
                        trace_id=trace_id,
                        span_id=span_id,
                        is_remote=is_remote,
                        trace_flags=trace_flags,
                    ),
                )
            ),
        )

        return carrier

    def test_inject_short_trace_id_short_span_id(self):
        carrier = self.carrier_inject(
            int("1", 16),
            int("2", 16),
            True,
            TraceFlags.SAMPLED,
        )

        self.assertEqual(carrier[OT_TRACE_ID_HEADER], "1")
        self.assertEqual(carrier[OT_SPAN_ID_HEADER], "2")

    def test_inject_trace_id_span_id_true(self):
        """Test valid trace_id, span_id and sampled true"""
        carrier = self.carrier_inject(
            int("80f198ee56343ba864fe8b2a57d3eff7", 16),
            int("e457b5a2e4d86bd1", 16),
            True,
            TraceFlags.SAMPLED,
        )

        self.assertEqual(carrier[OT_TRACE_ID_HEADER], "64fe8b2a57d3eff7")
        self.assertEqual(carrier[OT_SPAN_ID_HEADER], "e457b5a2e4d86bd1")
        self.assertEqual(carrier[OT_SAMPLED_HEADER], "true")

    def test_inject_trace_id_span_id_false(self):
        """Test valid trace_id, span_id and sampled true"""
        carrier = self.carrier_inject(
            int("80f198ee56343ba864fe8b2a57d3eff7", 16),
            int("e457b5a2e4d86bd1", 16),
            False,
            TraceFlags.DEFAULT,
        )

        self.assertEqual(carrier[OT_TRACE_ID_HEADER], "64fe8b2a57d3eff7")
        self.assertEqual(carrier[OT_SPAN_ID_HEADER], "e457b5a2e4d86bd1")
        self.assertEqual(carrier[OT_SAMPLED_HEADER], "false")

    def test_inject_truncate_traceid(self):
        """Test that traceid is truncated to 64 bits"""

        self.assertEqual(
            self.carrier_inject(
                int("80f198ee56343ba864fe8b2a57d3eff7", 16),
                int("e457b5a2e4d86bd1", 16),
                True,
                TraceFlags.DEFAULT,
            )[OT_TRACE_ID_HEADER],
            "64fe8b2a57d3eff7",
        )

    def test_inject_sampled_true(self):
        """Test that sampled true trace flags are injected"""

        self.assertEqual(
            self.carrier_inject(
                int("80f198ee56343ba864fe8b2a57d3eff7", 16),
                int("e457b5a2e4d86bd1", 16),
                True,
                TraceFlags.SAMPLED,
            )[OT_SAMPLED_HEADER],
            "true",
        )

    def test_inject_sampled_false(self):
        """Test that sampled false trace flags are injected"""

        self.assertEqual(
            self.carrier_inject(
                int("80f198ee56343ba864fe8b2a57d3eff7", 16),
                int("e457b5a2e4d86bd1", 16),
                True,
                TraceFlags.DEFAULT,
            )[OT_SAMPLED_HEADER],
            "false",
        )

    def test_inject_invalid_trace_id(self):
        """Test that no attributes are injected if the trace_id is invalid"""

        self.assertEqual(
            self.carrier_inject(
                INVALID_TRACE_ID,
                int("e457b5a2e4d86bd1", 16),
                True,
                TraceFlags.SAMPLED,
            ),
            {},
        )

    def test_inject_set_baggage(self):
        """Test that baggage is set"""

        carrier = {}

        self.ot_trace_propagator.inject(
            carrier,
            set_baggage(
                "key",
                "value",
                context=set_span_in_context(
                    _Span(
                        "child",
                        SpanContext(
                            trace_id=int(
                                "80f198ee56343ba864fe8b2a57d3eff7", 16
                            ),
                            span_id=int("e457b5a2e4d86bd1", 16),
                            is_remote=True,
                            trace_flags=TraceFlags.SAMPLED,
                        ),
                    )
                ),
            ),
        )

        self.assertEqual(carrier["".join([OT_BAGGAGE_PREFIX, "key"])], "value")

    def test_inject_invalid_baggage_keys(self):
        """Test that invalid baggage keys are not set"""

        carrier = {}

        self.ot_trace_propagator.inject(
            carrier,
            set_baggage(
                "(",
                "value",
                context=set_span_in_context(
                    _Span(
                        "child",
                        SpanContext(
                            trace_id=int(
                                "80f198ee56343ba864fe8b2a57d3eff7", 16
                            ),
                            span_id=int("e457b5a2e4d86bd1", 16),
                            is_remote=True,
                            trace_flags=TraceFlags.SAMPLED,
                        ),
                    )
                ),
            ),
        )

        self.assertNotIn("".join([OT_BAGGAGE_PREFIX, "!"]), carrier.keys())

    def test_inject_invalid_baggage_values(self):
        """Test that invalid baggage values are not set"""

        carrier = {}

        self.ot_trace_propagator.inject(
            carrier,
            set_baggage(
                "key",
                "α",
                context=set_span_in_context(
                    _Span(
                        "child",
                        SpanContext(
                            trace_id=int(
                                "80f198ee56343ba864fe8b2a57d3eff7", 16
                            ),
                            span_id=int("e457b5a2e4d86bd1", 16),
                            is_remote=True,
                            trace_flags=TraceFlags.SAMPLED,
                        ),
                    )
                ),
            ),
        )

        self.assertNotIn("".join([OT_BAGGAGE_PREFIX, "key"]), carrier.keys())

    def test_extract_trace_id_span_id_sampled_true(self):
        """Test valid trace_id, span_id and sampled true"""

        span_context = get_current_span(
            self.ot_trace_propagator.extract(
                {
                    OT_TRACE_ID_HEADER: "80f198ee56343ba864fe8b2a57d3eff7",
                    OT_SPAN_ID_HEADER: "e457b5a2e4d86bd1",
                    OT_SAMPLED_HEADER: "true",
                },
            )
        ).get_span_context()

        self.assertEqual(
            hex(span_context.trace_id)[2:], "80f198ee56343ba864fe8b2a57d3eff7"
        )
        self.assertEqual(hex(span_context.span_id)[2:], "e457b5a2e4d86bd1")
        self.assertTrue(span_context.is_remote)
        self.assertEqual(span_context.trace_flags, TraceFlags.SAMPLED)
        self.assertIsInstance(
            get_current_span().get_span_context().trace_flags, TraceFlags
        )

    def test_extract_trace_id_span_id_sampled_false(self):
        """Test valid trace_id, span_id and sampled false"""

        span_context = get_current_span(
            self.ot_trace_propagator.extract(
                {
                    OT_TRACE_ID_HEADER: "80f198ee56343ba864fe8b2a57d3eff7",
                    OT_SPAN_ID_HEADER: "e457b5a2e4d86bd1",
                    OT_SAMPLED_HEADER: "false",
                },
            )
        ).get_span_context()

        self.assertEqual(
            hex(span_context.trace_id)[2:], "80f198ee56343ba864fe8b2a57d3eff7"
        )
        self.assertEqual(hex(span_context.span_id)[2:], "e457b5a2e4d86bd1")
        self.assertTrue(span_context.is_remote)
        self.assertEqual(span_context.trace_flags, TraceFlags.DEFAULT)
        self.assertIsInstance(
            get_current_span().get_span_context().trace_flags, TraceFlags
        )

    def test_extract_invalid_trace_header_to_explict_ctx(self):
        invalid_headers = [
            ("abc123!", "e457b5a2e4d86bd1"),  # malformed trace id
            ("64fe8b2a57d3eff7", "abc123!"),  # malformed span id
            ("0" * 32, "e457b5a2e4d86bd1"),  # invalid trace id
            ("64fe8b2a57d3eff7", "0" * 16),  # invalid span id
        ]
        for trace_id, span_id in invalid_headers:
            with self.subTest(trace_id=trace_id, span_id=span_id):
                orig_ctx = Context({"k1": "v1"})

                ctx = self.ot_trace_propagator.extract(
                    {
                        OT_TRACE_ID_HEADER: trace_id,
                        OT_SPAN_ID_HEADER: span_id,
                        OT_SAMPLED_HEADER: "false",
                    },
                    orig_ctx,
                )
                self.assertDictEqual(orig_ctx, ctx)

    def test_extract_invalid_trace_header_to_implicit_ctx(self):
        invalid_headers = [
            ("abc123!", "e457b5a2e4d86bd1"),  # malformed trace id
            ("64fe8b2a57d3eff7", "abc123!"),  # malformed span id
            ("0" * 32, "e457b5a2e4d86bd1"),  # invalid trace id
            ("64fe8b2a57d3eff7", "0" * 16),  # invalid span id
        ]
        for trace_id, span_id in invalid_headers:
            with self.subTest(trace_id=trace_id, span_id=span_id):
                ctx = self.ot_trace_propagator.extract(
                    {
                        OT_TRACE_ID_HEADER: trace_id,
                        OT_SPAN_ID_HEADER: span_id,
                        OT_SAMPLED_HEADER: "false",
                    }
                )
                self.assertDictEqual(Context(), ctx)

    def test_extract_baggage(self):
        """Test baggage extraction"""

        context = self.ot_trace_propagator.extract(
            {
                OT_TRACE_ID_HEADER: "64fe8b2a57d3eff7",
                OT_SPAN_ID_HEADER: "e457b5a2e4d86bd1",
                OT_SAMPLED_HEADER: "false",
                "".join([OT_BAGGAGE_PREFIX, "abc"]): "abc",
                "".join([OT_BAGGAGE_PREFIX, "def"]): "def",
            },
        )
        span_context = get_current_span(context).get_span_context()

        self.assertEqual(hex(span_context.trace_id)[2:], "64fe8b2a57d3eff7")
        self.assertEqual(hex(span_context.span_id)[2:], "e457b5a2e4d86bd1")
        self.assertTrue(span_context.is_remote)
        self.assertEqual(span_context.trace_flags, TraceFlags.DEFAULT)

        baggage = get_all(context)

        self.assertEqual(baggage["abc"], "abc")
        self.assertEqual(baggage["def"], "def")

    def test_extract_empty_to_explicit_ctx(self):
        """Test extraction when no headers are present"""
        orig_ctx = Context({"k1": "v1"})
        ctx = self.ot_trace_propagator.extract({}, orig_ctx)

        self.assertDictEqual(orig_ctx, ctx)

    def test_extract_empty_to_implicit_ctx(self):
        ctx = self.ot_trace_propagator.extract({})
        self.assertDictEqual(Context(), ctx)
