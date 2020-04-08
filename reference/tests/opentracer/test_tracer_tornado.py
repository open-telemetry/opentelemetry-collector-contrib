import pytest
from opentracing.scope_managers.tornado import TornadoScopeManager


@pytest.fixture()
def ot_tracer(ot_tracer_factory):
    """Fixture providing an opentracer configured for tornado usage."""
    yield ot_tracer_factory('tornado_svc', {}, TornadoScopeManager())


class TestTracerTornado(object):
    """
    Since the ScopeManager is provided by OpenTracing we should simply test
    whether it exists and works for a very simple use-case.
    """

    def test_sanity(self, ot_tracer, writer):
        with ot_tracer.start_active_span('one'):
            with ot_tracer.start_active_span('two'):
                pass

        traces = writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 2
        assert traces[0][0].name == 'one'
        assert traces[0][1].name == 'two'

        # the parenting is correct
        assert traces[0][0] == traces[0][1]._parent
        assert traces[0][0].trace_id == traces[0][1].trace_id
