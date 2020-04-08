"""
The ``aiohttp`` integration traces all requests defined in the application handlers.
Auto instrumentation is available using the ``trace_app`` function::

    from aiohttp import web
    from ddtrace import tracer, patch
    from ddtrace.contrib.aiohttp import trace_app

    # patch third-party modules like aiohttp_jinja2
    patch(aiohttp=True)

    # create your application
    app = web.Application()
    app.router.add_get('/', home_handler)

    # trace your application handlers
    trace_app(app, tracer, service='async-api')
    web.run_app(app, port=8000)

Integration settings are attached to your application under the ``datadog_trace``
namespace. You can read or update them as follows::

    # disables distributed tracing for all received requests
    app['datadog_trace']['distributed_tracing_enabled'] = False

Available settings are:

* ``tracer`` (default: ``ddtrace.tracer``): set the default tracer instance that is used to
  trace `aiohttp` internals. By default the `ddtrace` tracer is used.
* ``service`` (default: ``aiohttp-web``): set the service name used by the tracer. Usually
  this configuration must be updated with a meaningful name.
* ``distributed_tracing_enabled`` (default: ``True``): enable distributed tracing during
  the middleware execution, so that a new span is created with the given ``trace_id`` and
  ``parent_id`` injected via request headers.
* ``analytics_enabled`` (default: ``None``): enables APM events in Trace Search & Analytics.

Third-party modules that are currently supported by the ``patch()`` method are:

* ``aiohttp_jinja2``

When a request span is created, a new ``Context`` for this logical execution is attached
to the ``request`` object, so that it can be used in the application code::

    async def home_handler(request):
        ctx = request['datadog_context']
        # do something with the tracing Context
"""
from ...utils.importlib import require_modules

required_modules = ['aiohttp']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch, unpatch
        from .middlewares import trace_app

        __all__ = [
            'patch',
            'unpatch',
            'trace_app',
        ]
