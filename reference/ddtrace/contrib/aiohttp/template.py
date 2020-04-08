import aiohttp_jinja2

from ddtrace import Pin

from ...ext import SpanTypes


def _trace_render_template(func, module, args, kwargs):
    """
    Trace the template rendering
    """
    # get the module pin
    pin = Pin.get_from(aiohttp_jinja2)
    if not pin or not pin.enabled():
        return func(*args, **kwargs)

    # original signature:
    # render_template(template_name, request, context, *, app_key=APP_KEY, encoding='utf-8')
    template_name = args[0]
    request = args[1]
    env = aiohttp_jinja2.get_env(request.app)

    # the prefix is available only on PackageLoader
    template_prefix = getattr(env.loader, 'package_path', '')
    template_meta = '{}/{}'.format(template_prefix, template_name)

    with pin.tracer.trace('aiohttp.template', span_type=SpanTypes.TEMPLATE) as span:
        span.set_meta('aiohttp.template', template_meta)
        return func(*args, **kwargs)
