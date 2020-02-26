from functools import wraps

from django.conf import settings as django_settings

from ...ext import SpanTypes
from ...internal.logger import get_logger
from .conf import settings, import_from_string
from .utils import quantize_key_values, _resource_from_cache_prefix


log = get_logger(__name__)

# code instrumentation
DATADOG_NAMESPACE = '__datadog_original_{method}'
TRACED_METHODS = [
    'get',
    'set',
    'add',
    'delete',
    'incr',
    'decr',
    'get_many',
    'set_many',
    'delete_many',
]

# standard tags
CACHE_BACKEND = 'django.cache.backend'
CACHE_COMMAND_KEY = 'django.cache.key'


def patch_cache(tracer):
    """
    Function that patches the inner cache system. Because the cache backend
    can have different implementations and connectors, this function must
    handle all possible interactions with the Django cache. What follows
    is currently traced:

    * in-memory cache
    * the cache client wrapper that could use any of the common
      Django supported cache servers (Redis, Memcached, Database, Custom)
    """
    # discover used cache backends
    cache_backends = set([cache['BACKEND'] for cache in django_settings.CACHES.values()])

    def _trace_operation(fn, method_name):
        """
        Return a wrapped function that traces a cache operation
        """
        cache_service_name = settings.DEFAULT_CACHE_SERVICE \
            if settings.DEFAULT_CACHE_SERVICE else settings.DEFAULT_SERVICE

        @wraps(fn)
        def wrapped(self, *args, **kwargs):
            # get the original function method
            method = getattr(self, DATADOG_NAMESPACE.format(method=method_name))
            with tracer.trace('django.cache', span_type=SpanTypes.CACHE, service=cache_service_name) as span:
                # update the resource name and tag the cache backend
                span.resource = _resource_from_cache_prefix(method_name, self)
                cache_backend = '{}.{}'.format(self.__module__, self.__class__.__name__)
                span.set_tag(CACHE_BACKEND, cache_backend)

                if args:
                    keys = quantize_key_values(args[0])
                    span.set_tag(CACHE_COMMAND_KEY, keys)

                return method(*args, **kwargs)
        return wrapped

    def _wrap_method(cls, method_name):
        """
        For the given class, wraps the method name with a traced operation
        so that the original method is executed, while the span is properly
        created
        """
        # check if the backend owns the given bounded method
        if not hasattr(cls, method_name):
            return

        # prevent patching each backend's method more than once
        if hasattr(cls, DATADOG_NAMESPACE.format(method=method_name)):
            log.debug('%s already traced', method_name)
        else:
            method = getattr(cls, method_name)
            setattr(cls, DATADOG_NAMESPACE.format(method=method_name), method)
            setattr(cls, method_name, _trace_operation(method, method_name))

    # trace all backends
    for cache_module in cache_backends:
        cache = import_from_string(cache_module, cache_module)

        for method in TRACED_METHODS:
            _wrap_method(cache, method)


def unpatch_method(cls, method_name):
    method = getattr(cls, DATADOG_NAMESPACE.format(method=method_name), None)
    if method is None:
        log.debug('nothing to do, the class is not patched')
        return
    setattr(cls, method_name, method)
    delattr(cls, DATADOG_NAMESPACE.format(method=method_name))


def unpatch_cache():
    cache_backends = set([cache['BACKEND'] for cache in django_settings.CACHES.values()])
    for cache_module in cache_backends:
        cache = import_from_string(cache_module, cache_module)

        for method in TRACED_METHODS:
            unpatch_method(cache, method)
