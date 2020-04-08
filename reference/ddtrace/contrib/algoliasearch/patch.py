from ddtrace.pin import Pin
from ddtrace.settings import config
from ddtrace.utils.wrappers import unwrap as _u
from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

DD_PATCH_ATTR = '_datadog_patch'

SERVICE_NAME = 'algoliasearch'
APP_NAME = 'algoliasearch'

try:
    import algoliasearch
    from algoliasearch.version import VERSION
    algoliasearch_version = tuple([int(i) for i in VERSION.split('.')])

    # Default configuration
    config._add('algoliasearch', dict(
        service_name=SERVICE_NAME,
        collect_query_text=False
    ))
except ImportError:
    algoliasearch_version = (0, 0)


def patch():
    if algoliasearch_version == (0, 0):
        return

    if getattr(algoliasearch, DD_PATCH_ATTR, False):
        return

    setattr(algoliasearch, '_datadog_patch', True)

    pin = Pin(
        service=config.algoliasearch.service_name, app=APP_NAME
    )

    if algoliasearch_version < (2, 0) and algoliasearch_version >= (1, 0):
        _w(algoliasearch.index, 'Index.search', _patched_search)
        pin.onto(algoliasearch.index.Index)
    elif algoliasearch_version >= (2, 0) and algoliasearch_version < (3, 0):
        from algoliasearch import search_index
        _w(algoliasearch, 'search_index.SearchIndex.search', _patched_search)
        pin.onto(search_index.SearchIndex)
    else:
        return


def unpatch():
    if algoliasearch_version == (0, 0):
        return

    if getattr(algoliasearch, DD_PATCH_ATTR, False):
        setattr(algoliasearch, DD_PATCH_ATTR, False)

    if algoliasearch_version < (2, 0) and algoliasearch_version >= (1, 0):
        _u(algoliasearch.index.Index, 'search')
    elif algoliasearch_version >= (2, 0) and algoliasearch_version < (3, 0):
        from algoliasearch import search_index
        _u(search_index.SearchIndex, 'search')
    else:
        return


# DEV: this maps serves the dual purpose of enumerating the algoliasearch.search() query_args that
# will be sent along as tags, as well as converting arguments names into tag names compliant with
# tag naming recommendations set out here: https://docs.datadoghq.com/tagging/
QUERY_ARGS_DD_TAG_MAP = {
    'page': 'page',
    'hitsPerPage': 'hits_per_page',
    'attributesToRetrieve': 'attributes_to_retrieve',
    'attributesToHighlight': 'attributes_to_highlight',
    'attributesToSnippet': 'attributes_to_snippet',
    'minWordSizefor1Typo': 'min_word_size_for_1_typo',
    'minWordSizefor2Typos': 'min_word_size_for_2_typos',
    'getRankingInfo': 'get_ranking_info',
    'aroundLatLng': 'around_lat_lng',
    'numericFilters': 'numeric_filters',
    'tagFilters': 'tag_filters',
    'queryType': 'query_type',
    'optionalWords': 'optional_words',
    'distinct': 'distinct'
}


def _patched_search(func, instance, wrapt_args, wrapt_kwargs):
    """
        wrapt_args is called the way it is to distinguish it from the 'args'
        argument to the algoliasearch.index.Index.search() method.
    """

    if algoliasearch_version < (2, 0) and algoliasearch_version >= (1, 0):
        function_query_arg_name = 'args'
    elif algoliasearch_version >= (2, 0) and algoliasearch_version < (3, 0):
        function_query_arg_name = 'request_options'
    else:
        return func(*wrapt_args, **wrapt_kwargs)

    pin = Pin.get_from(instance)
    if not pin or not pin.enabled():
        return func(*wrapt_args, **wrapt_kwargs)

    with pin.tracer.trace('algoliasearch.search', service=pin.service) as span:
        if not span.sampled:
            return func(*wrapt_args, **wrapt_kwargs)

        if config.algoliasearch.collect_query_text:
            span.set_tag('query.text', wrapt_kwargs.get('query', wrapt_args[0]))

        query_args = wrapt_kwargs.get(function_query_arg_name, wrapt_args[1] if len(wrapt_args) > 1 else None)

        if query_args and isinstance(query_args, dict):
            for query_arg, tag_name in QUERY_ARGS_DD_TAG_MAP.items():
                value = query_args.get(query_arg)
                if value is not None:
                    span.set_tag('query.args.{}'.format(tag_name), value)

        # Result would look like this
        # {
        #   'hits': [
        #     {
        #       .... your search results ...
        #     }
        #   ],
        #   'processingTimeMS': 1,
        #   'nbHits': 1,
        #   'hitsPerPage': 20,
        #   'exhaustiveNbHits': true,
        #   'params': 'query=xxx',
        #   'nbPages': 1,
        #   'query': 'xxx',
        #   'page': 0
        # }
        result = func(*wrapt_args, **wrapt_kwargs)

        if isinstance(result, dict):
            if result.get('processingTimeMS', None) is not None:
                span.set_metric('processing_time_ms', int(result['processingTimeMS']))

            if result.get('nbHits', None) is not None:
                span.set_metric('number_of_hits', int(result['nbHits']))

        return result
