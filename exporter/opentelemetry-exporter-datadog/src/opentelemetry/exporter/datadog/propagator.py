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

import typing

from opentelemetry import trace
from opentelemetry.context import Context
from opentelemetry.exporter.datadog import constants
from opentelemetry.propagators.textmap import (
    CarrierT,
    Getter,
    Setter,
    TextMapPropagator,
    default_getter,
    default_setter,
)
from opentelemetry.trace import get_current_span, set_span_in_context


class DatadogFormat(TextMapPropagator):
    """Propagator for the Datadog HTTP header format."""

    TRACE_ID_KEY = "x-datadog-trace-id"
    PARENT_ID_KEY = "x-datadog-parent-id"
    SAMPLING_PRIORITY_KEY = "x-datadog-sampling-priority"
    ORIGIN_KEY = "x-datadog-origin"

    def extract(
        self,
        carrier: CarrierT,
        context: typing.Optional[Context] = None,
        getter: Getter = default_getter,
    ) -> Context:
        if context is None:
            context = Context()

        trace_id = extract_first_element(
            getter.get(carrier, self.TRACE_ID_KEY)
        )

        span_id = extract_first_element(
            getter.get(carrier, self.PARENT_ID_KEY)
        )

        sampled = extract_first_element(
            getter.get(carrier, self.SAMPLING_PRIORITY_KEY)
        )

        origin = extract_first_element(getter.get(carrier, self.ORIGIN_KEY))

        trace_flags = trace.TraceFlags()
        if sampled and int(sampled) in (
            constants.AUTO_KEEP,
            constants.USER_KEEP,
        ):
            trace_flags = trace.TraceFlags(trace.TraceFlags.SAMPLED)

        if trace_id is None or span_id is None:
            return context

        trace_state = []
        if origin is not None:
            trace_state.append((constants.DD_ORIGIN, origin))
        span_context = trace.SpanContext(
            trace_id=int(trace_id),
            span_id=int(span_id),
            is_remote=True,
            trace_flags=trace_flags,
            trace_state=trace.TraceState(trace_state),
        )

        return set_span_in_context(
            trace.NonRecordingSpan(span_context), context
        )

    def inject(
        self,
        carrier: CarrierT,
        context: typing.Optional[Context] = None,
        setter: Setter = default_setter,
    ) -> None:
        span = get_current_span(context)
        span_context = span.get_span_context()
        if span_context == trace.INVALID_SPAN_CONTEXT:
            return
        sampled = (trace.TraceFlags.SAMPLED & span.context.trace_flags) != 0
        setter.set(
            carrier, self.TRACE_ID_KEY, format_trace_id(span.context.trace_id),
        )
        setter.set(
            carrier, self.PARENT_ID_KEY, format_span_id(span.context.span_id)
        )
        setter.set(
            carrier,
            self.SAMPLING_PRIORITY_KEY,
            str(constants.AUTO_KEEP if sampled else constants.AUTO_REJECT),
        )
        if constants.DD_ORIGIN in span.context.trace_state:
            setter.set(
                carrier,
                self.ORIGIN_KEY,
                span.context.trace_state[constants.DD_ORIGIN],
            )

    @property
    def fields(self):
        """Returns a set with the fields set in `inject`.

        See
        `opentelemetry.propagators.textmap.TextMapPropagator.fields`
        """
        return {
            self.TRACE_ID_KEY,
            self.PARENT_ID_KEY,
            self.SAMPLING_PRIORITY_KEY,
            self.ORIGIN_KEY,
        }


def format_trace_id(trace_id: int) -> str:
    """Format the trace id for Datadog."""
    return str(trace_id & 0xFFFFFFFFFFFFFFFF)


def format_span_id(span_id: int) -> str:
    """Format the span id for Datadog."""
    return str(span_id)


def extract_first_element(
    items: typing.Iterable[CarrierT],
) -> typing.Optional[CarrierT]:
    if items is None:
        return None
    return next(iter(items), None)
