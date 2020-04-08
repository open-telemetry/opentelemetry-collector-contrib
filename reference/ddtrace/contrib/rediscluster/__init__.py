"""Instrument rediscluster to report Redis Cluster queries.

``patch_all`` will automatically patch your Redis Cluster client to make it work.
::

    from ddtrace import Pin, patch
    import rediscluster

    # If not patched yet, you can patch redis specifically
    patch(rediscluster=True)

    # This will report a span with the default settings
    client = rediscluster.StrictRedisCluster(startup_nodes=[{'host':'localhost', 'port':'7000'}])
    client.get('my-key')

    # Use a pin to specify metadata related to this client
    Pin.override(client, service='redis-queue')
"""

from ...utils.importlib import require_modules

required_modules = ['rediscluster', 'rediscluster.client']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch

        __all__ = ['patch']
