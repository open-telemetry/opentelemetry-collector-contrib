import time

# 3rd party
from django.core.cache import caches

# testing
from .utils import DjangoTraceTestCase
from ...util import assert_dict_issuperset


class DjangoCacheRedisTest(DjangoTraceTestCase):
    """
    Ensures that the cache system is properly traced in
    different cache backend
    """
    def test_cache_redis_get(self):
        # get the redis cache
        cache = caches['redis']

        # (trace) the cache miss
        start = time.time()
        cache.get('missing_key')
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'get'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django_redis.cache.RedisCache',
            'django.cache.key': 'missing_key',
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_redis_get_many(self):
        # get the redis cache
        cache = caches['redis']

        # (trace) the cache miss
        start = time.time()
        cache.get_many(['missing_key', 'another_key'])
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'get_many'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django_redis.cache.RedisCache',
            'django.cache.key': str(['missing_key', 'another_key']),
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_pylibmc_get(self):
        # get the redis cache
        cache = caches['pylibmc']

        # (trace) the cache miss
        start = time.time()
        cache.get('missing_key')
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'get'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.memcached.PyLibMCCache',
            'django.cache.key': 'missing_key',
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_pylibmc_get_many(self):
        # get the redis cache
        cache = caches['pylibmc']

        # (trace) the cache miss
        start = time.time()
        cache.get_many(['missing_key', 'another_key'])
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'get_many'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.memcached.PyLibMCCache',
            'django.cache.key': str(['missing_key', 'another_key']),
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_memcached_get(self):
        # get the redis cache
        cache = caches['python_memcached']

        # (trace) the cache miss
        start = time.time()
        cache.get('missing_key')
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'get'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.memcached.MemcachedCache',
            'django.cache.key': 'missing_key',
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_memcached_get_many(self):
        # get the redis cache
        cache = caches['python_memcached']

        # (trace) the cache miss
        start = time.time()
        cache.get_many(['missing_key', 'another_key'])
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'get_many'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.memcached.MemcachedCache',
            'django.cache.key': str(['missing_key', 'another_key']),
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_django_pylibmc_get(self):
        # get the redis cache
        cache = caches['django_pylibmc']

        # (trace) the cache miss
        start = time.time()
        cache.get('missing_key')
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'get'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django_pylibmc.memcached.PyLibMCCache',
            'django.cache.key': 'missing_key',
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_django_pylibmc_get_many(self):
        # get the redis cache
        cache = caches['django_pylibmc']

        # (trace) the cache miss
        start = time.time()
        cache.get_many(['missing_key', 'another_key'])
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'get_many'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django_pylibmc.memcached.PyLibMCCache',
            'django.cache.key': str(['missing_key', 'another_key']),
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end
