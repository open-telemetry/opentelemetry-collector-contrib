from tornado import template

from ddtrace import Pin

from ...ext import SpanTypes


def generate(func, renderer, args, kwargs):
    """
    Wrap the ``generate`` method used in templates rendering. Because the method
    may be called everywhere, the execution is traced in a tracer StackContext that
    inherits the current one if it's already available.
    """
    # get the module pin
    pin = Pin.get_from(template)
    if not pin or not pin.enabled():
        return func(*args, **kwargs)

    # change the resource and the template name
    # if it's created from a string instead of a file
    if '<string>' in renderer.name:
        resource = template_name = 'render_string'
    else:
        resource = template_name = renderer.name

    # trace the original call
    with pin.tracer.trace(
        'tornado.template', service=pin.service, resource=resource, span_type=SpanTypes.TEMPLATE
    ) as span:
        span.set_meta('tornado.template_name', template_name)
        return func(*args, **kwargs)
