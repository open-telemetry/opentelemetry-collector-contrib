# -*- coding: utf-8 -*-
import time
import re

from unittest import TestCase

from ddtrace.contrib.flask import TraceMiddleware
from ddtrace.constants import SAMPLING_PRIORITY_KEY
from ddtrace.ext import http, errors

from tests.opentracer.utils import init_tracer
from .web import create_app
from ...test_tracer import get_dummy_tracer
from ...utils import assert_span_http_status_code


class TestFlask(TestCase):
    """Ensures Flask is properly instrumented."""

    def setUp(self):
        self.tracer = get_dummy_tracer()
        self.flask_app = create_app()
        self.traced_app = TraceMiddleware(
            self.flask_app,
            self.tracer,
            service='test.flask.service',
            distributed_tracing=True,
        )

        # make the app testable
        self.flask_app.config['TESTING'] = True
        self.app = self.flask_app.test_client()

    def test_double_instrumentation(self):
        # ensure Flask is never instrumented twice when `ddtrace-run`
        # and `TraceMiddleware` are used together. `traced_app` MUST
        # be assigned otherwise it's not possible to reproduce the
        # problem (the test scope must keep a strong reference)
        traced_app = TraceMiddleware(self.flask_app, self.tracer)  # noqa: F841
        rv = self.app.get('/child')
        assert rv.status_code == 200
        spans = self.tracer.writer.pop()
        assert len(spans) == 2

    def test_double_instrumentation_config(self):
        # ensure Flask uses the last set configuration to be sure
        # there are no breaking changes for who uses `ddtrace-run`
        # with the `TraceMiddleware`
        TraceMiddleware(
            self.flask_app,
            self.tracer,
            service='new-intake',
            distributed_tracing=False,
        )
        assert self.flask_app._service == 'new-intake'
        assert self.flask_app._use_distributed_tracing is False
        rv = self.app.get('/child')
        assert rv.status_code == 200
        spans = self.tracer.writer.pop()
        assert len(spans) == 2

    def test_child(self):
        start = time.time()
        rv = self.app.get('/child')
        end = time.time()
        # ensure request worked
        assert rv.status_code == 200
        assert rv.data == b'child'
        # ensure trace worked
        spans = self.tracer.writer.pop()
        assert len(spans) == 2

        spans_by_name = {s.name: s for s in spans}

        s = spans_by_name['flask.request']
        assert s.span_id
        assert s.trace_id
        assert not s.parent_id
        assert s.service == 'test.flask.service'
        assert s.resource == 'child'
        assert s.start >= start
        assert s.duration <= end - start
        assert s.error == 0

        c = spans_by_name['child']
        assert c.span_id
        assert c.trace_id == s.trace_id
        assert c.parent_id == s.span_id
        assert c.service == 'test.flask.service'
        assert c.resource == 'child'
        assert c.start >= start
        assert c.duration <= end - start
        assert c.error == 0

    def test_success(self):
        start = time.time()
        rv = self.app.get('/')
        end = time.time()

        # ensure request worked
        assert rv.status_code == 200
        assert rv.data == b'hello'

        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.service == 'test.flask.service'
        assert s.resource == 'index'
        assert s.start >= start
        assert s.duration <= end - start
        assert s.error == 0
        assert_span_http_status_code(s, 200)
        assert s.meta.get(http.METHOD) == 'GET'

        services = self.tracer.writer.pop_services()
        expected = {}
        assert services == expected

    def test_template(self):
        start = time.time()
        rv = self.app.get('/tmpl')
        end = time.time()

        # ensure request worked
        assert rv.status_code == 200
        assert rv.data == b'hello earth'

        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = self.tracer.writer.pop()
        assert len(spans) == 2
        by_name = {s.name: s for s in spans}
        s = by_name['flask.request']
        assert s.service == 'test.flask.service'
        assert s.resource == 'tmpl'
        assert s.start >= start
        assert s.duration <= end - start
        assert s.error == 0
        assert_span_http_status_code(s, 200)
        assert s.meta.get(http.METHOD) == 'GET'

        t = by_name['flask.template']
        assert t.get_tag('flask.template') == 'test.html'
        assert t.parent_id == s.span_id
        assert t.trace_id == s.trace_id
        assert s.start < t.start < t.start + t.duration < end

    def test_handleme(self):
        start = time.time()
        rv = self.app.get('/handleme')
        end = time.time()

        # ensure request worked
        assert rv.status_code == 202
        assert rv.data == b'handled'

        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.service == 'test.flask.service'
        assert s.resource == 'handle_me'
        assert s.start >= start
        assert s.duration <= end - start
        assert s.error == 0
        assert_span_http_status_code(s, 202)
        assert s.meta.get(http.METHOD) == 'GET'

    def test_template_err(self):
        start = time.time()
        try:
            self.app.get('/tmpl/err')
        except Exception:
            pass
        else:
            assert 0
        end = time.time()

        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        by_name = {s.name: s for s in spans}
        s = by_name['flask.request']
        assert s.service == 'test.flask.service'
        assert s.resource == 'tmpl_err'
        assert s.start >= start
        assert s.duration <= end - start
        assert s.error == 1
        assert_span_http_status_code(s, 500)
        assert s.meta.get(http.METHOD) == 'GET'

    def test_template_render_err(self):
        start = time.time()
        try:
            self.app.get('/tmpl/render_err')
        except Exception:
            pass
        else:
            assert 0
        end = time.time()

        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = self.tracer.writer.pop()
        assert len(spans) == 2
        by_name = {s.name: s for s in spans}
        s = by_name['flask.request']
        assert s.service == 'test.flask.service'
        assert s.resource == 'tmpl_render_err'
        assert s.start >= start
        assert s.duration <= end - start
        assert s.error == 1
        assert_span_http_status_code(s, 500)
        assert s.meta.get(http.METHOD) == 'GET'
        t = by_name['flask.template']
        assert t.get_tag('flask.template') == 'render_err.html'
        assert t.error == 1
        assert t.parent_id == s.span_id
        assert t.trace_id == s.trace_id

    def test_error(self):
        start = time.time()
        rv = self.app.get('/error')
        end = time.time()

        # ensure the request itself worked
        assert rv.status_code == 500
        assert rv.data == b'error'

        # ensure the request was traced.
        assert not self.tracer.current_span()
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.service == 'test.flask.service'
        assert s.resource == 'error'
        assert s.start >= start
        assert s.duration <= end - start
        assert_span_http_status_code(s, 500)
        assert s.meta.get(http.METHOD) == 'GET'

    def test_fatal(self):
        if not self.traced_app.use_signals:
            return

        start = time.time()
        try:
            self.app.get('/fatal')
        except ZeroDivisionError:
            pass
        else:
            assert 0
        end = time.time()

        # ensure the request was traced.
        assert not self.tracer.current_span()
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.service == 'test.flask.service'
        assert s.resource == 'fatal'
        assert s.start >= start
        assert s.duration <= end - start
        assert_span_http_status_code(s, 500)
        assert s.meta.get(http.METHOD) == 'GET'
        assert 'ZeroDivisionError' in s.meta.get(errors.ERROR_TYPE), s.meta
        assert 'by zero' in s.meta.get(errors.ERROR_MSG)
        assert re.search('File ".*/contrib/flask/web.py", line [0-9]+, in fatal', s.meta.get(errors.ERROR_STACK))

    def test_unicode(self):
        start = time.time()
        rv = self.app.get(u'/üŋïĉóđē')
        end = time.time()

        # ensure request worked
        assert rv.status_code == 200
        assert rv.data == b'\xc3\xbc\xc5\x8b\xc3\xaf\xc4\x89\xc3\xb3\xc4\x91\xc4\x93'

        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.service == 'test.flask.service'
        assert s.resource == u'üŋïĉóđē'
        assert s.start >= start
        assert s.duration <= end - start
        assert s.error == 0
        assert_span_http_status_code(s, 200)
        assert s.meta.get(http.METHOD) == 'GET'
        assert s.meta.get(http.URL) == u'http://localhost/üŋïĉóđē'

    def test_404(self):
        start = time.time()
        rv = self.app.get(u'/404/üŋïĉóđē')
        end = time.time()

        # ensure that we hit a 404
        assert rv.status_code == 404

        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.service == 'test.flask.service'
        assert s.resource == u'404'
        assert s.start >= start
        assert s.duration <= end - start
        assert s.error == 0
        assert_span_http_status_code(s, 404)
        assert s.meta.get(http.METHOD) == 'GET'
        assert s.meta.get(http.URL) == u'http://localhost/404/üŋïĉóđē'

    def test_propagation(self):
        rv = self.app.get('/', headers={
            'x-datadog-trace-id': '1234',
            'x-datadog-parent-id': '4567',
            'x-datadog-sampling-priority': '2'
        })

        # ensure request worked
        assert rv.status_code == 200
        assert rv.data == b'hello'

        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]

        # ensure the propagation worked well
        assert s.trace_id == 1234
        assert s.parent_id == 4567
        assert s.get_metric(SAMPLING_PRIORITY_KEY) == 2

    def test_custom_span(self):
        rv = self.app.get('/custom_span')
        assert rv.status_code == 200
        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.service == 'test.flask.service'
        assert s.resource == 'overridden'
        assert s.error == 0
        assert_span_http_status_code(s, 200)
        assert s.meta.get(http.METHOD) == 'GET'

    def test_success_200_ot(self):
        """OpenTracing version of test_success_200."""
        ot_tracer = init_tracer('my_svc', self.tracer)
        writer = self.tracer.writer

        with ot_tracer.start_active_span('ot_span'):
            start = time.time()
            rv = self.app.get('/')
            end = time.time()

        # ensure request worked
        assert rv.status_code == 200
        assert rv.data == b'hello'

        # ensure trace worked
        assert not self.tracer.current_span(), self.tracer.current_span().pprint()
        spans = writer.pop()
        assert len(spans) == 2
        ot_span, dd_span = spans

        # confirm the parenting
        assert ot_span.parent_id is None
        assert dd_span.parent_id == ot_span.span_id

        assert ot_span.resource == 'ot_span'
        assert ot_span.service == 'my_svc'

        assert dd_span.resource == 'index'
        assert dd_span.start >= start
        assert dd_span.duration <= end - start
        assert dd_span.error == 0
        assert_span_http_status_code(dd_span, 200)
        assert dd_span.meta.get(http.METHOD) == 'GET'
