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

import logging
import typing

import opentelemetry.trace as trace
from opentelemetry.context import Context
from opentelemetry.trace.propagation.textmap import (
    Getter,
    Setter,
    TextMapPropagator,
    TextMapPropagatorT,
)

_logger = logging.getLogger(__name__)


class AwsXRayFormat(TextMapPropagator):
    """Propagator for the AWS X-Ray Trace Header propagation protocol.

    See: https://docs.aws.amazon.com/xray/latest/devguide/xray-concepts.html#xray-concepts-tracingheader
    """

    # AWS
    TRACE_HEADER_KEY = "X-Amzn-Trace-Id"

    KV_PAIR_DELIMITER = ";"
    KEY_AND_VALUE_DELIMITER = "="

    TRACE_ID_KEY = "Root"
    TRACE_ID_LENGTH = 35
    TRACE_ID_VERSION = "1"
    TRACE_ID_DELIMITER = "-"
    TRACE_ID_DELIMITER_INDEX_1 = 1
    TRACE_ID_DELIMITER_INDEX_2 = 10
    TRACE_ID_FIRST_PART_LENGTH = 8

    PARENT_ID_KEY = "Parent"
    PARENT_ID_LENGTH = 16

    SAMPLED_FLAG_KEY = "Sampled"
    SAMPLED_FLAG_LENGTH = 1
    IS_SAMPLED = "1"
    NOT_SAMPLED = "0"

    # pylint: disable=too-many-locals
    # pylint: disable=too-many-return-statements
    # pylint: disable=too-many-branches
    # pylint: disable=too-many-statements
    def extract(
        self,
        getter: Getter[TextMapPropagatorT],
        carrier: TextMapPropagatorT,
        context: typing.Optional[Context] = None,
    ) -> Context:
        trace_header_list = getter(carrier, self.TRACE_HEADER_KEY)
        trace_header_list = getter.get(carrier, self.TRACE_HEADER_KEY)

        if not trace_header_list or len(trace_header_list) != 1:
            return trace.set_span_in_context(
                trace.INVALID_SPAN, context=context
            )

        trace_header = trace_header_list[0]

        if not trace_header:
            return trace.set_span_in_context(
                trace.INVALID_SPAN, context=context
            )

        trace_id = trace.INVALID_TRACE_ID
        span_id = trace.INVALID_SPAN_ID
        sampled = False

        next_kv_pair_start = 0

        while next_kv_pair_start < len(trace_header):
            try:
                kv_pair_delimiter_index = trace_header.index(
                    self.KV_PAIR_DELIMITER, next_kv_pair_start
                )
                kv_pair_subset = trace_header[
                    next_kv_pair_start:kv_pair_delimiter_index
                ]
                next_kv_pair_start = kv_pair_delimiter_index + 1
            except ValueError:
                kv_pair_subset = trace_header[next_kv_pair_start:]
                next_kv_pair_start = len(trace_header)

            stripped_kv_pair = kv_pair_subset.strip()

            try:
                key_and_value_delimiter_index = stripped_kv_pair.index(
                    self.KEY_AND_VALUE_DELIMITER
                )
            except ValueError:
                _logger.error(
                    (
                        "Error parsing X-Ray trace header. Invalid key value pair: %s. Returning INVALID span context.",
                        kv_pair_subset,
                    )
                )
                return trace.set_span_in_context(
                    trace.INVALID_SPAN, context=context
                )

            value = stripped_kv_pair[key_and_value_delimiter_index + 1 :]

            if stripped_kv_pair.startswith(self.TRACE_ID_KEY):
                if (
                    len(value) != self.TRACE_ID_LENGTH
                    or not value.startswith(self.TRACE_ID_VERSION)
                    or value[self.TRACE_ID_DELIMITER_INDEX_1]
                    != self.TRACE_ID_DELIMITER
                    or value[self.TRACE_ID_DELIMITER_INDEX_2]
                    != self.TRACE_ID_DELIMITER
                ):
                    _logger.error(
                        (
                            "Invalid TraceId in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    )
                    return trace.set_span_in_context(
                        trace.INVALID_SPAN, context=context
                    )

                timestamp_subset = value[
                    self.TRACE_ID_DELIMITER_INDEX_1
                    + 1 : self.TRACE_ID_DELIMITER_INDEX_2
                ]
                unique_id_subset = value[
                    self.TRACE_ID_DELIMITER_INDEX_2 + 1 : self.TRACE_ID_LENGTH
                ]
                try:
                    trace_id = int(timestamp_subset + unique_id_subset, 16)
                except ValueError:
                    _logger.error(
                        (
                            "Invalid TraceId in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    )
                    return trace.set_span_in_context(
                        trace.INVALID_SPAN, context=context
                    )
            elif stripped_kv_pair.startswith(self.PARENT_ID_KEY):
                if len(value) != self.PARENT_ID_LENGTH:
                    _logger.error(
                        (
                            "Invalid ParentId in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    )
                    return trace.set_span_in_context(
                        trace.INVALID_SPAN, context=context
                    )

                try:
                    span_id = int(value, 16)
                except ValueError:
                    _logger.error(
                        (
                            "Invalid TraceId in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    )
                    return trace.set_span_in_context(
                        trace.INVALID_SPAN, context=context
                    )
            elif stripped_kv_pair.startswith(self.SAMPLED_FLAG_KEY):
                is_sampled_flag_valid = True

                if len(value) != self.SAMPLED_FLAG_LENGTH:
                    is_sampled_flag_valid = False

                if is_sampled_flag_valid:
                    sampled_flag = value[0]
                    if sampled_flag == self.IS_SAMPLED:
                        sampled = True
                    elif sampled_flag == self.NOT_SAMPLED:
                        sampled = False
                    else:
                        is_sampled_flag_valid = False

                if not is_sampled_flag_valid:
                    _logger.error(
                        (
                            "Invalid Sampling flag in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    )
                    return trace.set_span_in_context(
                        trace.INVALID_SPAN, context=context
                    )

        options = 0
        if sampled:
            options |= trace.TraceFlags.SAMPLED

        span_context = trace.SpanContext(
            trace_id=trace_id,
            span_id=span_id,
            is_remote=True,
            trace_flags=trace.TraceFlags(options),
            trace_state=trace.TraceState(),
        )

        if not span_context.is_valid:
            _logger.error(
                "Invalid Span Extracted. Insertting INVALID span into provided context."
            )
            return trace.set_span_in_context(
                trace.INVALID_SPAN, context=context
            )

        return trace.set_span_in_context(
            trace.DefaultSpan(span_context), context=context
        )

    def inject(
        self,
        set_in_carrier: Setter[TextMapPropagatorT],
        carrier: TextMapPropagatorT,
        context: typing.Optional[Context] = None,
    ) -> None:
        span = trace.get_current_span(context=context)

        span_context = span.get_span_context()
        if not span_context.is_valid:
            return

        otel_trace_id = "{:032x}".format(span_context.trace_id)
        xray_trace_id = (
            self.TRACE_ID_VERSION
            + self.TRACE_ID_DELIMITER
            + otel_trace_id[: self.TRACE_ID_FIRST_PART_LENGTH]
            + self.TRACE_ID_DELIMITER
            + otel_trace_id[self.TRACE_ID_FIRST_PART_LENGTH :]
        )

        parent_id = "{:016x}".format(span_context.span_id)

        sampling_flag = (
            self.IS_SAMPLED
            if span_context.trace_flags & trace.TraceFlags.SAMPLED
            else self.NOT_SAMPLED
        )

        # TODO: Add OT trace state to the X-Ray trace header

        trace_header = (
            self.TRACE_ID_KEY
            + self.KEY_AND_VALUE_DELIMITER
            + xray_trace_id
            + self.KV_PAIR_DELIMITER
            + self.PARENT_ID_KEY
            + self.KEY_AND_VALUE_DELIMITER
            + parent_id
            + self.KV_PAIR_DELIMITER
            + self.SAMPLED_FLAG_KEY
            + self.KEY_AND_VALUE_DELIMITER
            + sampling_flag
        )

        set_in_carrier(
            carrier, self.TRACE_HEADER_KEY, trace_header,
        )
