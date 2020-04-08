"""Instrument Elasticsearch to report Elasticsearch queries.

``patch_all`` will automatically patch your Elasticsearch instance to make it work.
::

    from ddtrace import Pin, patch
    from elasticsearch import Elasticsearch

    # If not patched yet, you can patch elasticsearch specifically
    patch(elasticsearch=True)

    # This will report spans with the default instrumentation
    es = Elasticsearch(port=ELASTICSEARCH_CONFIG['port'])
    # Example of instrumented query
    es.indices.create(index='books', ignore=400)

    # Use a pin to specify metadata related to this client
    es = Elasticsearch(port=ELASTICSEARCH_CONFIG['port'])
    Pin.override(es.transport, service='elasticsearch-videos')
    es.indices.create(index='videos', ignore=400)
"""
from ...utils.importlib import require_modules

# DEV: We only require one of these modules to be available
required_modules = ['elasticsearch', 'elasticsearch1', 'elasticsearch2', 'elasticsearch5', 'elasticsearch6']

with require_modules(required_modules) as missing_modules:
    # We were able to find at least one of the required modules
    if set(missing_modules) != set(required_modules):
        from .transport import get_traced_transport
        from .patch import patch

        __all__ = ['get_traced_transport', 'patch']
