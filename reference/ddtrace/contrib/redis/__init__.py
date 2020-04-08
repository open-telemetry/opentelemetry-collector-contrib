"""Instrument redis to report Redis queries.

``patch_all`` will automatically patch your Redis client to make it work.
::

    from ddtrace import Pin, patch
    import redis

    # If not patched yet, you can patch redis specifically
    patch(redis=True)

    # This will report a span with the default settings
    client = redis.StrictRedis(host="localhost", port=6379)
    client.get("my-key")

    # Use a pin to specify metadata related to this client
    Pin.override(client, service='redis-queue')
"""

from ...utils.importlib import require_modules

required_modules = ['redis', 'redis.client']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch
        from .tracers import get_traced_redis, get_traced_redis_from

        __all__ = ['get_traced_redis', 'get_traced_redis_from', 'patch']
