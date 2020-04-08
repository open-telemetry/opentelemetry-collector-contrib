import os.path
import unittest

# 3rd party
from mako.template import Template
from mako.lookup import TemplateLookup
from mako.runtime import Context

from ddtrace import Pin
from ddtrace.contrib.mako import patch, unpatch
from ddtrace.compat import StringIO, to_unicode
from tests.test_tracer import get_dummy_tracer

TEST_DIR = os.path.dirname(os.path.realpath(__file__))
TMPL_DIR = os.path.join(TEST_DIR, 'templates')


class MakoTest(unittest.TestCase):
    def setUp(self):
        patch()
        self.tracer = get_dummy_tracer()
        Pin.override(Template, tracer=self.tracer)

    def tearDown(self):
        unpatch()

    def test_render(self):
        # render
        t = Template('Hello ${name}!')
        self.assertEqual(t.render(name='mako'), 'Hello mako!')

        spans = self.tracer.writer.pop()
        self.assertEqual(len(spans), 1)

        self.assertEqual(spans[0].service, 'mako')
        self.assertEqual(spans[0].span_type, 'template')
        self.assertEqual(spans[0].get_tag('mako.template_name'), '<memory>')
        self.assertEqual(spans[0].name, 'mako.template.render')
        self.assertEqual(spans[0].resource, '<memory>')

        # render_unicode
        t = Template('Hello ${name}!')
        self.assertEqual(t.render_unicode(name='mako'), to_unicode('Hello mako!'))
        spans = self.tracer.writer.pop()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].service, 'mako')
        self.assertEqual(spans[0].span_type, 'template')
        self.assertEqual(spans[0].get_tag('mako.template_name'), '<memory>')
        self.assertEqual(spans[0].name, 'mako.template.render_unicode')
        self.assertEqual(spans[0].resource, '<memory>')

        # render_context
        t = Template('Hello ${name}!')
        buf = StringIO()
        c = Context(buf, name='mako')
        t.render_context(c)
        self.assertEqual(buf.getvalue(), 'Hello mako!')
        spans = self.tracer.writer.pop()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].service, 'mako')
        self.assertEqual(spans[0].span_type, 'template')
        self.assertEqual(spans[0].get_tag('mako.template_name'), '<memory>')
        self.assertEqual(spans[0].name, 'mako.template.render_context')
        self.assertEqual(spans[0].resource, '<memory>')

    def test_file_template(self):
        tmpl_lookup = TemplateLookup(directories=[TMPL_DIR])
        t = tmpl_lookup.get_template('template.html')
        self.assertEqual(t.render(name='mako'), 'Hello mako!\n')

        spans = self.tracer.writer.pop()
        self.assertEqual(len(spans), 1)

        template_name = os.path.join(TMPL_DIR, 'template.html')

        self.assertEqual(spans[0].span_type, 'template')
        self.assertEqual(spans[0].service, 'mako')
        self.assertEqual(spans[0].get_tag('mako.template_name'), template_name)
        self.assertEqual(spans[0].name, 'mako.template.render')
        self.assertEqual(spans[0].resource, template_name)
