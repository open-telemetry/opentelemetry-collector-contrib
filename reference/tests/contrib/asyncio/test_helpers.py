import asyncio
import pytest

from ddtrace.context import Context
from ddtrace.internal.context_manager import CONTEXTVARS_IS_AVAILABLE
from ddtrace.contrib.asyncio import helpers
from .utils import AsyncioTestCase, mark_asyncio


@pytest.mark.skipif(
    CONTEXTVARS_IS_AVAILABLE,
    reason='only applicable to legacy asyncio integration'
)
class TestAsyncioHelpers(AsyncioTestCase):
    """
    Ensure that helpers set the ``Context`` properly when creating
    new ``Task`` or threads.
    """
    @mark_asyncio
    def test_set_call_context(self):
        # a different Context is set for the current logical execution
        task = asyncio.Task.current_task()
        ctx = Context()
        helpers.set_call_context(task, ctx)
        assert ctx == self.tracer.get_call_context()

    @mark_asyncio
    def test_ensure_future(self):
        # the wrapper should create a new Future that has the Context attached
        @asyncio.coroutine
        def future_work():
            # the ctx is available in this task
            ctx = self.tracer.get_call_context()
            assert 1 == len(ctx._trace)
            assert 'coroutine' == ctx._trace[0].name
            return ctx._trace[0].name

        self.tracer.trace('coroutine')
        # schedule future work and wait for a result
        delayed_task = helpers.ensure_future(future_work(), tracer=self.tracer)
        result = yield from asyncio.wait_for(delayed_task, timeout=1)
        assert 'coroutine' == result

    @mark_asyncio
    def test_run_in_executor_proxy(self):
        # the wrapper should pass arguments and results properly
        def future_work(number, name):
            assert 42 == number
            assert 'john' == name
            return True

        future = helpers.run_in_executor(self.loop, None, future_work, 42, 'john', tracer=self.tracer)
        result = yield from future
        assert result

    @mark_asyncio
    def test_run_in_executor_traces(self):
        # the wrapper should create a different Context when the Thread
        # is started; the new Context creates a new trace
        def future_work():
            # the Context is empty but the reference to the latest
            # span is here to keep the parenting
            ctx = self.tracer.get_call_context()
            assert 0 == len(ctx._trace)
            assert 'coroutine' == ctx._current_span.name
            return True

        span = self.tracer.trace('coroutine')
        future = helpers.run_in_executor(self.loop, None, future_work, tracer=self.tracer)
        # we close the Context
        span.finish()
        result = yield from future
        assert result

    @mark_asyncio
    def test_create_task(self):
        # the helper should create a new Task that has the Context attached
        @asyncio.coroutine
        def future_work():
            # the ctx is available in this task
            ctx = self.tracer.get_call_context()
            assert 0 == len(ctx._trace)
            child_span = self.tracer.trace('child_task')
            return child_span

        root_span = self.tracer.trace('main_task')
        # schedule future work and wait for a result
        task = helpers.create_task(future_work())
        result = yield from task
        assert root_span.trace_id == result.trace_id
        assert root_span.span_id == result.parent_id
