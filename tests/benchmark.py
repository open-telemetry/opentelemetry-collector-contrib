from ddtrace import Tracer
import pytest

from .test_tracer import DummyWriter


@pytest.fixture
def tracer():
    tracer = Tracer()
    tracer.writer = DummyWriter()
    return tracer


def test_tracer_context(benchmark, tracer):
    def func(tracer):
        with tracer.trace('a', service='s', resource='r', span_type='t'):
            pass

    benchmark(func, tracer)


def test_tracer_wrap_staticmethod(benchmark, tracer):
    class Foo(object):
        @staticmethod
        @tracer.wrap()
        def func():
            return 0

    f = Foo()
    benchmark(f.func)


def test_tracer_wrap_classmethod(benchmark, tracer):
    class Foo(object):
        @classmethod
        @tracer.wrap()
        def func(cls):
            return 0

    f = Foo()
    benchmark(f.func)


def test_tracer_wrap_instancemethod(benchmark, tracer):
    class Foo(object):
        @tracer.wrap()
        def func(self):
            return 0

    f = Foo()
    benchmark(f.func)


def test_tracer_start_span(benchmark, tracer):
    benchmark(tracer.start_span, 'benchmark')


def test_tracer_start_finish_span(benchmark, tracer):
    def func(tracer):
        s = tracer.start_span('benchmark')
        s.finish()

    benchmark(func, tracer)


def test_trace_simple_trace(benchmark, tracer):
    def func(tracer):
        with tracer.trace('parent'):
            for i in range(5):
                with tracer.trace('child') as c:
                    c.set_tag('i', i)

    benchmark(func, tracer)


def test_tracer_large_trace(benchmark, tracer):
    import random

    # generate trace with 1024 spans
    @tracer.wrap()
    def func(tracer, level=0):
        span = tracer.current_span()

        # do some work
        num = random.randint(1, 10)
        span.set_tag('num', num)

        if level < 10:
            func(tracer, level + 1)
            func(tracer, level + 1)

    benchmark(func, tracer)
