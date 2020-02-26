"""
To trace a request in a ``gevent`` environment, configure the tracer to use the greenlet
context provider, rather than the default one that relies on a thread-local storaging.

This allows the tracer to pick up a transaction exactly where it left off as greenlets
yield the context to another one.

The simplest way to trace a ``gevent`` application is to configure the tracer and
patch ``gevent`` **before importing** the library::

    # patch before importing gevent
    from ddtrace import patch, tracer
    patch(gevent=True)

    # use gevent as usual with or without the monkey module
    from gevent import monkey; monkey.patch_thread()

    def my_parent_function():
        with tracer.trace("web.request") as span:
            span.service = "web"
            gevent.spawn(worker_function)

    def worker_function():
        # then trace its child
        with tracer.trace("greenlet.call") as span:
            span.service = "greenlet"
            ...

            with tracer.trace("greenlet.child_call") as child:
                ...
"""
from ...utils.importlib import require_modules


required_modules = ['gevent']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .provider import GeventContextProvider
        from .patch import patch, unpatch

        context_provider = GeventContextProvider()

        __all__ = [
            'patch',
            'unpatch',
            'context_provider',
        ]
