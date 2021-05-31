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

from requests.structures import CaseInsensitiveDict

import opentelemetry.trace as trace_api
from opentelemetry.context import Context
from opentelemetry.sdk.extension.aws.trace.propagation.aws_xray_format import (
    TRACE_HEADER_KEY,
    AwsXRayFormat,
)
from opentelemetry.trace import (
    DEFAULT_TRACE_OPTIONS,
    DEFAULT_TRACE_STATE,
    INVALID_SPAN_CONTEXT,
    SpanContext,
    TraceFlags,
    TraceState,
    set_span_in_context,
)

TRACE_ID_BASE16 = "8a3c60f7d188f8fa79d48a391a778fa6"

SPAN_ID_BASE16 = "53995c3f42cd8ad8"

# Propagators Usage Methods


def get_as_list(dict_object, key):
    value = dict_object.get(key)
    return [value] if value is not None else []


# Inject Methods


def build_test_current_context(
    trace_id=int(TRACE_ID_BASE16, 16),
    span_id=int(SPAN_ID_BASE16, 16),
    is_remote=True,
    trace_flags=DEFAULT_TRACE_OPTIONS,
    trace_state=DEFAULT_TRACE_STATE,
):
    return set_span_in_context(
        trace_api.NonRecordingSpan(
            build_test_span_context(
                trace_id, span_id, is_remote, trace_flags, trace_state
            )
        )
    )


# Extract Methods


def get_nested_span_context(parent_context):
    return trace_api.get_current_span(parent_context).get_span_context()


# Helper Methods


def build_test_span_context(
    trace_id=int(TRACE_ID_BASE16, 16),
    span_id=int(SPAN_ID_BASE16, 16),
    is_remote=True,
    trace_flags=DEFAULT_TRACE_OPTIONS,
    trace_state=DEFAULT_TRACE_STATE,
):
    return SpanContext(trace_id, span_id, is_remote, trace_flags, trace_state,)


