"""
The bottle integration traces the Bottle web framework. Add the following
plugin to your app::

    import bottle
    from ddtrace import tracer
    from ddtrace.contrib.bottle import TracePlugin

    app = bottle.Bottle()
    plugin = TracePlugin(service="my-web-app")
    app.install(plugin)
"""

from ...utils.importlib import require_modules

required_modules = ['bottle']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .trace import TracePlugin
        from .patch import patch

        __all__ = ['TracePlugin', 'patch']
