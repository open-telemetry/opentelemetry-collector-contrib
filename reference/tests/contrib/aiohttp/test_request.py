import threading
import asyncio
import aiohttp_jinja2

from urllib import request
from aiohttp.test_utils import unittest_run_loop

from ddtrace.pin import Pin
from ddtrace.contrib.aiohttp.patch import patch, unpatch
from ddtrace.contrib.aiohttp.middlewares import trace_app

from .utils import TraceTestCase


class TestRequestTracing(TraceTestCase):
    """
    Ensures that the trace includes all traced components.
    """
    def enable_tracing(self):
        # enabled tracing:
        #   * middleware
        #   * templates
        trace_app(self.app, self.tracer)
        patch()
        Pin.override(aiohttp_jinja2, tracer=self.tracer)

    def disable_tracing(self):
        unpatch()

    @unittest_run_loop
    @asyncio.coroutine
    def test_full_request(self):
        # it should create a root span when there is a handler hit
        # with the proper tags
        request = yield from self.client.request('GET', '/template/')
        assert 200 == request.status
        yield from request.text()
        # the trace is created
        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])
        request_span = traces[0][0]
        template_span = traces[0][1]
        # request
        assert 'aiohttp-web' == request_span.service
        assert 'aiohttp.request' == request_span.name
        assert 'GET /template/' == request_span.resource
        # template
        assert 'aiohttp-web' == template_span.service
        assert 'aiohttp.template' == template_span.name
        assert 'aiohttp.template' == template_span.resource

    @unittest_run_loop
    @asyncio.coroutine
    def test_multiple_full_request(self):
        # it should handle multiple requests using the same loop
        def make_requests():
            url = self.client.make_url('/delayed/')
            response = request.urlopen(str(url)).read().decode('utf-8')
            assert 'Done' == response

        # blocking call executed in different threads
        threads = [threading.Thread(target=make_requests) for _ in range(10)]
        for t in threads:
            t.daemon = True
            t.start()

        # we should yield so that this loop can handle
        # threads' requests
        yield from asyncio.sleep(0.5)
        for t in threads:
            t.join(timeout=0.5)

        # the trace is created
        traces = self.tracer.writer.pop_traces()
        assert 10 == len(traces)
        assert 1 == len(traces[0])
