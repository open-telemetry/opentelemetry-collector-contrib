"""
To trace the falcon web framework, install the trace middleware::

    import falcon
    from ddtrace import tracer
    from ddtrace.contrib.falcon import TraceMiddleware

    mw = TraceMiddleware(tracer, 'my-falcon-app')
    falcon.API(middleware=[mw])

You can also use the autopatching functionality::

    import falcon
    from ddtrace import tracer, patch

    patch(falcon=True)

    app = falcon.API()

To disable distributed tracing when using autopatching, set the
``DATADOG_FALCON_DISTRIBUTED_TRACING`` environment variable to ``False``.

To enable generating APM events for Trace Search & Analytics, set the
``DD_FALCON_ANALYTICS_ENABLED`` environment variable to ``True``.

**Supported span hooks**

The following is a list of available tracer hooks that can be used to intercept
and modify spans created by this integration.

- ``request``
    - Called before the response has been finished
    - ``def on_falcon_request(span, request, response)``


Example::

    import falcon
    from ddtrace import config, patch_all
    patch_all()

    app = falcon.API()

    @config.falcon.hooks.on('request')
    def on_falcon_request(span, request, response):
        span.set_tag('my.custom', 'tag')

:ref:`Headers tracing <http-headers-tracing>` is supported for this integration.
"""
from ...utils.importlib import require_modules

required_modules = ['falcon']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .middleware import TraceMiddleware
        from .patch import patch

        __all__ = ['TraceMiddleware', 'patch']
