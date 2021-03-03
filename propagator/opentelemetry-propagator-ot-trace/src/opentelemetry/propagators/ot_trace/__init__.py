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

from re import compile as re_compile
from typing import Iterable, Optional

from opentelemetry.baggage import get_all, set_baggage
from opentelemetry.context import Context
from opentelemetry.propagators.textmap import (
    Getter,
    Setter,
    TextMapPropagator,
    TextMapPropagatorT,
)
from opentelemetry.trace import (
    INVALID_SPAN_ID,
    INVALID_TRACE_ID,
    NonRecordingSpan,
    SpanContext,
    TraceFlags,
    get_current_span,
    set_span_in_context,
)

OT_TRACE_ID_HEADER = "ot-tracer-traceid"
OT_SPAN_ID_HEADER = "ot-tracer-spanid"
OT_SAMPLED_HEADER = "ot-tracer-sampled"
OT_BAGGAGE_PREFIX = "ot-baggage-"

_valid_header_name = re_compile(r"[\w_^`!#$%&'*+.|~]+")
_valid_header_value = re_compile(r"[\t\x20-\x7e\x80-\xff]+")
_valid_extract_traceid = re_compile(r"[0-9a-f]{1,32}")
_valid_extract_spanid = re_compile(r"[0-9a-f]{1,16}")


class OTTracePropagator(TextMapPropagator):
    """Propagator for the OTTrace HTTP header format"""

    def extract(
        self,
        getter: Getter[TextMapPropagatorT],
        carrier: TextMapPropagatorT,
        context: Optional[Context] = None,
    ) -> Context:

        traceid = _extract_first_element(
            getter.get(carrier, OT_TRACE_ID_HEADER)
        )

        spanid = _extract_first_element(getter.get(carrier, OT_SPAN_ID_HEADER))

        sampled = _extract_first_element(
            getter.get(carrier, OT_SAMPLED_HEADER)
        )

        if sampled == "true":
            traceflags = TraceFlags.SAMPLED
        else:
            traceflags = TraceFlags.DEFAULT

        if (
            traceid != INVALID_TRACE_ID
            and _valid_extract_traceid.fullmatch(traceid) is not None
            and spanid != INVALID_SPAN_ID
            and _valid_extract_spanid.fullmatch(spanid) is not None
        ):
            context = set_span_in_context(
                NonRecordingSpan(
                    SpanContext(
                        trace_id=int(traceid, 16),
                        span_id=int(spanid, 16),
                        is_remote=True,
                        trace_flags=traceflags,
                    )
                ),
                context,
            )

            baggage = get_all(context) or {}

            for key in getter.keys(carrier):

                if not key.startswith(OT_BAGGAGE_PREFIX):
                    continue

                baggage[
                    key[len(OT_BAGGAGE_PREFIX) :]
                ] = _extract_first_element(getter.get(carrier, key))

            for key, value in baggage.items():
                context = set_baggage(key, value, context)

        return context

    def inject(
        self,
        set_in_carrier: Setter[TextMapPropagatorT],
        carrier: TextMapPropagatorT,
        context: Optional[Context] = None,
    ) -> None:

        span_context = get_current_span(context).get_span_context()

        if span_context.trace_id == INVALID_TRACE_ID:
            return

        set_in_carrier(
            carrier, OT_TRACE_ID_HEADER, hex(span_context.trace_id)[2:][-16:]
        )
        set_in_carrier(
            carrier, OT_SPAN_ID_HEADER, hex(span_context.span_id)[2:][-16:],
        )

        if span_context.trace_flags == TraceFlags.SAMPLED:
            traceflags = "true"
        else:
            traceflags = "false"

        set_in_carrier(carrier, OT_SAMPLED_HEADER, traceflags)

        baggage = get_all(context)

        if not baggage:
            return

        for header_name, header_value in baggage.items():

            if (
                _valid_header_name.fullmatch(header_name) is None
                or _valid_header_value.fullmatch(header_value) is None
            ):
                continue

            set_in_carrier(
                carrier,
                "".join([OT_BAGGAGE_PREFIX, header_name]),
                header_value,
            )

    @property
    def fields(self):
        """Returns a set with the fields set in `inject`.

        See
        `opentelemetry.propagators.textmap.TextMapPropagator.fields`
        """
        return {
            OT_TRACE_ID_HEADER,
            OT_SPAN_ID_HEADER,
            OT_SAMPLED_HEADER,
        }


def _extract_first_element(
    items: Iterable[TextMapPropagatorT],
) -> Optional[TextMapPropagatorT]:
    if items is None:
        return None
    return next(iter(items), None)
