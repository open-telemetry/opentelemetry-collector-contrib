"""
The ``mako`` integration traces templates rendering.
Auto instrumentation is available using the ``patch``. The following is an example::

    from ddtrace import patch
    from mako.template import Template

    patch(mako=True)

    t = Template(filename="index.html")

"""
from ...utils.importlib import require_modules

required_modules = ['mako']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch, unpatch

        __all__ = [
            'patch',
            'unpatch',
        ]
