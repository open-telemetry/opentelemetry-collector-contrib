import contextlib
import logging
import mock
import threading

from .base import BaseTestCase
from tests.test_tracer import get_dummy_tracer

import pytest

from ddtrace.span import Span
from ddtrace.context import Context
from ddtrace.constants import HOSTNAME_KEY
from ddtrace.ext.priority import USER_REJECT, AUTO_REJECT, AUTO_KEEP, USER_KEEP


@pytest.fixture
def tracer_with_debug_logging():
    # All the tracers, dummy or not, shares the same logging object.
    tracer = get_dummy_tracer()
    level = tracer.log.level
    tracer.log.setLevel(logging.DEBUG)
    try:
        yield tracer
    finally:
        tracer.log.setLevel(level)


@mock.patch('logging.Logger.debug')
def test_log_unfinished_spans(log, tracer_with_debug_logging):
    # when the root parent is finished, notify if there are spans still pending
    tracer = tracer_with_debug_logging
    ctx = Context()
    # manually create a root-child trace
    root = Span(tracer=tracer, name='root')
    child_1 = Span(tracer=tracer, name='child_1', trace_id=root.trace_id, parent_id=root.span_id)
    child_2 = Span(tracer=tracer, name='child_2', trace_id=root.trace_id, parent_id=root.span_id)
    child_1._parent = root
    child_2._parent = root
    ctx.add_span(root)
    ctx.add_span(child_1)
    ctx.add_span(child_2)
    # close only the parent
    root.finish()
    unfinished_spans_log = log.call_args_list[-3][0][2]
    child_1_log = log.call_args_list[-2][0][1]
    child_2_log = log.call_args_list[-1][0][1]
    assert 2 == unfinished_spans_log
    assert 'name child_1' in child_1_log
    assert 'name child_2' in child_2_log
    assert 'duration 0.000000s' in child_1_log
    assert 'duration 0.000000s' in child_2_log


