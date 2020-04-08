"""
The Botocore integration will trace all AWS calls made with the botocore
library. Libraries like Boto3 that use Botocore will also be patched.

This integration is automatically patched when using ``patch_all()``::

    import botocore.session
    from ddtrace import patch

    # If not patched yet, you can patch botocore specifically
    patch(botocore=True)

    # This will report spans with the default instrumentation
    botocore.session.get_session()
    lambda_client = session.create_client('lambda', region_name='us-east-1')
    # Example of instrumented query
    lambda_client.list_functions()
"""


from ...utils.importlib import require_modules

required_modules = ['botocore.client']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch
        __all__ = ['patch']
