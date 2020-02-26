import unittest

from ddtrace.contrib.tornado.compat import futures_available
from ddtrace.ext import http

from tornado import version_info

from .utils import TornadoTestCase
from ...utils import assert_span_http_status_code


class TestTornadoExecutor(TornadoTestCase):
    """
    Ensure that Tornado web handlers are properly traced even if
    ``@run_on_executor`` decorator is used.
    """
    def test_on_executor_handler(self):
        # it should trace a handler that uses @run_on_executor
        response = self.fetch('/executor_handler/')
        assert 200 == response.code

        traces = self.tracer.writer.pop_traces()
        assert 2 == len(traces)
        assert 1 == len(traces[0])
        assert 1 == len(traces[1])

        # this trace yields the execution of the thread
        request_span = traces[1][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.ExecutorHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 200)
        assert self.get_url('/executor_handler/') == request_span.get_tag(http.URL)
        assert 0 == request_span.error
        assert request_span.duration >= 0.05

        # this trace is executed in a different thread
        executor_span = traces[0][0]
        assert 'tornado-web' == executor_span.service
        assert 'tornado.executor.with' == executor_span.name
        assert executor_span.parent_id == request_span.span_id
        assert 0 == executor_span.error
        assert executor_span.duration >= 0.05

    @unittest.skipUnless(futures_available, 'Futures must be available to test direct submit')
    def test_on_executor_submit(self):
        # it should propagate the context when a handler uses directly the `executor.submit()`
        response = self.fetch('/executor_submit_handler/')
        assert 200 == response.code

        traces = self.tracer.writer.pop_traces()
        assert 2 == len(traces)
        assert 1 == len(traces[0])
        assert 1 == len(traces[1])

        # this trace yields the execution of the thread
        request_span = traces[1][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.ExecutorSubmitHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 200)
        assert self.get_url('/executor_submit_handler/') == request_span.get_tag(http.URL)
        assert 0 == request_span.error
        assert request_span.duration >= 0.05

        # this trace is executed in a different thread
        executor_span = traces[0][0]
        assert 'tornado-web' == executor_span.service
        assert 'tornado.executor.query' == executor_span.name
        assert executor_span.parent_id == request_span.span_id
        assert 0 == executor_span.error
        assert executor_span.duration >= 0.05

    def test_on_executor_exception_handler(self):
        # it should trace a handler that uses @run_on_executor
        response = self.fetch('/executor_exception/')
        assert 500 == response.code

        traces = self.tracer.writer.pop_traces()
        assert 2 == len(traces)
        assert 1 == len(traces[0])
        assert 1 == len(traces[1])

        # this trace yields the execution of the thread
        request_span = traces[1][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.ExecutorExceptionHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 500)
        assert self.get_url('/executor_exception/') == request_span.get_tag(http.URL)
        assert 1 == request_span.error
        assert 'Ouch!' == request_span.get_tag('error.msg')
        assert 'Exception: Ouch!' in request_span.get_tag('error.stack')

        # this trace is executed in a different thread
        executor_span = traces[0][0]
        assert 'tornado-web' == executor_span.service
        assert 'tornado.executor.with' == executor_span.name
        assert executor_span.parent_id == request_span.span_id
        assert 1 == executor_span.error
        assert 'Ouch!' == executor_span.get_tag('error.msg')
        assert 'Exception: Ouch!' in executor_span.get_tag('error.stack')

    @unittest.skipIf(
        (version_info[0], version_info[1]) in [(4, 0), (4, 1)],
        reason='Custom kwargs are available only for Tornado 4.2+',
    )
    def test_on_executor_custom_kwarg(self):
        # it should trace a handler that uses @run_on_executor
        # with the `executor` kwarg
        response = self.fetch('/executor_custom_handler/')
        assert 200 == response.code

        traces = self.tracer.writer.pop_traces()
        assert 2 == len(traces)
        assert 1 == len(traces[0])
        assert 1 == len(traces[1])

        # this trace yields the execution of the thread
        request_span = traces[1][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.ExecutorCustomHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 200)
        assert self.get_url('/executor_custom_handler/') == request_span.get_tag(http.URL)
        assert 0 == request_span.error
        assert request_span.duration >= 0.05

        # this trace is executed in a different thread
        executor_span = traces[0][0]
        assert 'tornado-web' == executor_span.service
        assert 'tornado.executor.with' == executor_span.name
        assert executor_span.parent_id == request_span.span_id
        assert 0 == executor_span.error
        assert executor_span.duration >= 0.05

    @unittest.skipIf(
        (version_info[0], version_info[1]) in [(4, 0), (4, 1)],
        reason='Custom kwargs are available only for Tornado 4.2+',
    )
    def test_on_executor_custom_args_kwarg(self):
        # it should raise an exception if the decorator is used improperly
        response = self.fetch('/executor_custom_args_handler/')
        assert 500 == response.code

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])

        # this trace yields the execution of the thread
        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.ExecutorCustomArgsHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 500)
        assert self.get_url('/executor_custom_args_handler/') == request_span.get_tag(http.URL)
        assert 1 == request_span.error
        assert 'cannot combine positional and keyword args' == request_span.get_tag('error.msg')
        assert 'ValueError' in request_span.get_tag('error.stack')

    @unittest.skipUnless(futures_available, 'Futures must be available to test direct submit')
    def test_futures_double_instrumentation(self):
        # it should not double wrap `ThreadpPoolExecutor.submit` method if
        # `futures` is already instrumented
        from ddtrace import patch
        patch(futures=True)
        from concurrent.futures import ThreadPoolExecutor
        from ddtrace.vendor.wrapt import BoundFunctionWrapper

        fn_wrapper = getattr(ThreadPoolExecutor.submit, '__wrapped__', None)
        assert not isinstance(fn_wrapper, BoundFunctionWrapper)
