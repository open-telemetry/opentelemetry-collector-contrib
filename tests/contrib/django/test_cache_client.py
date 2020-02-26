import time

# 3rd party
from django.core.cache import caches

# testing
from .utils import DjangoTraceTestCase, override_ddtrace_settings
from ...util import assert_dict_issuperset


class DjangoCacheWrapperTest(DjangoTraceTestCase):
    """
    Ensures that the cache system is properly traced
    """
    def test_cache_get(self):
        # get the default cache
        cache = caches['default']

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
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': 'missing_key',
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    @override_ddtrace_settings(DEFAULT_CACHE_SERVICE='foo')
    def test_cache_service_can_be_overriden(self):
        # get the default cache
        cache = caches['default']

        # (trace) the cache miss
        cache.get('missing_key')

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'foo'

    @override_ddtrace_settings(INSTRUMENT_CACHE=False)
    def test_cache_disabled(self):
        # get the default cache
        cache = caches['default']

        # (trace) the cache miss
        cache.get('missing_key')

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 0

    def test_cache_set(self):
        # get the default cache
        cache = caches['default']

        # (trace) the cache miss
        start = time.time()
        cache.set('a_new_key', 50)
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'set'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': 'a_new_key',
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_add(self):
        # get the default cache
        cache = caches['default']

        # (trace) the cache miss
        start = time.time()
        cache.add('a_new_key', 50)
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'add'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': 'a_new_key',
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_delete(self):
        # get the default cache
        cache = caches['default']

        # (trace) the cache miss
        start = time.time()
        cache.delete('an_existing_key')
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == 'django'
        assert span.resource == 'delete'
        assert span.name == 'django.cache'
        assert span.span_type == 'cache'
        assert span.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': 'an_existing_key',
            'env': 'test',
        }

        assert_dict_issuperset(span.meta, expected_meta)
        assert start < span.start < span.start + span.duration < end

    def test_cache_incr(self):
        # get the default cache, set the value and reset the spans
        cache = caches['default']
        cache.set('value', 0)
        self.tracer.writer.spans = []

        # (trace) the cache miss
        start = time.time()
        cache.incr('value')
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 2

        span_incr = spans[0]
        span_get = spans[1]

        # LocMemCache doesn't provide an atomic operation
        assert span_get.service == 'django'
        assert span_get.resource == 'get'
        assert span_get.name == 'django.cache'
        assert span_get.span_type == 'cache'
        assert span_get.error == 0
        assert span_incr.service == 'django'
        assert span_incr.resource == 'incr'
        assert span_incr.name == 'django.cache'
        assert span_incr.span_type == 'cache'
        assert span_incr.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': 'value',
            'env': 'test',
        }

        assert_dict_issuperset(span_get.meta, expected_meta)
        assert_dict_issuperset(span_incr.meta, expected_meta)
        assert start < span_incr.start < span_incr.start + span_incr.duration < end

    def test_cache_decr(self):
        # get the default cache, set the value and reset the spans
        cache = caches['default']
        cache.set('value', 0)
        self.tracer.writer.spans = []

        # (trace) the cache miss
        start = time.time()
        cache.decr('value')
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 3

        span_decr = spans[0]
        span_incr = spans[1]
        span_get = spans[2]

        # LocMemCache doesn't provide an atomic operation
        assert span_get.service == 'django'
        assert span_get.resource == 'get'
        assert span_get.name == 'django.cache'
        assert span_get.span_type == 'cache'
        assert span_get.error == 0
        assert span_incr.service == 'django'
        assert span_incr.resource == 'incr'
        assert span_incr.name == 'django.cache'
        assert span_incr.span_type == 'cache'
        assert span_incr.error == 0
        assert span_decr.service == 'django'
        assert span_decr.resource == 'decr'
        assert span_decr.name == 'django.cache'
        assert span_decr.span_type == 'cache'
        assert span_decr.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': 'value',
            'env': 'test',
        }

        assert_dict_issuperset(span_get.meta, expected_meta)
        assert_dict_issuperset(span_incr.meta, expected_meta)
        assert_dict_issuperset(span_decr.meta, expected_meta)
        assert start < span_decr.start < span_decr.start + span_decr.duration < end

    def test_cache_get_many(self):
        # get the default cache
        cache = caches['default']

        # (trace) the cache miss
        start = time.time()
        cache.get_many(['missing_key', 'another_key'])
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 3

        span_get_many = spans[0]
        span_get_first = spans[1]
        span_get_second = spans[2]

        # LocMemCache doesn't provide an atomic operation
        assert span_get_first.service == 'django'
        assert span_get_first.resource == 'get'
        assert span_get_first.name == 'django.cache'
        assert span_get_first.span_type == 'cache'
        assert span_get_first.error == 0
        assert span_get_second.service == 'django'
        assert span_get_second.resource == 'get'
        assert span_get_second.name == 'django.cache'
        assert span_get_second.span_type == 'cache'
        assert span_get_second.error == 0
        assert span_get_many.service == 'django'
        assert span_get_many.resource == 'get_many'
        assert span_get_many.name == 'django.cache'
        assert span_get_many.span_type == 'cache'
        assert span_get_many.error == 0

        expected_meta = {
            'django.cache.backend': 'django.core.cache.backends.locmem.LocMemCache',
            'django.cache.key': str(['missing_key', 'another_key']),
            'env': 'test',
        }

        assert_dict_issuperset(span_get_many.meta, expected_meta)
        assert start < span_get_many.start < span_get_many.start + span_get_many.duration < end

    def test_cache_set_many(self):
        # get the default cache
        cache = caches['default']

        # (trace) the cache miss
        start = time.time()
        cache.set_many({'first_key': 1, 'second_key': 2})
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 3

        span_set_many = spans[0]
        span_set_first = spans[1]
        span_set_second = spans[2]

        # LocMemCache doesn't provide an atomic operation
        assert span_set_first.service == 'django'
        assert span_set_first.resource == 'set'
        assert span_set_first.name == 'django.cache'
        assert span_set_first.span_type == 'cache'
        assert span_set_first.error == 0
        assert span_set_second.service == 'django'
        assert span_set_second.resource == 'set'
        assert span_set_second.name == 'django.cache'
        assert span_set_second.span_type == 'cache'
        assert span_set_second.error == 0
        assert span_set_many.service == 'django'
        assert span_set_many.resource == 'set_many'
        assert span_set_many.name == 'django.cache'
        assert span_set_many.span_type == 'cache'
        assert span_set_many.error == 0

        assert span_set_many.meta['django.cache.backend'] == 'django.core.cache.backends.locmem.LocMemCache'
        assert 'first_key' in span_set_many.meta['django.cache.key']
        assert 'second_key' in span_set_many.meta['django.cache.key']
        assert start < span_set_many.start < span_set_many.start + span_set_many.duration < end

    def test_cache_delete_many(self):
        # get the default cache
        cache = caches['default']

        # (trace) the cache miss
        start = time.time()
        cache.delete_many(['missing_key', 'another_key'])
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 3

        span_delete_many = spans[0]
        span_delete_first = spans[1]
        span_delete_second = spans[2]

        # LocMemCache doesn't provide an atomic operation
        assert span_delete_first.service == 'django'
        assert span_delete_first.resource == 'delete'
        assert span_delete_first.name == 'django.cache'
        assert span_delete_first.span_type == 'cache'
        assert span_delete_first.error == 0
        assert span_delete_second.service == 'django'
        assert span_delete_second.resource == 'delete'
        assert span_delete_second.name == 'django.cache'
        assert span_delete_second.span_type == 'cache'
        assert span_delete_second.error == 0
        assert span_delete_many.service == 'django'
        assert span_delete_many.resource == 'delete_many'
        assert span_delete_many.name == 'django.cache'
        assert span_delete_many.span_type == 'cache'
        assert span_delete_many.error == 0

        assert span_delete_many.meta['django.cache.backend'] == 'django.core.cache.backends.locmem.LocMemCache'
        assert 'missing_key' in span_delete_many.meta['django.cache.key']
        assert 'another_key' in span_delete_many.meta['django.cache.key']
        assert start < span_delete_many.start < span_delete_many.start + span_delete_many.duration < end
