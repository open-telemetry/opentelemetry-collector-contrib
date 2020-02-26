import ddtrace
import opentracing
from opentracing import Format

from ddtrace.opentracer.span_context import SpanContext


class TestTracerCompatibility(object):
    """Ensure that our opentracer produces results in the underlying ddtracer."""

    def test_ottracer_uses_global_ddtracer(self):
        """Ensure that the opentracer will by default use the global ddtracer
        as its underlying Datadog tracer.
        """
        tracer = ddtrace.opentracer.Tracer()
        assert tracer._dd_tracer is ddtrace.tracer

    def test_custom_ddtracer(self):
        """A user should be able to specify their own Datadog tracer instance if
        they wish.
        """
        custom_dd_tracer = ddtrace.Tracer()
        tracer = ddtrace.opentracer.Tracer(dd_tracer=custom_dd_tracer)
        assert tracer._dd_tracer is custom_dd_tracer

    def test_ot_dd_global_tracers(self, global_tracer):
        """Ensure our test function opentracer_init() prep"""
        ot_tracer = global_tracer
        dd_tracer = global_tracer._dd_tracer

        # check all the global references
        assert ot_tracer is opentracing.tracer
        assert ot_tracer._dd_tracer is dd_tracer
        assert dd_tracer is ddtrace.tracer

    def test_ot_dd_nested_trace(self, ot_tracer, dd_tracer, writer):
        """Ensure intertwined usage of the opentracer and ddtracer."""

        with ot_tracer.start_span('my_ot_span') as ot_span:
            with dd_tracer.trace('my_dd_span') as dd_span:
                pass
        spans = writer.pop()
        assert len(spans) == 2

        # confirm the ordering
        assert spans[0] is ot_span._dd_span
        assert spans[1] is dd_span

        # check the parenting
        assert spans[0].parent_id is None
        assert spans[1].parent_id == spans[0].span_id

    def test_dd_ot_nested_trace(self, ot_tracer, dd_tracer, writer):
        """Ensure intertwined usage of the opentracer and ddtracer."""
        with dd_tracer.trace('my_dd_span') as dd_span:
            with ot_tracer.start_span('my_ot_span') as ot_span:
                pass
        spans = writer.pop()
        assert len(spans) == 2

        # confirm the ordering
        assert spans[0] is dd_span
        assert spans[1] is ot_span._dd_span

        # check the parenting
        assert spans[0].parent_id is None
        assert spans[1].parent_id is spans[0].span_id

    def test_ot_dd_ot_dd_nested_trace(self, ot_tracer, dd_tracer, writer):
        """Ensure intertwined usage of the opentracer and ddtracer."""
        with ot_tracer.start_span('my_ot_span') as ot_span:
            with dd_tracer.trace('my_dd_span') as dd_span:
                with ot_tracer.start_span('my_ot_span') as ot_span2:
                    with dd_tracer.trace('my_dd_span') as dd_span2:
                        pass

        spans = writer.pop()
        assert len(spans) == 4

        # confirm the ordering
        assert spans[0] is ot_span._dd_span
        assert spans[1] is dd_span
        assert spans[2] is ot_span2._dd_span
        assert spans[3] is dd_span2

        # check the parenting
        assert spans[0].parent_id is None
        assert spans[1].parent_id is spans[0].span_id
        assert spans[2].parent_id is spans[1].span_id
        assert spans[3].parent_id is spans[2].span_id

    def test_ot_ot_dd_ot_dd_nested_trace_active(self, ot_tracer, dd_tracer, writer):
        """Ensure intertwined usage of the opentracer and ddtracer."""
        with ot_tracer.start_active_span('my_ot_span') as ot_scope:
            with ot_tracer.start_active_span('my_ot_span') as ot_scope2:
                with dd_tracer.trace('my_dd_span') as dd_span:
                    with ot_tracer.start_active_span('my_ot_span') as ot_scope3:
                        with dd_tracer.trace('my_dd_span') as dd_span2:
                            pass

        spans = writer.pop()
        assert len(spans) == 5

        # confirm the ordering
        assert spans[0] is ot_scope.span._dd_span
        assert spans[1] is ot_scope2.span._dd_span
        assert spans[2] is dd_span
        assert spans[3] is ot_scope3.span._dd_span
        assert spans[4] is dd_span2

        # check the parenting
        assert spans[0].parent_id is None
        assert spans[1].parent_id == spans[0].span_id
        assert spans[2].parent_id == spans[1].span_id
        assert spans[3].parent_id == spans[2].span_id
        assert spans[4].parent_id == spans[3].span_id

    def test_consecutive_trace(self, ot_tracer, dd_tracer, writer):
        """Ensure consecutive usage of the opentracer and ddtracer."""
        with ot_tracer.start_active_span('my_ot_span') as ot_scope:
            pass

        with dd_tracer.trace('my_dd_span') as dd_span:
            pass

        with ot_tracer.start_active_span('my_ot_span') as ot_scope2:
            pass

        with dd_tracer.trace('my_dd_span') as dd_span2:
            pass

        spans = writer.pop()
        assert len(spans) == 4

        # confirm the ordering
        assert spans[0] is ot_scope.span._dd_span
        assert spans[1] is dd_span
        assert spans[2] is ot_scope2.span._dd_span
        assert spans[3] is dd_span2

        # check the parenting
        assert spans[0].parent_id is None
        assert spans[1].parent_id is None
        assert spans[2].parent_id is None
        assert spans[3].parent_id is None

    def test_ddtrace_wrapped_fn(self, ot_tracer, dd_tracer, writer):
        """Ensure ddtrace wrapped functions work with the opentracer"""

        @dd_tracer.wrap()
        def fn():
            with ot_tracer.start_span('ot_span_inner'):
                pass

        with ot_tracer.start_active_span('ot_span_outer'):
            fn()

        spans = writer.pop()
        assert len(spans) == 3

        # confirm the ordering
        assert spans[0].name == 'ot_span_outer'
        assert spans[1].name == 'tests.opentracer.test_dd_compatibility.fn'
        assert spans[2].name == 'ot_span_inner'

        # check the parenting
        assert spans[0].parent_id is None
        assert spans[1].parent_id is spans[0].span_id
        assert spans[2].parent_id is spans[1].span_id

    def test_distributed_trace_propagation(self, ot_tracer, dd_tracer, writer):
        """Ensure that a propagated span context is properly activated."""
        span_ctx = SpanContext(trace_id=123, span_id=456)
        carrier = {}
        ot_tracer.inject(span_ctx, Format.HTTP_HEADERS, carrier)

        # extract should activate the span so that a subsequent start_span
        # will inherit from the propagated span context
        ot_tracer.extract(Format.HTTP_HEADERS, carrier)

        with dd_tracer.trace('test') as span:
            pass

        assert span.parent_id == 456
        assert span.trace_id == 123

        spans = writer.pop()
        assert len(spans) == 1
