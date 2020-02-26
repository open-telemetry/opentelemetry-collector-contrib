import time

# 3rd party
from django.template import Context, Template

# testing
from .utils import DjangoTraceTestCase, override_ddtrace_settings


class DjangoTemplateTest(DjangoTraceTestCase):
    """
    Ensures that the template system is properly traced
    """
    def test_template(self):
        # prepare a base template using the default engine
        template = Template('Hello {{name}}!')
        ctx = Context({'name': 'Django'})

        # (trace) the template rendering
        start = time.time()
        assert template.render(ctx) == 'Hello Django!'
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert spans, spans
        assert len(spans) == 1

        span = spans[0]
        assert span.span_type == 'template'
        assert span.name == 'django.template'
        assert span.get_tag('django.template_name') == 'unknown'
        assert start < span.start < span.start + span.duration < end

    @override_ddtrace_settings(INSTRUMENT_TEMPLATE=False)
    def test_template_disabled(self):
        # prepare a base template using the default engine
        template = Template('Hello {{name}}!')
        ctx = Context({'name': 'Django'})

        # (trace) the template rendering
        assert template.render(ctx) == 'Hello Django!'

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 0
