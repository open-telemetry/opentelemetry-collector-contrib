import mako
from mako.template import Template

from ...ext import SpanTypes
from ...pin import Pin
from ...utils.importlib import func_name
from ...utils.wrappers import unwrap as _u
from ...vendor.wrapt import wrap_function_wrapper as _w
from .constants import DEFAULT_TEMPLATE_NAME


def patch():
    if getattr(mako, '__datadog_patch', False):
        # already patched
        return
    setattr(mako, '__datadog_patch', True)

    Pin(service='mako', app='mako').onto(Template)

    _w(mako, 'template.Template.render', _wrap_render)
    _w(mako, 'template.Template.render_unicode', _wrap_render)
    _w(mako, 'template.Template.render_context', _wrap_render)


def unpatch():
    if not getattr(mako, '__datadog_patch', False):
        return
    setattr(mako, '__datadog_patch', False)

    _u(mako.template.Template, 'render')
    _u(mako.template.Template, 'render_unicode')
    _u(mako.template.Template, 'render_context')


def _wrap_render(wrapped, instance, args, kwargs):
    pin = Pin.get_from(instance)
    if not pin or not pin.enabled():
        return wrapped(*args, **kwargs)

    template_name = instance.filename or DEFAULT_TEMPLATE_NAME
    with pin.tracer.trace(func_name(wrapped), pin.service, span_type=SpanTypes.TEMPLATE) as span:
        try:
            template = wrapped(*args, **kwargs)
            return template
        finally:
            span.resource = template_name
            span.set_tag('mako.template_name', template_name)
