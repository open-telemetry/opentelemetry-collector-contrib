import bottle
import webtest

import ddtrace
from ddtrace import compat
from ddtrace.contrib.bottle import TracePlugin

from ...base import BaseTracerTestCase
from ...utils import assert_span_http_status_code

SERVICE = 'bottle-app'


class TraceBottleDistributedTest(BaseTracerTestCase):
    """
    Ensures that Bottle is properly traced.
    """
    def setUp(self):
        super(TraceBottleDistributedTest, self).setUp()

        # provide a dummy tracer
        self._original_tracer = ddtrace.tracer
        ddtrace.tracer = self.tracer
        # provide a Bottle app
        self.app = bottle.Bottle()

    def tearDown(self):
        # restore the tracer
        ddtrace.tracer = self._original_tracer

    def _trace_app_distributed(self, tracer=None):
        self.app.install(TracePlugin(service=SERVICE, tracer=tracer))
        self.app = webtest.TestApp(self.app)

    def _trace_app_not_distributed(self, tracer=None):
        self.app.install(TracePlugin(service=SERVICE, tracer=tracer, distributed_tracing=False))
        self.app = webtest.TestApp(self.app)

    def test_distributed(self):
        # setup our test app
        @self.app.route('/hi/<name>')
        def hi(name):
            return 'hi %s' % name
        self._trace_app_distributed(self.tracer)

        # make a request
        headers = {'x-datadog-trace-id': '123',
                   'x-datadog-parent-id': '456'}
        resp = self.app.get('/hi/dougie', headers=headers)
        assert resp.status_int == 200
        assert compat.to_unicode(resp.body) == u'hi dougie'

        # validate it's traced
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.name == 'bottle.request'
        assert s.service == 'bottle-app'
        assert s.resource == 'GET /hi/<name>'
        assert_span_http_status_code(s, 200)
        assert s.get_tag('http.method') == 'GET'
        # check distributed headers
        assert 123 == s.trace_id
        assert 456 == s.parent_id

    def test_not_distributed(self):
        # setup our test app
        @self.app.route('/hi/<name>')
        def hi(name):
            return 'hi %s' % name
        self._trace_app_not_distributed(self.tracer)

        # make a request
        headers = {'x-datadog-trace-id': '123',
                   'x-datadog-parent-id': '456'}
        resp = self.app.get('/hi/dougie', headers=headers)
        assert resp.status_int == 200
        assert compat.to_unicode(resp.body) == u'hi dougie'

        # validate it's traced
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.name == 'bottle.request'
        assert s.service == 'bottle-app'
        assert s.resource == 'GET /hi/<name>'
        assert_span_http_status_code(s, 200)
        assert s.get_tag('http.method') == 'GET'
        # check distributed headers
        assert 123 != s.trace_id
        assert 456 != s.parent_id