class TestTracingContext(BaseTestCase):
    """
    Tests related to the ``Context`` class that hosts the trace for the
    current execution flow.
    """
    @contextlib.contextmanager
    def override_partial_flush(self, ctx, enabled, min_spans):
        original_enabled = ctx._partial_flush_enabled
        original_min_spans = ctx._partial_flush_min_spans

        ctx._partial_flush_enabled = enabled
        ctx._partial_flush_min_spans = min_spans

        try:
            yield
        finally:
            ctx._partial_flush_enabled = original_enabled
            ctx._partial_flush_min_spans = original_min_spans

    def test_add_span(self):
        # it should add multiple spans
        ctx = Context()
        span = Span(tracer=None, name='fake_span')
        ctx.add_span(span)
        assert 1 == len(ctx._trace)
        assert 'fake_span' == ctx._trace[0].name
        assert ctx == span.context

    def test_context_sampled(self):
        # a context is sampled if the spans are sampled
        ctx = Context()
        span = Span(tracer=None, name='fake_span')
        ctx.add_span(span)
        span.finish()
        trace, sampled = ctx.get()
        assert sampled is True
        assert ctx.sampling_priority is None

    def test_context_priority(self):
        # a context is sampled if the spans are sampled
        ctx = Context()
        for priority in [USER_REJECT, AUTO_REJECT, AUTO_KEEP, USER_KEEP, None, 999]:
            ctx.sampling_priority = priority
            span = Span(tracer=None, name=('fake_span_%s' % repr(priority)))
            ctx.add_span(span)
            span.finish()
            # It's "normal" to have sampled be true even when priority sampling is
            # set to 0 or -1. It would stay false even even with priority set to 2.
            # The only criteria to send (or not) the spans to the agent should be
            # this "sampled" attribute, as it's tightly related to the trace weight.
            assert priority == ctx.sampling_priority
            trace, sampled = ctx.get()
            assert sampled is True, 'priority has no impact on sampled status'

    def test_current_span(self):
        # it should return the current active span
        ctx = Context()
        span = Span(tracer=None, name='fake_span')
        ctx.add_span(span)
        assert span == ctx.get_current_span()

    def test_current_root_span_none(self):
        # it should return none when there is no root span
        ctx = Context()
        assert ctx.get_current_root_span() is None

    def test_current_root_span(self):
        # it should return the current active root span
        ctx = Context()
        span = Span(tracer=None, name='fake_span')
        ctx.add_span(span)
        assert span == ctx.get_current_root_span()

    def test_close_span(self):
        # it should keep track of closed spans, moving
        # the current active to it's parent
        ctx = Context()
        span = Span(tracer=None, name='fake_span')
        ctx.add_span(span)
        ctx.close_span(span)
        assert ctx.get_current_span() is None

    def test_get_trace(self):
        # it should return the internal trace structure
        # if the context is finished
        ctx = Context()
        span = Span(tracer=None, name='fake_span')
        ctx.add_span(span)
        span.finish()
        trace, sampled = ctx.get()
        assert [span] == trace
        assert sampled is True
        # the context should be empty
        assert 0 == len(ctx._trace)
        assert ctx._current_span is None

    def test_get_trace_empty(self):
        # it should return None if the Context is not finished
        ctx = Context()
        span = Span(tracer=None, name='fake_span')
        ctx.add_span(span)
        trace, sampled = ctx.get()
        assert trace is None
        assert sampled is None

    @mock.patch('ddtrace.internal.hostname.get_hostname')
    def test_get_report_hostname_enabled(self, get_hostname):
        get_hostname.return_value = 'test-hostname'

        with self.override_global_config(dict(report_hostname=True)):
            # Create a context and add a span and finish it
            ctx = Context()
            span = Span(tracer=None, name='fake_span')
            ctx.add_span(span)
            span.finish()

            # Assert that we have not added the tag to the span yet
            assert span.get_tag(HOSTNAME_KEY) is None

            # Assert that retrieving the trace sets the tag
            trace, _ = ctx.get()
            assert trace[0].get_tag(HOSTNAME_KEY) == 'test-hostname'
            assert span.get_tag(HOSTNAME_KEY) == 'test-hostname'

    @mock.patch('ddtrace.internal.hostname.get_hostname')
    def test_get_report_hostname_disabled(self, get_hostname):
        get_hostname.return_value = 'test-hostname'

        with self.override_global_config(dict(report_hostname=False)):
            # Create a context and add a span and finish it
            ctx = Context()
            span = Span(tracer=None, name='fake_span')
            ctx.add_span(span)
            span.finish()

            # Assert that we have not added the tag to the span yet
            assert span.get_tag(HOSTNAME_KEY) is None

            # Assert that retrieving the trace does not set the tag
            trace, _ = ctx.get()
            assert trace[0].get_tag(HOSTNAME_KEY) is None
            assert span.get_tag(HOSTNAME_KEY) is None

    @mock.patch('ddtrace.internal.hostname.get_hostname')
    def test_get_report_hostname_default(self, get_hostname):
        get_hostname.return_value = 'test-hostname'

        # Create a context and add a span and finish it
        ctx = Context()
        span = Span(tracer=None, name='fake_span')
        ctx.add_span(span)
        span.finish()

        # Assert that we have not added the tag to the span yet
        assert span.get_tag(HOSTNAME_KEY) is None

        # Assert that retrieving the trace does not set the tag
        trace, _ = ctx.get()
        assert trace[0].get_tag(HOSTNAME_KEY) is None
        assert span.get_tag(HOSTNAME_KEY) is None

    def test_partial_flush(self):
        """
        When calling `Context.get`
        When partial flushing is enabled
        When we have just enough finished spans to flush
        We return the finished spans
        """
        tracer = get_dummy_tracer()
        ctx = Context()

        # Create a root span with 5 children, all of the children are finished, the root is not
        root = Span(tracer=tracer, name='root')
        ctx.add_span(root)
        for i in range(5):
            child = Span(tracer=tracer, name='child_{}'.format(i), trace_id=root.trace_id, parent_id=root.span_id)
            child._parent = root
            child.finished = True
            ctx.add_span(child)
            ctx.close_span(child)

        with self.override_partial_flush(ctx, enabled=True, min_spans=5):
            trace, sampled = ctx.get()

        self.assertIsNotNone(trace)
        self.assertIsNotNone(sampled)

        self.assertEqual(len(trace), 5)
        self.assertEqual(
            set(['child_0', 'child_1', 'child_2', 'child_3', 'child_4']),
            set([span.name for span in trace])
        )

        # Ensure we clear/reset internal stats as expected
        self.assertEqual(ctx._trace, [root])
        with self.override_partial_flush(ctx, enabled=True, min_spans=5):
            trace, sampled = ctx.get()
            self.assertIsNone(trace)
            self.assertIsNone(sampled)

    def test_partial_flush_too_many(self):
        """
        When calling `Context.get`
        When partial flushing is enabled
        When we have more than the minimum number of spans needed to flush
        We return the finished spans
        """
        tracer = get_dummy_tracer()
        ctx = Context()

        # Create a root span with 5 children, all of the children are finished, the root is not
        root = Span(tracer=tracer, name='root')
        ctx.add_span(root)
        for i in range(5):
            child = Span(tracer=tracer, name='child_{}'.format(i), trace_id=root.trace_id, parent_id=root.span_id)
            child._parent = root
            child.finished = True
            ctx.add_span(child)
            ctx.close_span(child)

        with self.override_partial_flush(ctx, enabled=True, min_spans=1):
            trace, sampled = ctx.get()

        self.assertIsNotNone(trace)
        self.assertIsNotNone(sampled)

        self.assertEqual(len(trace), 5)
        self.assertEqual(
            set(['child_0', 'child_1', 'child_2', 'child_3', 'child_4']),
            set([span.name for span in trace])
        )

        # Ensure we clear/reset internal stats as expected
        self.assertEqual(ctx._trace, [root])
        with self.override_partial_flush(ctx, enabled=True, min_spans=5):
            trace, sampled = ctx.get()
            self.assertIsNone(trace)
            self.assertIsNone(sampled)

    def test_partial_flush_too_few(self):
        """
        When calling `Context.get`
        When partial flushing is enabled
        When we do not have enough finished spans to flush
        We return no spans
        """
        tracer = get_dummy_tracer()
        ctx = Context()

        # Create a root span with 5 children, all of the children are finished, the root is not
        root = Span(tracer=tracer, name='root')
        ctx.add_span(root)
        for i in range(5):
            child = Span(tracer=tracer, name='child_{}'.format(i), trace_id=root.trace_id, parent_id=root.span_id)
            child._parent = root
            child.finished = True
            ctx.add_span(child)
            ctx.close_span(child)

        # Test with having 1 too few spans for partial flush
        with self.override_partial_flush(ctx, enabled=True, min_spans=6):
            trace, sampled = ctx.get()

        self.assertIsNone(trace)
        self.assertIsNone(sampled)

        self.assertEqual(len(ctx._trace), 6)
        self.assertEqual(
            set(['root', 'child_0', 'child_1', 'child_2', 'child_3', 'child_4']),
            set([span.name for span in ctx._trace])
        )

    def test_partial_flush_remaining(self):
        """
        When calling `Context.get`
        When partial flushing is enabled
        When we have some unfinished spans
        We keep the unfinished spans around
        """
        tracer = get_dummy_tracer()
        ctx = Context()

        # Create a root span with 5 children, all of the children are finished, the root is not
        root = Span(tracer=tracer, name='root')
        ctx.add_span(root)
        for i in range(10):
            child = Span(tracer=tracer, name='child_{}'.format(i), trace_id=root.trace_id, parent_id=root.span_id)
            child._parent = root
            ctx.add_span(child)

            # CLose the first 5 only
            if i < 5:
                child.finished = True
                ctx.close_span(child)

        with self.override_partial_flush(ctx, enabled=True, min_spans=5):
            trace, sampled = ctx.get()

        # Assert partially flushed spans
        self.assertTrue(len(trace), 5)
        self.assertIsNotNone(sampled)
        self.assertEqual(
            set(['child_0', 'child_1', 'child_2', 'child_3', 'child_4']),
            set([span.name for span in trace])
        )

        # Assert remaining unclosed spans
        self.assertEqual(len(ctx._trace), 6)
        self.assertEqual(
            set(['root', 'child_5', 'child_6', 'child_7', 'child_8', 'child_9']),
            set([span.name for span in ctx._trace]),
        )

    def test_finished(self):
        # a Context is finished if all spans inside are finished
        ctx = Context()
        span = Span(tracer=None, name='fake_span')
        ctx.add_span(span)
        ctx.close_span(span)

    @mock.patch('logging.Logger.debug')
    def test_log_unfinished_spans_disabled(self, log):
        # the trace finished status logging is disabled
        tracer = get_dummy_tracer()
        ctx = Context()
        # manually create a root-child trace
        root = Span(tracer=tracer, name='root')
        child_1 = Span(tracer=tracer, name='child_1', trace_id=root.trace_id, parent_id=root.span_id)
        child_2 = Span(tracer=tracer, name='child_2', trace_id=root.trace_id, parent_id=root.span_id)
        child_1._parent = root
        child_2._parent = root
        ctx.add_span(root)
        ctx.add_span(child_1)
        ctx.add_span(child_2)
        # close only the parent
        root.finish()
        # the logger has never been invoked to print unfinished spans
        for call, _ in log.call_args_list:
            msg = call[0]
            assert 'the trace has %d unfinished spans' not in msg

    @mock.patch('logging.Logger.debug')
    def test_log_unfinished_spans_when_ok(self, log):
        # if the unfinished spans logging is enabled but the trace is finished, don't log anything
        tracer = get_dummy_tracer()
        ctx = Context()
        # manually create a root-child trace
        root = Span(tracer=tracer, name='root')
        child = Span(tracer=tracer, name='child_1', trace_id=root.trace_id, parent_id=root.span_id)
        child._parent = root
        ctx.add_span(root)
        ctx.add_span(child)
        # close the trace
        child.finish()
        root.finish()
        # the logger has never been invoked to print unfinished spans
        for call, _ in log.call_args_list:
            msg = call[0]
            assert 'the trace has %d unfinished spans' not in msg

    def test_thread_safe(self):
        # the Context must be thread-safe
        ctx = Context()

        def _fill_ctx():
            span = Span(tracer=None, name='fake_span')
            ctx.add_span(span)

        threads = [threading.Thread(target=_fill_ctx) for _ in range(100)]

        for t in threads:
            t.daemon = True
            t.start()

        for t in threads:
            t.join()

        assert 100 == len(ctx._trace)

    def test_clone(self):
        ctx = Context()
        ctx.sampling_priority = 2
        # manually create a root-child trace
        root = Span(tracer=None, name='root')
        child = Span(tracer=None, name='child_1', trace_id=root.trace_id, parent_id=root.span_id)
        child._parent = root
        ctx.add_span(root)
        ctx.add_span(child)
        cloned_ctx = ctx.clone()
        assert cloned_ctx._parent_trace_id == ctx._parent_trace_id
        assert cloned_ctx._parent_span_id == ctx._parent_span_id
        assert cloned_ctx._sampling_priority == ctx._sampling_priority
        assert cloned_ctx._dd_origin == ctx._dd_origin
        assert cloned_ctx._current_span == ctx._current_span
        assert cloned_ctx._trace == []
