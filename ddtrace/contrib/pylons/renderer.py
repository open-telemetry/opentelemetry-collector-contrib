import pylons

from pylons import config

from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

from .compat import legacy_pylons
from .constants import CONFIG_MIDDLEWARE


def trace_rendering():
    """Patch all Pylons renderers. It supports multiple versions
    of Pylons and multiple renderers.
    """
    # patch only once
    if getattr(pylons.templating, '__datadog_patch', False):
        return
    setattr(pylons.templating, '__datadog_patch', True)

    if legacy_pylons:
        # Pylons <= 0.9.7
        _w('pylons.templating', 'render', _traced_renderer)
    else:
        # Pylons > 0.9.7
        _w('pylons.templating', 'render_mako', _traced_renderer)
        _w('pylons.templating', 'render_mako_def', _traced_renderer)
        _w('pylons.templating', 'render_genshi', _traced_renderer)
        _w('pylons.templating', 'render_jinja2', _traced_renderer)


def _traced_renderer(wrapped, instance, args, kwargs):
    """Traced renderer"""
    tracer = config[CONFIG_MIDDLEWARE]._tracer
    with tracer.trace('pylons.render') as span:
        span.set_tag('template.name', args[0])
        return wrapped(*args, **kwargs)
