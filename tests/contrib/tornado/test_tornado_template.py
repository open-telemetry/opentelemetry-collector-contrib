from tornado import template

import pytest

from .utils import TornadoTestCase
from ...utils import assert_span_http_status_code

from ddtrace.ext import http


class TestTornadoTemplate(TornadoTestCase):
    """
    Ensure that Tornado templates are properly traced inside and
    outside web handlers.
    """
    def test_template_handler(self):
        # it should trace the template rendering
        response = self.fetch('/template/')
        assert 200 == response.code
        assert 'This is a rendered page called "home"\n' == response.body.decode('utf-8')

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])

        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.TemplateHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 200)
        assert self.get_url('/template/') == request_span.get_tag(http.URL)
        assert 0 == request_span.error

        template_span = traces[0][1]
        assert 'tornado-web' == template_span.service
        assert 'tornado.template' == template_span.name
        assert 'template' == template_span.span_type
        assert 'templates/page.html' == template_span.resource
        assert 'templates/page.html' == template_span.get_tag('tornado.template_name')
        assert template_span.parent_id == request_span.span_id
        assert 0 == template_span.error

    def test_template_renderer(self):
        # it should trace the Template generation even outside web handlers
        t = template.Template('Hello {{ name }}!')
        value = t.generate(name='world')
        assert value == b'Hello world!'

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])

        template_span = traces[0][0]
        assert 'tornado-web' == template_span.service
        assert 'tornado.template' == template_span.name
        assert 'template' == template_span.span_type
        assert 'render_string' == template_span.resource
        assert 'render_string' == template_span.get_tag('tornado.template_name')
        assert 0 == template_span.error

    def test_template_partials(self):
        # it should trace the template rendering when partials are used
        response = self.fetch('/template_partial/')
        assert 200 == response.code
        assert 'This is a list:\n\n* python\n\n\n* go\n\n\n* ruby\n\n\n' == response.body.decode('utf-8')

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 5 == len(traces[0])

        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.TemplatePartialHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 200)
        assert self.get_url('/template_partial/') == request_span.get_tag(http.URL)
        assert 0 == request_span.error

        template_root = traces[0][1]
        assert 'tornado-web' == template_root.service
        assert 'tornado.template' == template_root.name
        assert 'template' == template_root.span_type
        assert 'templates/list.html' == template_root.resource
        assert 'templates/list.html' == template_root.get_tag('tornado.template_name')
        assert template_root.parent_id == request_span.span_id
        assert 0 == template_root.error

        template_span = traces[0][2]
        assert 'tornado-web' == template_span.service
        assert 'tornado.template' == template_span.name
        assert 'template' == template_span.span_type
        assert 'templates/item.html' == template_span.resource
        assert 'templates/item.html' == template_span.get_tag('tornado.template_name')
        assert template_span.parent_id == template_root.span_id
        assert 0 == template_span.error

        template_span = traces[0][3]
        assert 'tornado-web' == template_span.service
        assert 'tornado.template' == template_span.name
        assert 'template' == template_span.span_type
        assert 'templates/item.html' == template_span.resource
        assert 'templates/item.html' == template_span.get_tag('tornado.template_name')
        assert template_span.parent_id == template_root.span_id
        assert 0 == template_span.error

        template_span = traces[0][4]
        assert 'tornado-web' == template_span.service
        assert 'tornado.template' == template_span.name
        assert 'template' == template_span.span_type
        assert 'templates/item.html' == template_span.resource
        assert 'templates/item.html' == template_span.get_tag('tornado.template_name')
        assert template_span.parent_id == template_root.span_id
        assert 0 == template_span.error

    def test_template_exception_handler(self):
        # it should trace template rendering exceptions
        response = self.fetch('/template_exception/')
        assert 500 == response.code

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])

        request_span = traces[0][0]
        assert 'tornado-web' == request_span.service
        assert 'tornado.request' == request_span.name
        assert 'web' == request_span.span_type
        assert 'tests.contrib.tornado.web.app.TemplateExceptionHandler' == request_span.resource
        assert 'GET' == request_span.get_tag('http.method')
        assert_span_http_status_code(request_span, 500)
        assert self.get_url('/template_exception/') == request_span.get_tag(http.URL)
        assert 1 == request_span.error
        assert 'ModuleThatDoesNotExist' in request_span.get_tag('error.msg')
        assert 'AttributeError' in request_span.get_tag('error.stack')

        template_span = traces[0][1]
        assert 'tornado-web' == template_span.service
        assert 'tornado.template' == template_span.name
        assert 'template' == template_span.span_type
        assert 'templates/exception.html' == template_span.resource
        assert 'templates/exception.html' == template_span.get_tag('tornado.template_name')
        assert template_span.parent_id == request_span.span_id
        assert 1 == template_span.error
        assert 'ModuleThatDoesNotExist' in template_span.get_tag('error.msg')
        assert 'AttributeError' in template_span.get_tag('error.stack')

    def test_template_renderer_exception(self):
        # it should trace the Template exceptions generation even outside web handlers
        t = template.Template('{% module ModuleThatDoesNotExist() %}')
        with pytest.raises(NameError):
            t.generate()

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])

        template_span = traces[0][0]
        assert 'tornado-web' == template_span.service
        assert 'tornado.template' == template_span.name
        assert 'template' == template_span.span_type
        assert 'render_string' == template_span.resource
        assert 'render_string' == template_span.get_tag('tornado.template_name')
        assert 1 == template_span.error
        assert 'is not defined' in template_span.get_tag('error.msg')
        assert 'NameError' in template_span.get_tag('error.stack')
