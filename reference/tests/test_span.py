import mock
import time

from unittest.case import SkipTest

from ddtrace.context import Context
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.span import Span
from ddtrace.ext import SpanTypes, errors, priority
from .base import BaseTracerTestCase


class SpanTestCase(BaseTracerTestCase):
    def test_ids(self):
        s = Span(tracer=None, name='span.test')
        assert s.trace_id
        assert s.span_id
        assert not s.parent_id

        s2 = Span(tracer=None, name='t', trace_id=1, span_id=2, parent_id=1)
        assert s2.trace_id == 1
        assert s2.span_id == 2
        assert s2.parent_id == 1

    def test_tags(self):
        s = Span(tracer=None, name='test.span')
        s.set_tag('a', 'a')
        s.set_tag('b', 1)
        s.set_tag('c', '1')
        d = s.to_dict()
        assert d['meta'] == dict(a='a', c='1')
        assert d['metrics'] == dict(b=1)

    def test_numeric_tags(self):
        s = Span(tracer=None, name='test.span')
        s.set_tag('negative', -1)
        s.set_tag('zero', 0)
        s.set_tag('positive', 1)
        s.set_tag('large_int', 2**53)
        s.set_tag('really_large_int', (2**53) + 1)
        s.set_tag('large_negative_int', -(2**53))
        s.set_tag('really_large_negative_int', -((2**53) + 1))
        s.set_tag('float', 12.3456789)
        s.set_tag('negative_float', -12.3456789)
        s.set_tag('large_float', 2.0**53)
        s.set_tag('really_large_float', (2.0**53) + 1)

        d = s.to_dict()
        assert d['meta'] == dict(
            really_large_int=str(((2**53) + 1)),
            really_large_negative_int=str(-((2**53) + 1)),
        )
        assert d['metrics'] == {
            'negative': -1,
            'zero': 0,
            'positive': 1,
            'large_int': 2**53,
            'large_negative_int': -(2**53),
            'float': 12.3456789,
            'negative_float': -12.3456789,
            'large_float': 2.0**53,
            'really_large_float': (2.0**53) + 1,
        }

    def test_set_tag_bool(self):
        s = Span(tracer=None, name='test.span')
        s.set_tag('true', True)
        s.set_tag('false', False)

        d = s.to_dict()
        assert d['meta'] == dict(true='True', false='False')
        assert 'metrics' not in d

    def test_set_tag_metric(self):
        s = Span(tracer=None, name='test.span')

        s.set_tag('test', 'value')
        assert s.meta == dict(test='value')
        assert s.metrics == dict()

        s.set_tag('test', 1)
        assert s.meta == dict()
        assert s.metrics == dict(test=1)

    def test_set_valid_metrics(self):
        s = Span(tracer=None, name='test.span')
        s.set_metric('a', 0)
        s.set_metric('b', -12)
        s.set_metric('c', 12.134)
        s.set_metric('d', 1231543543265475686787869123)
        s.set_metric('e', '12.34')
        d = s.to_dict()
        expected = {
            'a': 0,
            'b': -12,
            'c': 12.134,
            'd': 1231543543265475686787869123,
            'e': 12.34,
        }
        assert d['metrics'] == expected

    def test_set_invalid_metric(self):
        s = Span(tracer=None, name='test.span')

        invalid_metrics = [
            None,
            {},
            [],
            s,
            'quarante-douze',
            float('nan'),
            float('inf'),
            1j
        ]

        for i, m in enumerate(invalid_metrics):
            k = str(i)
            s.set_metric(k, m)
            assert s.get_metric(k) is None

    def test_set_numpy_metric(self):
        try:
            import numpy as np
        except ImportError:
            raise SkipTest('numpy not installed')
        s = Span(tracer=None, name='test.span')
        s.set_metric('a', np.int64(1))
        assert s.get_metric('a') == 1
        assert type(s.get_metric('a')) == float

    def test_tags_not_string(self):
        # ensure we can cast as strings
        class Foo(object):
            def __repr__(self):
                1 / 0

        s = Span(tracer=None, name='test.span')
        s.set_tag('a', Foo())

    def test_finish(self):
        # ensure finish will record a span
        ctx = Context()
        s = Span(self.tracer, 'test.span', context=ctx)
        ctx.add_span(s)
        assert s.duration is None

        sleep = 0.05
        with s as s1:
            assert s is s1
            time.sleep(sleep)
        assert s.duration >= sleep, '%s < %s' % (s.duration, sleep)
        self.assert_span_count(1)

    def test_finish_no_tracer(self):
        # ensure finish works with no tracer without raising exceptions
        s = Span(tracer=None, name='test.span')
        s.finish()

    def test_finish_called_multiple_times(self):
        # we should only record a span the first time finish is called on it
        ctx = Context()
        s = Span(self.tracer, 'bar', context=ctx)
        ctx.add_span(s)
        s.finish()
        s.finish()
        self.assert_span_count(1)

    def test_finish_set_span_duration(self):
        # If set the duration on a span, the span should be recorded with this
        # duration
        s = Span(tracer=None, name='test.span')
        s.duration = 1337.0
        s.finish()
        assert s.duration == 1337.0

    def test_traceback_with_error(self):
        s = Span(None, 'test.span')
        try:
            1 / 0
        except ZeroDivisionError:
            s.set_traceback()
        else:
            assert 0, 'should have failed'

        assert s.error
        assert 'by zero' in s.get_tag(errors.ERROR_MSG)
        assert 'ZeroDivisionError' in s.get_tag(errors.ERROR_TYPE)

    def test_traceback_without_error(self):
        s = Span(None, 'test.span')
        s.set_traceback()
        assert not s.error
        assert not s.get_tag(errors.ERROR_MSG)
        assert not s.get_tag(errors.ERROR_TYPE)
        assert 'in test_traceback_without_error' in s.get_tag(errors.ERROR_STACK)

    def test_ctx_mgr(self):
        s = Span(self.tracer, 'bar')
        assert not s.duration
        assert not s.error

        e = Exception('boo')
        try:
            with s:
                time.sleep(0.01)
                raise e
        except Exception as out:
            assert out == e
            assert s.duration > 0, s.duration
            assert s.error
            assert s.get_tag(errors.ERROR_MSG) == 'boo'
            assert 'Exception' in s.get_tag(errors.ERROR_TYPE)
            assert s.get_tag(errors.ERROR_STACK)

        else:
            assert 0, 'should have failed'

    def test_span_type(self):
        s = Span(tracer=None, name='test.span', service='s', resource='r', span_type=SpanTypes.WEB)
        s.set_tag('a', '1')
        s.set_meta('b', '2')
        s.finish()

        d = s.to_dict()
        assert d
        assert d['span_id'] == s.span_id
        assert d['trace_id'] == s.trace_id
        assert d['parent_id'] == s.parent_id
        assert d['meta'] == {'a': '1', 'b': '2'}
        assert d['type'] == 'web'
        assert d['error'] == 0
        assert type(d['error']) == int

    def test_span_to_dict(self):
        s = Span(tracer=None, name='test.span', service='s', resource='r')
        s.span_type = 'foo'
        s.set_tag('a', '1')
        s.set_meta('b', '2')
        s.finish()

        d = s.to_dict()
        assert d
        assert d['span_id'] == s.span_id
        assert d['trace_id'] == s.trace_id
        assert d['parent_id'] == s.parent_id
        assert d['meta'] == {'a': '1', 'b': '2'}
        assert d['type'] == 'foo'
        assert d['error'] == 0
        assert type(d['error']) == int

    def test_span_to_dict_sub(self):
        parent = Span(tracer=None, name='test.span', service='s', resource='r')
        s = Span(tracer=None, name='test.span', service='s', resource='r')
        s._parent = parent
        s.span_type = 'foo'
        s.set_tag('a', '1')
        s.set_meta('b', '2')
        s.finish()

        d = s.to_dict()
        assert d
        assert d['span_id'] == s.span_id
        assert d['trace_id'] == s.trace_id
        assert d['parent_id'] == s.parent_id
        assert d['meta'] == {'a': '1', 'b': '2'}
        assert d['type'] == 'foo'
        assert d['error'] == 0
        assert type(d['error']) == int

    def test_span_boolean_err(self):
        s = Span(tracer=None, name='foo.bar', service='s', resource='r')
        s.error = True
        s.finish()

        d = s.to_dict()
        assert d
        assert d['error'] == 1
        assert type(d['error']) == int

    @mock.patch('ddtrace.span.log')
    def test_numeric_tags_none(self, span_log):
        s = Span(tracer=None, name='test.span')
        s.set_tag(ANALYTICS_SAMPLE_RATE_KEY, None)
        d = s.to_dict()
        assert d
        assert 'metrics' not in d

        # Ensure we log a debug message
        span_log.debug.assert_called_once_with(
            'ignoring not number metric %s:%s',
            ANALYTICS_SAMPLE_RATE_KEY,
            None,
        )

    def test_numeric_tags_true(self):
        s = Span(tracer=None, name='test.span')
        s.set_tag(ANALYTICS_SAMPLE_RATE_KEY, True)
        d = s.to_dict()
        assert d
        expected = {
            ANALYTICS_SAMPLE_RATE_KEY: 1.0
        }
        assert d['metrics'] == expected

    def test_numeric_tags_value(self):
        s = Span(tracer=None, name='test.span')
        s.set_tag(ANALYTICS_SAMPLE_RATE_KEY, 0.5)
        d = s.to_dict()
        assert d
        expected = {
            ANALYTICS_SAMPLE_RATE_KEY: 0.5
        }
        assert d['metrics'] == expected

    def test_numeric_tags_bad_value(self):
        s = Span(tracer=None, name='test.span')
        s.set_tag(ANALYTICS_SAMPLE_RATE_KEY, 'Hello')
        d = s.to_dict()
        assert d
        assert 'metrics' not in d

    def test_set_tag_manual_keep(self):
        ctx = Context()
        s = Span(tracer=None, name='root.span', service='s', resource='r', context=ctx)

        assert s.context == ctx
        assert ctx.sampling_priority != priority.USER_KEEP
        assert s.context.sampling_priority != priority.USER_KEEP
        assert s.meta == dict()

        s.set_tag('manual.keep')
        assert ctx.sampling_priority == priority.USER_KEEP
        assert s.context.sampling_priority == priority.USER_KEEP
        assert s.meta == dict()

        ctx.sampling_priority = priority.AUTO_REJECT
        assert ctx.sampling_priority == priority.AUTO_REJECT
        assert s.context.sampling_priority == priority.AUTO_REJECT
        assert s.meta == dict()

        s.set_tag('manual.keep')
        assert ctx.sampling_priority == priority.USER_KEEP
        assert s.context.sampling_priority == priority.USER_KEEP
        assert s.meta == dict()

    def test_set_tag_manual_drop(self):
        ctx = Context()
        s = Span(tracer=None, name='root.span', service='s', resource='r', context=ctx)

        assert s.context == ctx
        assert ctx.sampling_priority != priority.USER_REJECT
        assert s.context.sampling_priority != priority.USER_REJECT
        assert s.meta == dict()

        s.set_tag('manual.drop')
        assert ctx.sampling_priority == priority.USER_REJECT
        assert s.context.sampling_priority == priority.USER_REJECT
        assert s.meta == dict()

        ctx.sampling_priority = priority.AUTO_REJECT
        assert ctx.sampling_priority == priority.AUTO_REJECT
        assert s.context.sampling_priority == priority.AUTO_REJECT
        assert s.meta == dict()

        s.set_tag('manual.drop')
        assert ctx.sampling_priority == priority.USER_REJECT
        assert s.context.sampling_priority == priority.USER_REJECT
        assert s.meta == dict()

    def test_set_tag_none(self):
        s = Span(tracer=None, name='root.span', service='s', resource='r')
        assert s.meta == dict()

        s.set_tag('custom.key', '100')

        assert s.meta == {'custom.key': '100'}

        s.set_tag('custom.key', None)

        assert s.meta == {'custom.key': 'None'}

    def test_duration_zero(self):
        s = Span(tracer=None, name='foo.bar', service='s', resource='r', start=123)
        s.finish(finish_time=123)
        assert s.duration_ns == 0
        assert s.duration == 0

    def test_start_int(self):
        s = Span(tracer=None, name='foo.bar', service='s', resource='r', start=123)
        assert s.start == 123
        assert s.start_ns == 123000000000

        s = Span(tracer=None, name='foo.bar', service='s', resource='r', start=123.123)
        assert s.start == 123.123
        assert s.start_ns == 123123000000

        s = Span(tracer=None, name='foo.bar', service='s', resource='r', start=123.123)
        s.start = 234567890.0
        assert s.start == 234567890
        assert s.start_ns == 234567890000000000

    def test_duration_int(self):
        s = Span(tracer=None, name='foo.bar', service='s', resource='r')
        s.finish()
        assert isinstance(s.duration_ns, int)
        assert isinstance(s.duration, float)

        s = Span(tracer=None, name='foo.bar', service='s', resource='r', start=123)
        s.finish(finish_time=123.2)
        assert s.duration_ns == 200000000
        assert s.duration == 0.2

        s = Span(tracer=None, name='foo.bar', service='s', resource='r', start=123.1)
        s.finish(finish_time=123.2)
        assert s.duration_ns == 100000000
        assert s.duration == 0.1

        s = Span(tracer=None, name='foo.bar', service='s', resource='r', start=122)
        s.finish(finish_time=123)
        assert s.duration_ns == 1000000000
        assert s.duration == 1
