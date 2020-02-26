"""
code to measure django template rendering.
"""
# project
from ...ext import SpanTypes
from ...internal.logger import get_logger

# 3p
from django.template import Template

log = get_logger(__name__)

RENDER_ATTR = '_datadog_original_render'


def patch_template(tracer):
    """ will patch django's template rendering function to include timing
        and trace information.
    """

    # FIXME[matt] we're patching the template class here. ideally we'd only
    # patch so we can use multiple tracers at once, but i suspect this is fine
    # in practice.
    if getattr(Template, RENDER_ATTR, None):
        log.debug('already patched')
        return

    setattr(Template, RENDER_ATTR, Template.render)

    def traced_render(self, context):
        with tracer.trace('django.template', span_type=SpanTypes.TEMPLATE) as span:
            try:
                return Template._datadog_original_render(self, context)
            finally:
                template_name = self.name or getattr(context, 'template_name', None) or 'unknown'
                span.resource = template_name
                span.set_tag('django.template_name', template_name)

    Template.render = traced_render


def unpatch_template():
    render = getattr(Template, RENDER_ATTR, None)
    if render is None:
        log.debug('nothing to do Template is already patched')
        return
    Template.render = render
    delattr(Template, RENDER_ATTR)
