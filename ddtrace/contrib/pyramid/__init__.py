r"""To trace requests from a Pyramid application, trace your application
config::


    from pyramid.config import Configurator
    from ddtrace.contrib.pyramid import trace_pyramid

    settings = {
        'datadog_trace_service' : 'my-web-app-name',
    }

    config = Configurator(settings=settings)
    trace_pyramid(config)

    # use your config as normal.
    config.add_route('index', '/')

Available settings are:

* ``datadog_trace_service``: change the `pyramid` service name
* ``datadog_trace_enabled``: sets if the Tracer is enabled or not
* ``datadog_distributed_tracing``: set it to ``False`` to disable Distributed Tracing
* ``datadog_analytics_enabled``: set it to ``True`` to enable generating APM events for Trace Search & Analytics

If you use the ``pyramid.tweens`` settings value to set the tweens for your
application, you need to add ``ddtrace.contrib.pyramid:trace_tween_factory``
explicitly to the list. For example::

    settings = {
        'datadog_trace_service' : 'my-web-app-name',
        'pyramid.tweens', 'your_tween_no_1\\nyour_tween_no_2\\nddtrace.contrib.pyramid:trace_tween_factory',
    }

    config = Configurator(settings=settings)
    trace_pyramid(config)

    # use your config as normal.
    config.add_route('index', '/')

"""

from ...utils.importlib import require_modules


required_modules = ['pyramid']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .trace import trace_pyramid, trace_tween_factory, includeme
        from .patch import patch

        __all__ = [
            'patch',
            'trace_pyramid',
            'trace_tween_factory',
            'includeme',
        ]
