import pytest
from ddtrace.opentracer.span import Span
from ..test_tracer import get_dummy_tracer


@pytest.fixture
def nop_tracer():
    from ddtrace.opentracer import Tracer
    tracer = Tracer(service_name='mysvc', config={})
    # use the same test tracer used by the primary tests
    tracer._tracer = get_dummy_tracer()
    return tracer


@pytest.fixture
def nop_span_ctx():
    from ddtrace.ext.priority import AUTO_KEEP
    from ddtrace.opentracer.span_context import SpanContext
    return SpanContext(sampling_priority=AUTO_KEEP)


@pytest.fixture
def nop_span(nop_tracer, nop_span_ctx):
    return Span(nop_tracer, nop_span_ctx, 'my_op_name')


class TestSpan(object):
    """Test the Datadog OpenTracing Span implementation."""

    def test_init(self, nop_tracer, nop_span_ctx):
        """Very basic test for skeleton code"""
        span = Span(nop_tracer, nop_span_ctx, 'my_op_name')
        assert not span.finished

    def test_tags(self, nop_span):
        """Set a tag and get it back."""
        nop_span.set_tag('test', 23)
        assert nop_span._get_metric('test') == 23

    def test_set_baggage(self, nop_span):
        """Test setting baggage."""
        r = nop_span.set_baggage_item('test', 23)
        assert r is nop_span

        r = nop_span.set_baggage_item('1', 1).set_baggage_item('2', 2)
        assert r is nop_span

    def test_get_baggage(self, nop_span):
        """Test setting and getting baggage."""
        # test a single item
        nop_span.set_baggage_item('test', 23)
        assert int(nop_span.get_baggage_item('test')) == 23

        # test multiple items
        nop_span.set_baggage_item('1', '1').set_baggage_item('2', 2)
        assert int(nop_span.get_baggage_item('test')) == 23
        assert nop_span.get_baggage_item('1') == '1'
        assert int(nop_span.get_baggage_item('2')) == 2

    def test_log_kv(self, nop_span):
        """Ensure logging values doesn't break anything."""
        # just log a bunch of values
        nop_span.log_kv({'myval': 2})
        nop_span.log_kv({'myval2': 3})
        nop_span.log_kv({'myval3': 5})
        nop_span.log_kv({'myval': 2})

    def test_log_dd_kv(self, nop_span):
        """Ensure keys that can be handled by our impl. are indeed handled. """
        import traceback
        from ddtrace.ext import errors

        stack_trace = str(traceback.format_stack())
        nop_span.log_kv({
            'event': 'error',
            'error': 3,
            'message': 'my error message',
            'stack': stack_trace,
        })

        # Ensure error flag is set...
        assert nop_span._dd_span.error
        # ...and that error tags are set with the correct key
        assert nop_span._get_tag(errors.ERROR_STACK) == stack_trace
        assert nop_span._get_tag(errors.ERROR_MSG) == 'my error message'
        assert nop_span._get_metric(errors.ERROR_TYPE) == 3

    def test_operation_name(self, nop_span):
        """Sanity check for setting the operation name."""
        # just try setting the operation name
        nop_span.set_operation_name('new_op_name')
        assert nop_span._dd_span.name == 'new_op_name'

    def test_context_manager(self, nop_span):
        """Test the span context manager."""
        import time

        assert not nop_span.finished
        # run the context manager but since the span has not been added
        # to the span context, we will not get any traces
        with nop_span:
            time.sleep(0.005)

        # span should be finished when the context manager exits
        assert nop_span.finished

        # there should be no traces (see above comment)
        spans = nop_span.tracer._tracer.writer.pop()
        assert len(spans) == 0

    def test_immutable_span_context(self, nop_span):
        """Ensure span contexts are immutable."""
        before_ctx = nop_span._context
        nop_span.set_baggage_item('key', 'value')
        after_ctx = nop_span._context
        # should be different contexts
        assert before_ctx is not after_ctx


class TestSpanCompatibility(object):
    """Ensure our opentracer spans features correspond to datadog span features.
    """
    def test_set_tag(self, nop_span):
        nop_span.set_tag('test', 2)
        assert nop_span._get_metric('test') == 2

    def test_tag_resource_name(self, nop_span):
        nop_span.set_tag('resource.name', 'myresource')
        assert nop_span._dd_span.resource == 'myresource'

    def test_tag_span_type(self, nop_span):
        nop_span.set_tag('span.type', 'db')
        assert nop_span._dd_span.span_type == 'db'

    def test_tag_service_name(self, nop_span):
        nop_span.set_tag('service.name', 'mysvc234')
        assert nop_span._dd_span.service == 'mysvc234'

    def test_tag_db_statement(self, nop_span):
        nop_span.set_tag('db.statement', 'SELECT * FROM USERS')
        assert nop_span._dd_span.resource == 'SELECT * FROM USERS'

    def test_tag_peer_hostname(self, nop_span):
        nop_span.set_tag('peer.hostname', 'peername')
        assert nop_span._dd_span.get_tag('out.host') == 'peername'

    def test_tag_peer_port(self, nop_span):
        nop_span.set_tag('peer.port', 55555)
        assert nop_span._get_metric('out.port') == 55555

    def test_tag_sampling_priority(self, nop_span):
        nop_span.set_tag('sampling.priority', '2')
        assert nop_span._dd_span.context._sampling_priority == '2'
