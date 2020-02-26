import os

from .trace import TracePlugin

import bottle

from ddtrace.vendor import wrapt


def patch():
    """Patch the bottle.Bottle class
    """
    if getattr(bottle, '_datadog_patch', False):
        return

    setattr(bottle, '_datadog_patch', True)
    wrapt.wrap_function_wrapper('bottle', 'Bottle.__init__', traced_init)


def traced_init(wrapped, instance, args, kwargs):
    wrapped(*args, **kwargs)

    service = os.environ.get('DATADOG_SERVICE_NAME') or 'bottle'

    plugin = TracePlugin(service=service)
    instance.install(plugin)
