"""
The Algoliasearch__ integration will add tracing to your Algolia searches.

::

    from ddtrace import patch_all
    patch_all()

    from algoliasearch import algoliasearch
    client = alogliasearch.Client(<ID>, <API_KEY>)
    index = client.init_index(<INDEX_NAME>)
    index.search("your query", args={"attributesToRetrieve": "attribute1,attribute1"})

Configuration
~~~~~~~~~~~~~

.. py:data:: ddtrace.config.algoliasearch['collect_query_text']

   Whether to pass the text of your query onto Datadog. Since this may contain sensitive data it's off by default

   Default: ``False``

.. __: https://www.algolia.com
"""

from ...utils.importlib import require_modules

with require_modules(['algoliasearch', 'algoliasearch.version']) as missing_modules:
    if not missing_modules:
        from .patch import patch, unpatch

        __all__ = ['patch', 'unpatch']
