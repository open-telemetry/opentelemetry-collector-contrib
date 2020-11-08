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


class AwsParseTraceHeaderError(Exception):
    def __init__(self, message):
        super().__init__()
        self.message = message


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

        try:
            trace_id, span_id, sampled = self._extract_span_properties(
                trace_header
            )
        except AwsParseTraceHeaderError as err:
            _logger.debug(err.message)
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
            _logger.debug(
                "Invalid Span Extracted. Insertting INVALID span into provided context."
            )
            return trace.set_span_in_context(
                trace.INVALID_SPAN, context=context
            )

        return trace.set_span_in_context(
            trace.DefaultSpan(span_context), context=context
        )

    def _extract_span_properties(self, trace_header):
        trace_id = trace.INVALID_TRACE_ID
        span_id = trace.INVALID_SPAN_ID
        sampled = False

        for kv_pair_str in trace_header.split(self.KV_PAIR_DELIMITER):
            try:
                key_str, value_str = kv_pair_str.split(
                    self.KEY_AND_VALUE_DELIMITER
                )
                key, value = key_str.strip(), value_str.strip()
            except ValueError as ex:
                raise AwsParseTraceHeaderError(
                    (
                        "Error parsing X-Ray trace header. Invalid key value pair: %s. Returning INVALID span context.",
                        kv_pair_str,
                    )
                ) from ex
            if key == self.TRACE_ID_KEY:
                if not self._validate_trace_id(value):
                    raise AwsParseTraceHeaderError(
                        (
                            "Invalid TraceId in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    )

                try:
                    trace_id = self._parse_trace_id(value)
                except ValueError as ex:
                    raise AwsParseTraceHeaderError(
                        (
                            "Invalid TraceId in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    ) from ex
            elif key == self.PARENT_ID_KEY:
                if not self._validate_span_id(value):
                    raise AwsParseTraceHeaderError(
                        (
                            "Invalid ParentId in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    )

                try:
                    span_id = AwsXRayFormat._parse_span_id(value)
                except ValueError as ex:
                    raise AwsParseTraceHeaderError(
                        (
                            "Invalid TraceId in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    ) from ex
            elif key == self.SAMPLED_FLAG_KEY:
                if not self._validate_sampled_flag(value):
                    raise AwsParseTraceHeaderError(
                        (
                            "Invalid Sampling flag in X-Ray trace header: '%s' with value '%s'. Returning INVALID span context.",
                            self.TRACE_HEADER_KEY,
                            trace_header,
                        )
                    )

                sampled = self._parse_sampled_flag(value)

        return trace_id, span_id, sampled

    def _validate_trace_id(self, trace_id_str):
        return (
            len(trace_id_str) == self.TRACE_ID_LENGTH
            and trace_id_str.startswith(self.TRACE_ID_VERSION)
            and trace_id_str[self.TRACE_ID_DELIMITER_INDEX_1]
            == self.TRACE_ID_DELIMITER
            and trace_id_str[self.TRACE_ID_DELIMITER_INDEX_2]
            == self.TRACE_ID_DELIMITER
        )

    def _parse_trace_id(self, trace_id_str):
        timestamp_subset = trace_id_str[
            self.TRACE_ID_DELIMITER_INDEX_1
            + 1 : self.TRACE_ID_DELIMITER_INDEX_2
        ]
        unique_id_subset = trace_id_str[
            self.TRACE_ID_DELIMITER_INDEX_2 + 1 : self.TRACE_ID_LENGTH
        ]
        return int(timestamp_subset + unique_id_subset, 16)

    def _validate_span_id(self, span_id_str):
        return len(span_id_str) == self.PARENT_ID_LENGTH

    @staticmethod
    def _parse_span_id(span_id_str):
        return int(span_id_str, 16)

    def _validate_sampled_flag(self, sampled_flag_str):
        return len(
            sampled_flag_str
        ) == self.SAMPLED_FLAG_LENGTH and sampled_flag_str in (
            self.IS_SAMPLED,
            self.NOT_SAMPLED,
        )

    def _parse_sampled_flag(self, sampled_flag_str):
        return sampled_flag_str[0] == self.IS_SAMPLED

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
