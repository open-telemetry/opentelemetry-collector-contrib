import asyncio
import pytest
from opentracing.scope_managers.asyncio import AsyncioScopeManager

import ddtrace
from ddtrace.opentracer.utils import get_context_provider_for_scope_manager

from tests.contrib.asyncio.utils import AsyncioTestCase, mark_asyncio
from .conftest import ot_tracer_factory  # noqa: F401


@pytest.fixture()
def ot_tracer(request, ot_tracer_factory):  # noqa: F811
    # use the dummy asyncio ot tracer
    request.instance.ot_tracer = ot_tracer_factory(
        'asyncio_svc',
        config={},
        scope_manager=AsyncioScopeManager(),
        context_provider=ddtrace.contrib.asyncio.context_provider,
    )
    request.instance.ot_writer = request.instance.ot_tracer._dd_tracer.writer
    request.instance.dd_tracer = request.instance.ot_tracer._dd_tracer


@pytest.mark.usefixtures('ot_tracer')
class TestTracerAsyncio(AsyncioTestCase):

    def reset(self):
        self.ot_writer.pop_traces()

    @mark_asyncio
    def test_trace_coroutine(self):
        # it should use the task context when invoked in a coroutine
        with self.ot_tracer.start_span('coroutine'):
            pass

        traces = self.ot_writer.pop_traces()

        assert len(traces) == 1
        assert len(traces[0]) == 1
        assert traces[0][0].name == 'coroutine'

    @mark_asyncio
    def test_trace_multiple_coroutines(self):
        # if multiple coroutines have nested tracing, they must belong
        # to the same trace
        @asyncio.coroutine
        def coro():
            # another traced coroutine
            with self.ot_tracer.start_active_span('coroutine_2'):
                return 42

        with self.ot_tracer.start_active_span('coroutine_1'):
            value = yield from coro()

        # the coroutine has been called correctly
        assert value == 42
        # a single trace has been properly reported
        traces = self.ot_writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 2
        assert traces[0][0].name == 'coroutine_1'
        assert traces[0][1].name == 'coroutine_2'
        # the parenting is correct
        assert traces[0][0] == traces[0][1]._parent
        assert traces[0][0].trace_id == traces[0][1].trace_id

    @mark_asyncio
    def test_exception(self):
        @asyncio.coroutine
        def f1():
            with self.ot_tracer.start_span('f1'):
                raise Exception('f1 error')

        with pytest.raises(Exception):
            yield from f1()

        traces = self.ot_writer.pop_traces()
        assert len(traces) == 1
        spans = traces[0]
        assert len(spans) == 1
        span = spans[0]
        assert span.error == 1
        assert span.get_tag('error.msg') == 'f1 error'
        assert 'Exception: f1 error' in span.get_tag('error.stack')

    @mark_asyncio
    def test_trace_multiple_calls(self):
        # create multiple futures so that we expect multiple
        # traces instead of a single one (helper not used)
        @asyncio.coroutine
        def coro():
            # another traced coroutine
            with self.ot_tracer.start_span('coroutine'):
                yield from asyncio.sleep(0.01)

        futures = [asyncio.ensure_future(coro()) for x in range(10)]
        for future in futures:
            yield from future

        traces = self.ot_writer.pop_traces()

        assert len(traces) == 10
        assert len(traces[0]) == 1
        assert traces[0][0].name == 'coroutine'


@pytest.mark.usefixtures('ot_tracer')
class TestTracerAsyncioCompatibility(AsyncioTestCase):
    """Ensure the opentracer works in tandem with the ddtracer and asyncio."""

    @mark_asyncio
    def test_trace_multiple_coroutines_ot_dd(self):
        """
        Ensure we can trace from opentracer to ddtracer across asyncio
        context switches.
        """
        # if multiple coroutines have nested tracing, they must belong
        # to the same trace
        @asyncio.coroutine
        def coro():
            # another traced coroutine
            with self.dd_tracer.trace('coroutine_2'):
                return 42

        with self.ot_tracer.start_active_span('coroutine_1'):
            value = yield from coro()

        # the coroutine has been called correctly
        assert value == 42
        # a single trace has been properly reported
        traces = self.ot_tracer._dd_tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 2
        assert traces[0][0].name == 'coroutine_1'
        assert traces[0][1].name == 'coroutine_2'
        # the parenting is correct
        assert traces[0][0] == traces[0][1]._parent
        assert traces[0][0].trace_id == traces[0][1].trace_id

    @mark_asyncio
    def test_trace_multiple_coroutines_dd_ot(self):
        """
        Ensure we can trace from ddtracer to opentracer across asyncio
        context switches.
        """
        # if multiple coroutines have nested tracing, they must belong
        # to the same trace
        @asyncio.coroutine
        def coro():
            # another traced coroutine
            with self.ot_tracer.start_span('coroutine_2'):
                return 42

        with self.dd_tracer.trace('coroutine_1'):
            value = yield from coro()

        # the coroutine has been called correctly
        assert value == 42
        # a single trace has been properly reported
        traces = self.ot_tracer._dd_tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 2
        assert traces[0][0].name == 'coroutine_1'
        assert traces[0][1].name == 'coroutine_2'
        # the parenting is correct
        assert traces[0][0] == traces[0][1]._parent
        assert traces[0][0].trace_id == traces[0][1].trace_id


@pytest.mark.skipif(
    ddtrace.internal.context_manager.CONTEXTVARS_IS_AVAILABLE,
    reason='only applicable to legacy asyncio provider'
)
class TestUtilsAsyncio(object):
    """Test the util routines of the opentracer with asyncio specific
    configuration.
    """

    def test_get_context_provider_for_scope_manager_asyncio(self):
        scope_manager = AsyncioScopeManager()
        ctx_prov = get_context_provider_for_scope_manager(scope_manager)
        assert isinstance(
            ctx_prov, ddtrace.contrib.asyncio.provider.AsyncioContextProvider
        )

    def test_tracer_context_provider_config(self):
        tracer = ddtrace.opentracer.Tracer('mysvc', scope_manager=AsyncioScopeManager())
        assert isinstance(
            tracer._dd_tracer.context_provider,
            ddtrace.contrib.asyncio.provider.AsyncioContextProvider,
        )
