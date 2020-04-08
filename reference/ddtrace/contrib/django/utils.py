from ...compat import parse
from ...internal.logger import get_logger

log = get_logger(__name__)


def _resource_from_cache_prefix(resource, cache):
    """
    Combine the resource name with the cache prefix (if any)
    """
    if getattr(cache, 'key_prefix', None):
        name = '{} {}'.format(resource, cache.key_prefix)
    else:
        name = resource

    # enforce lowercase to make the output nicer to read
    return name.lower()


def quantize_key_values(key):
    """
    Used in the Django trace operation method, it ensures that if a dict
    with values is used, we removes the values from the span meta
    attributes. For example::

        >>> quantize_key_values({'key', 'value'})
        # returns ['key']
    """
    if isinstance(key, dict):
        return key.keys()

    return key


def get_request_uri(request):
    """
    Helper to rebuild the original request url

    query string or fragments are not included.
    """
    # DEV: We do this instead of `request.build_absolute_uri()` since
    #      an exception can get raised, we want to always build a url
    #      regardless of any exceptions raised from `request.get_host()`
    host = None
    try:
        host = request.get_host()  # this will include host:port
    except Exception:
        log.debug('Failed to get Django request host', exc_info=True)

    if not host:
        try:
            # Try to build host how Django would have
            # https://github.com/django/django/blob/e8d0d2a5efc8012dcc8bf1809dec065ebde64c81/django/http/request.py#L85-L102
            if 'HTTP_HOST' in request.META:
                host = request.META['HTTP_HOST']
            else:
                host = request.META['SERVER_NAME']
                port = str(request.META['SERVER_PORT'])
                if port != ('443' if request.is_secure() else '80'):
                    host = '{0}:{1}'.format(host, port)
        except Exception:
            # This really shouldn't ever happen, but lets guard here just in case
            log.debug('Failed to build Django request host', exc_info=True)
            host = 'unknown'

    # Build request url from the information available
    # DEV: We are explicitly omitting query strings since they may contain sensitive information
    return parse.urlunparse(parse.ParseResult(
        scheme=request.scheme,
        netloc=host,
        path=request.path,
        params='',
        query='',
        fragment='',
    ))
