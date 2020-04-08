from ddtrace.constants import SAMPLING_PRIORITY_KEY, ORIGIN_KEY

from .utils import PyramidTestCase, PyramidBase


def includeme(config):
    pass


class TestPyramid(PyramidTestCase):
    instrument = True

    def test_tween_overridden(self):
        # in case our tween is overriden by the user config we should
        # not log rendering
        self.override_settings({'pyramid.tweens': 'pyramid.tweens.excview_tween_factory'})
        self.app.get('/json', status=200)
        spans = self.tracer.writer.pop()
        assert len(spans) == 0


class TestPyramidDistributedTracingDefault(PyramidBase):
    instrument = True

    def get_settings(self):
        return {}

    def test_distributed_tracing(self):
        # ensure the Context is properly created
        # if distributed tracing is enabled
        headers = {
            'x-datadog-trace-id': '100',
            'x-datadog-parent-id': '42',
            'x-datadog-sampling-priority': '2',
            'x-datadog-origin': 'synthetics',
        }
        self.app.get('/', headers=headers, status=200)
        writer = self.tracer.writer
        spans = writer.pop()
        assert len(spans) == 1
        # check the propagated Context
        span = spans[0]
        assert span.trace_id == 100
        assert span.parent_id == 42
        assert span.get_metric(SAMPLING_PRIORITY_KEY) == 2
        assert span.get_tag(ORIGIN_KEY) == 'synthetics'


class TestPyramidDistributedTracingDisabled(PyramidBase):
    instrument = True

    def get_settings(self):
        return {
            'datadog_distributed_tracing': False,
        }

    def test_distributed_tracing_disabled(self):
        # we do not inherit context if distributed tracing is disabled
        headers = {
            'x-datadog-trace-id': '100',
            'x-datadog-parent-id': '42',
            'x-datadog-sampling-priority': '2',
            'x-datadog-origin': 'synthetics',
        }
        self.app.get('/', headers=headers, status=200)
        writer = self.tracer.writer
        spans = writer.pop()
        assert len(spans) == 1
        # check the propagated Context
        span = spans[0]
        assert span.trace_id != 100
        assert span.parent_id != 42
        assert span.get_metric(SAMPLING_PRIORITY_KEY) != 2
        assert span.get_tag(ORIGIN_KEY) != 'synthetics'
