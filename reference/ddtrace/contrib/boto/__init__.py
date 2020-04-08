"""
Boto integration will trace all AWS calls made via boto2.
This integration is automatically patched when using ``patch_all()``::

    import boto.ec2
    from ddtrace import patch

    # If not patched yet, you can patch boto specifically
    patch(boto=True)

    # This will report spans with the default instrumentation
    ec2 = boto.ec2.connect_to_region("us-west-2")
    # Example of instrumented query
    ec2.get_all_instances()
"""

from ...utils.importlib import require_modules

required_modules = ['boto.connection']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch
        __all__ = ['patch']
