"""
The aiobotocore integration will trace all AWS calls made with the ``aiobotocore``
library. This integration isn't enabled when applying the default patching.
To enable it, you must run ``patch_all(aiobotocore=True)``

::

    import aiobotocore.session
    from ddtrace import patch

    # If not patched yet, you can patch botocore specifically
    patch(aiobotocore=True)

    # This will report spans with the default instrumentation
    aiobotocore.session.get_session()
    lambda_client = session.create_client('lambda', region_name='us-east-1')

    # This query generates a trace
    lambda_client.list_functions()
"""
from ...utils.importlib import require_modules


required_modules = ['aiobotocore.client']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch

        __all__ = ['patch']
