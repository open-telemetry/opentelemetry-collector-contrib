from opentracing import SpanContext as OpenTracingSpanContext

from ddtrace.context import Context as DatadogContext


class SpanContext(OpenTracingSpanContext):
    """Implementation of the OpenTracing span context."""

    def __init__(self, trace_id=None, span_id=None,
                 sampling_priority=None, baggage=None, ddcontext=None):
        # create a new dict for the baggage if it is not provided
        # NOTE: it would be preferable to use opentracing.SpanContext.EMPTY_BAGGAGE
        #       but it is mutable.
        # see: opentracing-python/blob/8775c7bfc57fd66e1c8bcf9a54d3e434d37544f9/opentracing/span.py#L30
        baggage = baggage or {}

        if ddcontext is not None:
            self._dd_context = ddcontext
        else:
            self._dd_context = DatadogContext(
                trace_id=trace_id,
                span_id=span_id,
                sampling_priority=sampling_priority,
            )

        self._baggage = dict(baggage)

    @property
    def baggage(self):
        return self._baggage

    def set_baggage_item(self, key, value):
        """Sets a baggage item in this span context.

        Note that this operation mutates the baggage of this span context
        """
        self.baggage[key] = value

    def with_baggage_item(self, key, value):
        """Returns a copy of this span with a new baggage item.

        Useful for instantiating new child span contexts.
        """
        baggage = dict(self._baggage)
        baggage[key] = value
        return SpanContext(ddcontext=self._dd_context, baggage=baggage)

    def get_baggage_item(self, key):
        """Gets a baggage item in this span context."""
        return self.baggage.get(key, None)
