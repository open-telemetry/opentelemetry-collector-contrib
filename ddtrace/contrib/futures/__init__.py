"""
The ``futures`` integration propagates the current active Tracing Context
between threads. The integration ensures that when operations are executed
in a new thread, that thread can continue the previously generated trace.

The integration doesn't trace automatically threads execution, so manual
instrumentation or another integration must be activated. Threads propagation
is not enabled by default with the `patch_all()` method and must be activated
as follows::

    from ddtrace import patch, patch_all

    patch(futures=True)
    # or, when instrumenting all libraries
    patch_all(futures=True)
"""
from ...utils.importlib import require_modules


required_modules = ['concurrent.futures']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch, unpatch

        __all__ = [
            'patch',
            'unpatch',
        ]
