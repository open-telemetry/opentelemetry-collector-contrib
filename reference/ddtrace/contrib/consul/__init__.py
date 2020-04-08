"""Instrument Consul to trace KV queries.

Only supports tracing for the syncronous client.

``patch_all`` will automatically patch your Consul client to make it work.
::

    from ddtrace import Pin, patch
    import consul

    # If not patched yet, you can patch consul specifically
    patch(consul=True)

    # This will report a span with the default settings
    client = consul.Consul(host="127.0.0.1", port=8500)
    client.get("my-key")

    # Use a pin to specify metadata related to this client
    Pin.override(client, service='consul-kv')
"""

from ...utils.importlib import require_modules

required_modules = ['consul']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch, unpatch
        __all__ = ['patch', 'unpatch']
