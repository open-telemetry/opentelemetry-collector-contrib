# -*- coding: utf-8 -*-

# project
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.ext import net
from ddtrace.contrib.flask_cache import get_traced_cache
from ddtrace.contrib.flask_cache.tracers import CACHE_BACKEND

# 3rd party
from flask import Flask

# testing
from tests.opentracer.utils import init_tracer
from ..config import REDIS_CONFIG, MEMCACHED_CONFIG
from ...base import BaseTracerTestCase
from ...util import assert_dict_issuperset


class FlaskCacheTest(BaseTracerTestCase):
    SERVICE = 'test-flask-cache'
    TEST_REDIS_PORT = REDIS_CONFIG['port']
    TEST_MEMCACHED_PORT = MEMCACHED_CONFIG['port']

    def setUp(self):
        super(FlaskCacheTest, self).setUp()

        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(self.tracer, service=self.SERVICE)
        app = Flask(__name__)
        self.cache = Cache(app, config={'CACHE_TYPE': 'simple'})

    def test_simple_cache_get(self):
        self.cache.get(u'á_complex_operation')
        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.service, self.SERVICE)
        self.assertEqual(span.resource, 'get')
        self.assertEqual(span.name, 'flask_cache.cmd')
        self.assertEqual(span.span_type, 'cache')
        self.assertEqual(span.error, 0)

        expected_meta = {
            'flask_cache.key': u'á_complex_operation',
            'flask_cache.backend': 'simple',
        }

        assert_dict_issuperset(span.meta, expected_meta)

    def test_simple_cache_set(self):
        self.cache.set(u'á_complex_operation', u'with_á_value\nin two lines')
        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.service, self.SERVICE)
        self.assertEqual(span.resource, 'set')
        self.assertEqual(span.name, 'flask_cache.cmd')
        self.assertEqual(span.span_type, 'cache')
        self.assertEqual(span.error, 0)

        expected_meta = {
            'flask_cache.key': u'á_complex_operation',
            'flask_cache.backend': 'simple',
        }

        assert_dict_issuperset(span.meta, expected_meta)

    def test_simple_cache_add(self):
        self.cache.add(u'á_complex_number', 50)
        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.service, self.SERVICE)
        self.assertEqual(span.resource, 'add')
        self.assertEqual(span.name, 'flask_cache.cmd')
        self.assertEqual(span.span_type, 'cache')
        self.assertEqual(span.error, 0)

        expected_meta = {
            'flask_cache.key': u'á_complex_number',
            'flask_cache.backend': 'simple',
        }

        assert_dict_issuperset(span.meta, expected_meta)

    def test_simple_cache_delete(self):
        self.cache.delete(u'á_complex_operation')
        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.service, self.SERVICE)
        self.assertEqual(span.resource, 'delete')
        self.assertEqual(span.name, 'flask_cache.cmd')
        self.assertEqual(span.span_type, 'cache')
        self.assertEqual(span.error, 0)

        expected_meta = {
            'flask_cache.key': u'á_complex_operation',
            'flask_cache.backend': 'simple',
        }

        assert_dict_issuperset(span.meta, expected_meta)

    def test_simple_cache_delete_many(self):
        self.cache.delete_many('complex_operation', 'another_complex_op')
        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.service, self.SERVICE)
        self.assertEqual(span.resource, 'delete_many')
        self.assertEqual(span.name, 'flask_cache.cmd')
        self.assertEqual(span.span_type, 'cache')
        self.assertEqual(span.error, 0)

        expected_meta = {
            'flask_cache.key': "['complex_operation', 'another_complex_op']",
            'flask_cache.backend': 'simple',
        }

        assert_dict_issuperset(span.meta, expected_meta)

    def test_simple_cache_clear(self):
        self.cache.clear()
        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.service, self.SERVICE)
        self.assertEqual(span.resource, 'clear')
        self.assertEqual(span.name, 'flask_cache.cmd')
        self.assertEqual(span.span_type, 'cache')
        self.assertEqual(span.error, 0)

        expected_meta = {
            'flask_cache.backend': 'simple',
        }

        assert_dict_issuperset(span.meta, expected_meta)

    def test_simple_cache_get_many(self):
        self.cache.get_many('first_complex_op', 'second_complex_op')
        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.service, self.SERVICE)
        self.assertEqual(span.resource, 'get_many')
        self.assertEqual(span.name, 'flask_cache.cmd')
        self.assertEqual(span.span_type, 'cache')
        self.assertEqual(span.error, 0)

        expected_meta = {
            'flask_cache.key': "['first_complex_op', 'second_complex_op']",
            'flask_cache.backend': 'simple',
        }

        assert_dict_issuperset(span.meta, expected_meta)

    def test_simple_cache_set_many(self):
        self.cache.set_many({
            'first_complex_op': 10,
            'second_complex_op': 20,
        })
        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.service, self.SERVICE)
        self.assertEqual(span.resource, 'set_many')
        self.assertEqual(span.name, 'flask_cache.cmd')
        self.assertEqual(span.span_type, 'cache')
        self.assertEqual(span.error, 0)

        self.assertEqual(span.meta['flask_cache.backend'], 'simple')
        self.assertTrue('first_complex_op' in span.meta['flask_cache.key'])
        self.assertTrue('second_complex_op' in span.meta['flask_cache.key'])

    def test_default_span_tags(self):
        # test tags and attributes
        with self.cache._TracedCache__trace('flask_cache.cmd') as span:
            self.assertEqual(span.service, self.SERVICE)
            self.assertEqual(span.span_type, 'cache')
            self.assertEqual(span.meta[CACHE_BACKEND], 'simple')
            self.assertTrue(net.TARGET_HOST not in span.meta)
            self.assertTrue(net.TARGET_PORT not in span.meta)

    def test_default_span_tags_for_redis(self):
        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(self.tracer, service=self.SERVICE)
        app = Flask(__name__)
        config = {
            'CACHE_TYPE': 'redis',
            'CACHE_REDIS_PORT': self.TEST_REDIS_PORT,
        }
        cache = Cache(app, config=config)
        # test tags and attributes
        with cache._TracedCache__trace('flask_cache.cmd') as span:
            self.assertEqual(span.service, self.SERVICE)
            self.assertEqual(span.span_type, 'cache')
            self.assertEqual(span.meta[CACHE_BACKEND], 'redis')
            self.assertEqual(span.meta[net.TARGET_HOST], 'localhost')
            self.assertEqual(span.metrics[net.TARGET_PORT], self.TEST_REDIS_PORT)

    def test_default_span_tags_memcached(self):
        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(self.tracer, service=self.SERVICE)
        app = Flask(__name__)
        config = {
            'CACHE_TYPE': 'memcached',
            'CACHE_MEMCACHED_SERVERS': ['127.0.0.1:{}'.format(self.TEST_MEMCACHED_PORT)],
        }
        cache = Cache(app, config=config)
        # test tags and attributes
        with cache._TracedCache__trace('flask_cache.cmd') as span:
            self.assertEqual(span.service, self.SERVICE)
            self.assertEqual(span.span_type, 'cache')
            self.assertEqual(span.meta[CACHE_BACKEND], 'memcached')
            self.assertEqual(span.meta[net.TARGET_HOST], '127.0.0.1')
            self.assertEqual(span.metrics[net.TARGET_PORT], self.TEST_MEMCACHED_PORT)

    def test_simple_cache_get_ot(self):
        """OpenTracing version of test_simple_cache_get."""
        ot_tracer = init_tracer('my_svc', self.tracer)

        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(self.tracer, service=self.SERVICE)
        app = Flask(__name__)
        cache = Cache(app, config={'CACHE_TYPE': 'simple'})

        with ot_tracer.start_active_span('ot_span'):
            cache.get(u'á_complex_operation')

        spans = self.get_spans()
        self.assertEqual(len(spans), 2)
        ot_span, dd_span = spans

        # confirm the parenting
        self.assertIsNone(ot_span.parent_id)
        self.assertEqual(dd_span.parent_id, ot_span.span_id)

        self.assertEqual(ot_span.resource, 'ot_span')
        self.assertEqual(ot_span.service, 'my_svc')

        self.assertEqual(dd_span.service, self.SERVICE)
        self.assertEqual(dd_span.resource, 'get')
        self.assertEqual(dd_span.name, 'flask_cache.cmd')
        self.assertEqual(dd_span.span_type, 'cache')
        self.assertEqual(dd_span.error, 0)

        expected_meta = {
            'flask_cache.key': u'á_complex_operation',
            'flask_cache.backend': 'simple',
        }

        assert_dict_issuperset(dd_span.meta, expected_meta)

    def test_analytics_default(self):
        self.cache.get(u'á_complex_operation')
        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertIsNone(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_with_rate(self):
        with self.override_config(
            'flask_cache',
            dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            self.cache.get(u'á_complex_operation')

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    def test_analytics_without_rate(self):
        with self.override_config(
            'flask_cache',
            dict(analytics_enabled=True)
        ):
            self.cache.get(u'á_complex_operation')

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)
