import os.path
import unittest

# 3rd party
import jinja2

from ddtrace import Pin, config
from ddtrace.contrib.jinja2 import patch, unpatch
from tests.test_tracer import get_dummy_tracer

TEST_DIR = os.path.dirname(os.path.realpath(__file__))
TMPL_DIR = os.path.join(TEST_DIR, 'templates')


class Jinja2Test(unittest.TestCase):
    def setUp(self):
        patch()
        # prevent cache effects when using Template('code...')
        jinja2.environment._spontaneous_environments.clear()
        # provide a dummy tracer
        self.tracer = get_dummy_tracer()
        Pin.override(jinja2.environment.Environment, tracer=self.tracer)

    def tearDown(self):
        # restore the tracer
        unpatch()

    def test_render_inline_template(self):
        t = jinja2.environment.Template('Hello {{name}}!')
        assert t.render(name='Jinja') == 'Hello Jinja!'

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 2

        for span in spans:
            assert span.service is None
            assert span.span_type == 'template'
            assert span.get_tag('jinja2.template_name') == '<memory>'

        assert spans[0].name == 'jinja2.compile'
        assert spans[1].name == 'jinja2.render'

    def test_generate_inline_template(self):
        t = jinja2.environment.Template('Hello {{name}}!')
        assert ''.join(t.generate(name='Jinja')) == 'Hello Jinja!'

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 2

        for span in spans:
            assert span.service is None
            assert span.span_type == 'template'
            assert span.get_tag('jinja2.template_name') == '<memory>'

        assert spans[0].name == 'jinja2.compile'
        assert spans[1].name == 'jinja2.render'

    def test_file_template(self):
        loader = jinja2.loaders.FileSystemLoader(TMPL_DIR)
        env = jinja2.Environment(loader=loader)
        t = env.get_template('template.html')
        assert t.render(name='Jinja') == 'Message: Hello Jinja!'

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 5

        for span in spans:
            assert span.span_type == 'template'
            assert span.service is None

        # templates.html extends base.html
        def get_def(s):
            return s.name, s.get_tag('jinja2.template_name')

        assert get_def(spans[0]) == ('jinja2.load', 'template.html')
        assert get_def(spans[1]) == ('jinja2.compile', 'template.html')
        assert get_def(spans[2]) == ('jinja2.render', 'template.html')
        assert get_def(spans[3]) == ('jinja2.load', 'base.html')
        assert get_def(spans[4]) == ('jinja2.compile', 'base.html')

        # additionnal checks for jinja2.load
        assert spans[0].get_tag('jinja2.template_path') == os.path.join(TMPL_DIR, 'template.html')
        assert spans[3].get_tag('jinja2.template_path') == os.path.join(TMPL_DIR, 'base.html')

    def test_service_name(self):
        # don't inherit the service name from the parent span, but force the value.
        loader = jinja2.loaders.FileSystemLoader(TMPL_DIR)
        env = jinja2.Environment(loader=loader)

        cfg = config.get_from(env)
        cfg['service_name'] = 'renderer'

        t = env.get_template('template.html')
        assert t.render(name='Jinja') == 'Message: Hello Jinja!'

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 5

        for span in spans:
            assert span.service == 'renderer'

    def test_inherit_service(self):
        # When there is a parent span and no custom service_name, the service name is inherited
        loader = jinja2.loaders.FileSystemLoader(TMPL_DIR)
        env = jinja2.Environment(loader=loader)

        with self.tracer.trace('parent.span', service='web'):
            t = env.get_template('template.html')
            assert t.render(name='Jinja') == 'Message: Hello Jinja!'

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 6

        for span in spans:
            assert span.service == 'web'
