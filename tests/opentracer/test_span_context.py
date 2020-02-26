from ddtrace.opentracer.span_context import SpanContext


class TestSpanContext(object):

    def test_init(self):
        """Make sure span context creation is fine."""
        span_ctx = SpanContext()
        assert span_ctx

    def test_baggage(self):
        """Ensure baggage passed is the resulting baggage of the span context."""
        baggage = {
            'some': 'stuff',
        }

        span_ctx = SpanContext(baggage=baggage)

        assert span_ctx.baggage == baggage

    def test_with_baggage_item(self):
        """Should allow immutable extension of new span contexts."""
        baggage = {
            '1': 1,
        }

        first_ctx = SpanContext(baggage=baggage)

        second_ctx = first_ctx.with_baggage_item('2', 2)

        assert '2' not in first_ctx.baggage
        assert second_ctx.baggage is not first_ctx.baggage

    def test_span_context_immutable_baggage(self):
        """Ensure that two different span contexts do not share baggage."""
        ctx1 = SpanContext()
        ctx1.set_baggage_item('test', 3)
        ctx2 = SpanContext()
        assert 'test' not in ctx2._baggage
