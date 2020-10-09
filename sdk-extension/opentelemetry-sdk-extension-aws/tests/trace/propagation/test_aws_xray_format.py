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

from requests.structures import CaseInsensitiveDict

import opentelemetry.trace as trace_api
from opentelemetry.sdk.extension.aws.trace.propagation.aws_xray_format import (
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


def build_test_context(
    trace_id=int(TRACE_ID_BASE16, 16),
    span_id=int(SPAN_ID_BASE16, 16),
    is_remote=True,
    trace_flags=DEFAULT_TRACE_OPTIONS,
    trace_state=DEFAULT_TRACE_STATE,
):
    return set_span_in_context(
        trace_api.DefaultSpan(
            SpanContext(
                trace_id, span_id, is_remote, trace_flags, trace_state,
            )
        )
    )


def build_dict_with_xray_trace_header(
    trace_id="{}{}{}{}{}".format(
        AwsXRayFormat.TRACE_ID_VERSION,
        AwsXRayFormat.TRACE_ID_DELIMITER,
        TRACE_ID_BASE16[: AwsXRayFormat.TRACE_ID_FIRST_PART_LENGTH],
        AwsXRayFormat.TRACE_ID_DELIMITER,
        TRACE_ID_BASE16[AwsXRayFormat.TRACE_ID_FIRST_PART_LENGTH :],
    ),
    span_id=SPAN_ID_BASE16,
    sampled="0",
):
    carrier = CaseInsensitiveDict()

    carrier[AwsXRayFormat.TRACE_HEADER_KEY] = "".join(
        [
            "{}{}{}{}".format(
                AwsXRayFormat.TRACE_ID_KEY,
                AwsXRayFormat.KEY_AND_VALUE_DELIMITER,
                trace_id,
                AwsXRayFormat.KV_PAIR_DELIMITER,
            ),
            "{}{}{}{}".format(
                AwsXRayFormat.PARENT_ID_KEY,
                AwsXRayFormat.KEY_AND_VALUE_DELIMITER,
                span_id,
                AwsXRayFormat.KV_PAIR_DELIMITER,
            ),
            "{}{}{}".format(
                AwsXRayFormat.SAMPLED_FLAG_KEY,
                AwsXRayFormat.KEY_AND_VALUE_DELIMITER,
                sampled,
            ),
        ]
    )

    return carrier


# Extract Methods


def get_extracted_span_context(encompassing_context):
    return trace_api.get_current_span(encompassing_context).get_span_context()


class AwsXRayPropagatorTest(unittest.TestCase):
    carrier_setter = CaseInsensitiveDict.__setitem__
    carrier_getter = get_as_list
    XRAY_PROPAGATOR = AwsXRayFormat()

    # Inject Tests

    def test_inject_into_non_sampled_context(self):
        carrier = CaseInsensitiveDict()

        AwsXRayPropagatorTest.XRAY_PROPAGATOR.inject(
            AwsXRayPropagatorTest.carrier_setter, carrier, build_test_context()
        )

        self.assertTrue(
            set(carrier.items()).issubset(
                set(build_dict_with_xray_trace_header().items())
            ),
            "Failed to inject into context that was not yet sampled",
        )

    def test_inject_into_sampled_context(self):
        carrier = CaseInsensitiveDict()

        AwsXRayPropagatorTest.XRAY_PROPAGATOR.inject(
            AwsXRayPropagatorTest.carrier_setter,
            carrier,
            build_test_context(trace_flags=TraceFlags(TraceFlags.SAMPLED)),
        )

        self.assertTrue(
            set(carrier.items()).issubset(
                set(build_dict_with_xray_trace_header(sampled="1").items(),)
            ),
            "Failed to inject into context that was already sampled",
        )

    def test_inject_into_context_with_non_default_state(self):
        carrier = CaseInsensitiveDict()

        AwsXRayPropagatorTest.XRAY_PROPAGATOR.inject(
            AwsXRayPropagatorTest.carrier_setter,
            carrier,
            build_test_context(trace_state=TraceState({"foo": "bar"})),
        )

        # TODO: (NathanielRN) Assert trace state when the propagator supports it
        self.assertTrue(
            set(carrier.items()).issubset(
                set(build_dict_with_xray_trace_header().items(),)
            ),
            "Failed to inject into context with non default state",
        )

    # Extract Tests

    def test_extract_empty_carrier_from_none_carrier(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter, CaseInsensitiveDict()
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_empty_carrier_from_invalid_context(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter, CaseInsensitiveDict()
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_not_sampled_context(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            build_dict_with_xray_trace_header(),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            get_extracted_span_context(build_test_context()),
        )

    def test_extract_sampled_context(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            build_dict_with_xray_trace_header(sampled="1"),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            get_extracted_span_context(
                build_test_context(trace_flags=TraceFlags(TraceFlags.SAMPLED))
            ),
        )

    def test_extract_different_order(self):
        default_xray_trace_header_dict = build_dict_with_xray_trace_header()
        trace_header_components = default_xray_trace_header_dict[
            AwsXRayFormat.TRACE_HEADER_KEY
        ].split(AwsXRayFormat.KV_PAIR_DELIMITER)
        reversed_trace_header_components = AwsXRayFormat.KV_PAIR_DELIMITER.join(
            trace_header_components[::-1]
        )
        xray_trace_header_dict_in_different_order = CaseInsensitiveDict(
            {AwsXRayFormat.TRACE_HEADER_KEY: reversed_trace_header_components}
        )
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            xray_trace_header_dict_in_different_order,
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            get_extracted_span_context(build_test_context()),
        )

    def test_extract_with_additional_fields(self):
        default_xray_trace_header_dict = build_dict_with_xray_trace_header()
        xray_trace_header_dict_with_extra_fields = CaseInsensitiveDict(
            {
                AwsXRayFormat.TRACE_HEADER_KEY: default_xray_trace_header_dict[
                    AwsXRayFormat.TRACE_HEADER_KEY
                ]
                + ";Foo=Bar"
            }
        )
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            xray_trace_header_dict_with_extra_fields,
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            get_extracted_span_context(build_test_context()),
        )

    def test_extract_invalid_xray_trace_header(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            CaseInsensitiveDict({AwsXRayFormat.TRACE_HEADER_KEY: ""}),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_invalid_trace_id(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            build_dict_with_xray_trace_header(
                trace_id="abcdefghijklmnopqrstuvwxyz123456"
            ),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_invalid_trace_id_size(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            build_dict_with_xray_trace_header(
                trace_id="1-8a3c60f7-d188f8fa79d48a391a778fa600"
            ),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_invalid_span_id(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            build_dict_with_xray_trace_header(span_id="abcdefghijklmnop"),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_invalid_span_id_size(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            build_dict_with_xray_trace_header(span_id="53995c3f42cd8ad800"),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_invalid_empty_sampled_flag(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            build_dict_with_xray_trace_header(sampled=""),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_invalid_sampled_flag_size(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            build_dict_with_xray_trace_header(sampled="10002"),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )

    def test_extract_invalid_non_numeric_sampled_flag(self):
        actual_context_encompassing_extracted = AwsXRayPropagatorTest.XRAY_PROPAGATOR.extract(
            AwsXRayPropagatorTest.carrier_getter,
            build_dict_with_xray_trace_header(sampled="a"),
        )

        self.assertEqual(
            get_extracted_span_context(actual_context_encompassing_extracted),
            INVALID_SPAN_CONTEXT,
        )
