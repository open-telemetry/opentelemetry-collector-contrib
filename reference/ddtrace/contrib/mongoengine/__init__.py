"""Instrument mongoengine to report MongoDB queries.

``patch_all`` will automatically patch your mongoengine connect method to make it work.
::

    from ddtrace import Pin, patch
    import mongoengine

    # If not patched yet, you can patch mongoengine specifically
    patch(mongoengine=True)

    # At that point, mongoengine is instrumented with the default settings
    mongoengine.connect('db', alias='default')

    # Use a pin to specify metadata related to this client
    client = mongoengine.connect('db', alias='master')
    Pin.override(client, service="mongo-master")
"""

from ...utils.importlib import require_modules


required_modules = ['mongoengine']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch, trace_mongoengine

        __all__ = ['patch', 'trace_mongoengine']
