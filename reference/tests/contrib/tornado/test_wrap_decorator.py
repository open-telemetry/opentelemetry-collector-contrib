from ddtrace.ext import http

from .utils import TornadoTestCase
from ...utils import assert_span_http_status_code


class TestTornadoWebWrapper(TornadoTestCase):
    """
    Ensure that Tracer.wrap() works with Tornado web handlers.
    """
    def test_nested_wrap_handler(self):
        # it should trace a handler that calls a coroutine
        response = self.fetch('/nested_wrap/')
        assert 200 == response.code
        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])
        # check request span
        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.NestedWrapHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 200)
        assert self.get_url('/nested_wrap/') == request_span.get_tag(http.URL)
        assert 0 == request_span.error
        # check nested span
        nested_span = traces[0][1]
        assert 'tornado-web' == nested_span.service
        assert 'tornado.coro' == nested_span.name
        assert 0 == nested_span.error
        # check durations because of the yield sleep
        assert request_span.duration >= 0.05
        assert nested_span.duration >= 0.05

    def test_nested_exception_wrap_handler(self):
        # it should trace a handler that calls a coroutine that raises an exception
        response = self.fetch('/nested_exception_wrap/')
        assert 500 == response.code
        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])
        # check request span
        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.NestedExceptionWrapHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 500)
        assert self.get_url('/nested_exception_wrap/') == request_span.get_tag(http.URL)
        assert 1 == request_span.error
        assert 'Ouch!' == request_span.get_tag('error.msg')
        assert 'Exception: Ouch!' in request_span.get_tag('error.stack')
        # check nested span
        nested_span = traces[0][1]
        assert 'tornado-web' == nested_span.service
        assert 'tornado.coro' == nested_span.name
        assert 1 == nested_span.error
        assert 'Ouch!' == nested_span.get_tag('error.msg')
        assert 'Exception: Ouch!' in nested_span.get_tag('error.stack')
        # check durations because of the yield sleep
        assert request_span.duration >= 0.05
        assert nested_span.duration >= 0.05

    def test_sync_nested_wrap_handler(self):
        # it should trace a handler that calls a coroutine
        response = self.fetch('/sync_nested_wrap/')
        assert 200 == response.code
        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])
        # check request span
        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.SyncNestedWrapHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 200)
        assert self.get_url('/sync_nested_wrap/') == request_span.get_tag(http.URL)
        assert 0 == request_span.error
        # check nested span
        nested_span = traces[0][1]
        assert 'tornado-web' == nested_span.service
        assert 'tornado.func' == nested_span.name
        assert 0 == nested_span.error
        # check durations because of the yield sleep
        assert request_span.duration >= 0.05
        assert nested_span.duration >= 0.05

    def test_sync_nested_exception_wrap_handler(self):
        # it should trace a handler that calls a coroutine that raises an exception
        response = self.fetch('/sync_nested_exception_wrap/')
        assert 500 == response.code
        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])
        # check request span
        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.SyncNestedExceptionWrapHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 500)
        assert self.get_url('/sync_nested_exception_wrap/') == request_span.get_tag(http.URL)
        assert 1 == request_span.error
        assert 'Ouch!' == request_span.get_tag('error.msg')
        assert 'Exception: Ouch!' in request_span.get_tag('error.stack')
        # check nested span
        nested_span = traces[0][1]
        assert 'tornado-web' == nested_span.service
        assert 'tornado.func' == nested_span.name
        assert 1 == nested_span.error
        assert 'Ouch!' == nested_span.get_tag('error.msg')
        assert 'Exception: Ouch!' in nested_span.get_tag('error.stack')
        # check durations because of the yield sleep
        assert request_span.duration >= 0.05
        assert nested_span.duration >= 0.05

    def test_nested_wrap_executor_handler(self):
        # it should trace a handler that calls a blocking function in a different executor
        response = self.fetch('/executor_wrap_handler/')
        assert 200 == response.code
        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])
        # check request span
        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.ExecutorWrapHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 200)
        assert self.get_url('/executor_wrap_handler/') == request_span.get_tag(http.URL)
        assert 0 == request_span.error
        # check nested span in the executor
        nested_span = traces[0][1]
        assert 'tornado-web' == nested_span.service
        assert 'tornado.executor.wrap' == nested_span.name
        assert 0 == nested_span.error
        # check durations because of the yield sleep
        assert request_span.duration >= 0.05
        assert nested_span.duration >= 0.05

    def test_nested_exception_wrap_executor_handler(self):
        # it should trace a handler that calls a blocking function in a different
        # executor that raises an exception
        response = self.fetch('/executor_wrap_exception/')
        assert 500 == response.code
        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])
        # check request span
        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.ExecutorExceptionWrapHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 500)
        assert self.get_url('/executor_wrap_exception/') == request_span.get_tag(http.URL)
        assert 1 == request_span.error
        assert 'Ouch!' == request_span.get_tag('error.msg')
        assert 'Exception: Ouch!' in request_span.get_tag('error.stack')
        # check nested span
        nested_span = traces[0][1]
        assert 'tornado-web' == nested_span.service
        assert 'tornado.executor.wrap' == nested_span.name
        assert 1 == nested_span.error
        assert 'Ouch!' == nested_span.get_tag('error.msg')
        assert 'Exception: Ouch!' in nested_span.get_tag('error.stack')
        # check durations because of the yield sleep
        assert request_span.duration >= 0.05
        assert nested_span.duration >= 0.05
