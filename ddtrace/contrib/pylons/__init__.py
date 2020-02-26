"""
The pylons trace middleware will track request timings. To
install the middleware, prepare your WSGI application and do
the following::

    from pylons.wsgiapp import PylonsApp

    from ddtrace import tracer
    from ddtrace.contrib.pylons import PylonsTraceMiddleware

    app = PylonsApp(...)

    traced_app = PylonsTraceMiddleware(app, tracer, service='my-pylons-app')

Then you can define your routes and views as usual.
"""

from ...utils.importlib import require_modules


required_modules = ['pylons.wsgiapp']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .middleware import PylonsTraceMiddleware
        from .patch import patch, unpatch

        __all__ = [
            'patch',
            'unpatch',
            'PylonsTraceMiddleware',
        ]