class AwsXRayPropagatorTest(unittest.TestCase):
    XRAY_PROPAGATOR = AwsXRayFormat()

    # Inject Tests

    def test_inject_into_non_sampled_context(self):
        carrier = CaseInsensitiveDict()

        AwsXRayPropagatorTest.XRAY_PROPAGATOR.inject(
            carrier, build_test_current_context(),
        )

        injected_items = set(carrier.items())
        expected_items = set(
            CaseInsensitiveDict(
                {
                    TRACE_HEADER_KEY: "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=0"
                }
            ).items()
        )

        self.assertEqual(injected_items, expected_items)

    def test_inject_into_sampled_context(self):
        carrier = CaseInsensitiveDict()

        AwsXRayPropagatorTest.XRAY_PROPAGATOR.inject(
            carrier,
            build_test_current_context(
                trace_flags=TraceFlags(TraceFlags.SAMPLED)
            ),
        )

        injected_items = set(carrier.items())
        expected_items = set(
            CaseInsensitiveDict(
                {
                    TRACE_HEADER_KEY: "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=1"
                }
            ).items()
        )

        self.assertEqual(injected_items, expected_items)

    def test_inject_into_context_with_non_default_state(self):
        carrier = CaseInsensitiveDict()

        AwsXRayPropagatorTest.XRAY_PROPAGATOR.inject(
            carrier,
            build_test_current_context(
                trace_state=TraceState([("foo", "bar")])
            ),
        )

        # TODO: (NathanielRN) Assert trace state when the propagator supports it
        injected_items = set(carrier.items())
        expected_items = set(
            CaseInsensitiveDict(
                {
                    TRACE_HEADER_KEY: "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=0"
                }
            ).items()
        )

        self.assertEqual(injected_items, expected_items)

    def test_inject_reported_fields_matches_carrier_fields(self):
        carrier = CaseInsensitiveDict()

        AwsXRayPropagatorTest.XRAY_PROPAGATOR.inject(
            carrier, build_test_current_context(),
        )

        injected_keys = set(carrier.keys())

        self.assertEqual(
            injected_keys, AwsXRayPropagatorTest.XRAY_PROPAGATOR.fields
        )

    # Extract Tests

    def test_extract_empty_carrier_to_explicit_ctx(self):
        orig_ctx = Context({"k1": "v1"})
        context_with_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            CaseInsensitiveDict(), orig_ctx
        )
        self.assertDictEqual(orig_ctx, context_with_extracted)

    def test_extract_empty_carrier_to_implicit_ctx(self):
        context_with_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            CaseInsensitiveDict()
        )
        self.assertDictEqual(Context(), context_with_extracted)

    def test_extract_not_sampled_context(self):
        context_with_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            CaseInsensitiveDict(
                {
                    TRACE_HEADER_KEY: "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=0"
                }
            ),
        )

        self.assertEqual(
            get_nested_span_context(context_with_extracted),
            build_test_span_context(),
        )

    def test_extract_sampled_context(self):
        context_with_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            CaseInsensitiveDict(
                {
                    TRACE_HEADER_KEY: "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=1"
                }
            ),
        )

        self.assertEqual(
            get_nested_span_context(context_with_extracted),
            build_test_span_context(
                trace_flags=TraceFlags(TraceFlags.SAMPLED)
            ),
        )

    def test_extract_different_order(self):
        context_with_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            CaseInsensitiveDict(
                {
                    TRACE_HEADER_KEY: "Sampled=0;Parent=53995c3f42cd8ad8;Root=1-8a3c60f7-d188f8fa79d48a391a778fa6"
                }
            ),
        )

        self.assertEqual(
            get_nested_span_context(context_with_extracted),
            build_test_span_context(),
        )

    def test_extract_with_additional_fields(self):
        context_with_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            CaseInsensitiveDict(
                {
                    TRACE_HEADER_KEY: "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=0;Foo=Bar"
                }
            ),
        )

        self.assertEqual(
            get_nested_span_context(context_with_extracted),
            build_test_span_context(),
        )

    def test_extract_with_extra_whitespace(self):
        context_with_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            CaseInsensitiveDict(
                {
                    TRACE_HEADER_KEY: "  Root  =  1-8a3c60f7-d188f8fa79d48a391a778fa6  ;  Parent  =  53995c3f42cd8ad8  ;  Sampled  =  0   "
                }
            ),
        )

        self.assertEqual(
            get_nested_span_context(context_with_extracted),
            build_test_span_context(),
        )

    def test_extract_invalid_xray_trace_header(self):
        context_with_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            CaseInsensitiveDict({TRACE_HEADER_KEY: ""}),
        )

        self.assertEqual(
            get_nested_span_context(context_with_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_invalid_to_explicit_ctx(self):
        trace_headers = [
            "Root=1-12345678-abcdefghijklmnopqrstuvwx;Parent=53995c3f42cd8ad8;Sampled=0",  # invalid trace id
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa600;Parent=53995c3f42cd8ad8;Sampled=0",  # invalid size trace id
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=abcdefghijklmnop;Sampled=0",  # invalid span id
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad800;Sampled=0"  # invalid size span id
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=",  # no sampled flag
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=011",  # invalid size sampled
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=a",  # non numeric sampled flag
        ]
        for trace_header in trace_headers:
            with self.subTest(trace_header=trace_header):
                orig_ctx = Context({"k1": "v1"})

                ctx = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
                    CaseInsensitiveDict({TRACE_HEADER_KEY: trace_header}),
                    orig_ctx,
                )

                self.assertDictEqual(orig_ctx, ctx)

    def test_extract_invalid_to_implicit_ctx(self):
        trace_headers = [
            "Root=1-12345678-abcdefghijklmnopqrstuvwx;Parent=53995c3f42cd8ad8;Sampled=0",  # invalid trace id
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa600;Parent=53995c3f42cd8ad8;Sampled=0",  # invalid size trace id
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=abcdefghijklmnop;Sampled=0",  # invalid span id
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad800;Sampled=0"  # invalid size span id
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=",  # no sampled flag
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=011",  # invalid size sampled
            "Root=1-8a3c60f7-d188f8fa79d48a391a778fa6;Parent=53995c3f42cd8ad8;Sampled=a",  # non numeric sampled flag
        ]
        for trace_header in trace_headers:
            with self.subTest(trace_header=trace_header):
                ctx = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
                    CaseInsensitiveDict({TRACE_HEADER_KEY: trace_header}),
                )

                self.assertDictEqual(Context(), ctx)

    @patch(
        "opentelemetry.sdk.extension.aws.trace."
        "propagation.aws_xray_format.trace"
    )
    def test_fields(self, mock_trace):
        """Make sure the fields attribute returns the fields used in inject"""

        mock_trace.configure_mock(
            **{
                "get_current_span.return_value": Mock(
                    **{
                        "get_span_context.return_value": Mock(
                            **{"is_valid": True, "trace_id": 1, "span_id": 1}
                        )
                    }
                )
            }
        )

        mock_setter = Mock()

        AwsXRayPropagatorTest.XRAY_PROPAGATOR.inject({}, setter=mock_setter)

        inject_fields = set()

        for call in mock_setter.mock_calls:
            inject_fields.add(call[1][1])

        self.assertEqual(
            AwsXRayPropagatorTest.XRAY_PROPAGATOR.fields, inject_fields
        )
