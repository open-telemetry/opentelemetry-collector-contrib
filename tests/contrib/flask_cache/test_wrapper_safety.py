# -*- coding: utf-8 -*-
import unittest

# project
from ddtrace.ext import net
from ddtrace.tracer import Tracer
from ddtrace.contrib.flask_cache import get_traced_cache
from ddtrace.contrib.flask_cache.tracers import CACHE_BACKEND

# 3rd party
from flask import Flask
from redis.exceptions import ConnectionError
import pytest

# testing
from ...test_tracer import DummyWriter


class FlaskCacheWrapperTest(unittest.TestCase):
    SERVICE = 'test-flask-cache'

    def test_cache_get_without_arguments(self):
        # initialize the dummy writer
        writer = DummyWriter()
        tracer = Tracer()
        tracer.writer = writer

        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(tracer, service=self.SERVICE)
        app = Flask(__name__)
        cache = Cache(app, config={'CACHE_TYPE': 'simple'})

        # make a wrong call
        with pytest.raises(TypeError) as ex:
            cache.get()

        # ensure that the error is not caused by our tracer
        assert 'get()' in ex.value.args[0]
        assert 'argument' in ex.value.args[0]
        spans = writer.pop()
        # an error trace must be sent
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.SERVICE
        assert span.resource == 'get'
        assert span.name == 'flask_cache.cmd'
        assert span.span_type == 'cache'
        assert span.error == 1

    def test_cache_set_without_arguments(self):
        # initialize the dummy writer
        writer = DummyWriter()
        tracer = Tracer()
        tracer.writer = writer

        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(tracer, service=self.SERVICE)
        app = Flask(__name__)
        cache = Cache(app, config={'CACHE_TYPE': 'simple'})

        # make a wrong call
        with pytest.raises(TypeError) as ex:
            cache.set()

        # ensure that the error is not caused by our tracer
        assert 'set()' in ex.value.args[0]
        assert 'argument' in ex.value.args[0]
        spans = writer.pop()
        # an error trace must be sent
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.SERVICE
        assert span.resource == 'set'
        assert span.name == 'flask_cache.cmd'
        assert span.span_type == 'cache'
        assert span.error == 1

    def test_cache_add_without_arguments(self):
        # initialize the dummy writer
        writer = DummyWriter()
        tracer = Tracer()
        tracer.writer = writer

        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(tracer, service=self.SERVICE)
        app = Flask(__name__)
        cache = Cache(app, config={'CACHE_TYPE': 'simple'})

        # make a wrong call
        with pytest.raises(TypeError) as ex:
            cache.add()

        # ensure that the error is not caused by our tracer
        assert 'add()' in ex.value.args[0]
        assert 'argument' in ex.value.args[0]
        spans = writer.pop()
        # an error trace must be sent
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.SERVICE
        assert span.resource == 'add'
        assert span.name == 'flask_cache.cmd'
        assert span.span_type == 'cache'
        assert span.error == 1

    def test_cache_delete_without_arguments(self):
        # initialize the dummy writer
        writer = DummyWriter()
        tracer = Tracer()
        tracer.writer = writer

        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(tracer, service=self.SERVICE)
        app = Flask(__name__)
        cache = Cache(app, config={'CACHE_TYPE': 'simple'})

        # make a wrong call
        with pytest.raises(TypeError) as ex:
            cache.delete()

        # ensure that the error is not caused by our tracer
        assert 'delete()' in ex.value.args[0]
        assert 'argument' in ex.value.args[0]
        spans = writer.pop()
        # an error trace must be sent
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.SERVICE
        assert span.resource == 'delete'
        assert span.name == 'flask_cache.cmd'
        assert span.span_type == 'cache'
        assert span.error == 1

    def test_cache_set_many_without_arguments(self):
        # initialize the dummy writer
        writer = DummyWriter()
        tracer = Tracer()
        tracer.writer = writer

        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(tracer, service=self.SERVICE)
        app = Flask(__name__)
        cache = Cache(app, config={'CACHE_TYPE': 'simple'})

        # make a wrong call
        with pytest.raises(TypeError) as ex:
            cache.set_many()

        # ensure that the error is not caused by our tracer
        assert 'set_many()' in ex.value.args[0]
        assert 'argument' in ex.value.args[0]
        spans = writer.pop()
        # an error trace must be sent
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.SERVICE
        assert span.resource == 'set_many'
        assert span.name == 'flask_cache.cmd'
        assert span.span_type == 'cache'
        assert span.error == 1

    def test_redis_cache_tracing_with_a_wrong_connection(self):
        # initialize the dummy writer
        writer = DummyWriter()
        tracer = Tracer()
        tracer.writer = writer

        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(tracer, service=self.SERVICE)
        app = Flask(__name__)
        config = {
            'CACHE_TYPE': 'redis',
            'CACHE_REDIS_PORT': 2230,
            'CACHE_REDIS_HOST': '127.0.0.1'
        }
        cache = Cache(app, config=config)

        # use a wrong redis connection
        with pytest.raises(ConnectionError) as ex:
            cache.get(u'á_complex_operation')

        # ensure that the error is not caused by our tracer
        assert '127.0.0.1:2230. Connection refused.' in ex.value.args[0]
        spans = writer.pop()
        # an error trace must be sent
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.SERVICE
        assert span.resource == 'get'
        assert span.name == 'flask_cache.cmd'
        assert span.span_type == 'cache'
        assert span.meta[CACHE_BACKEND] == 'redis'
        assert span.meta[net.TARGET_HOST] == '127.0.0.1'
        assert span.metrics[net.TARGET_PORT] == 2230
        assert span.error == 1

    def test_memcached_cache_tracing_with_a_wrong_connection(self):
        # initialize the dummy writer
        writer = DummyWriter()
        tracer = Tracer()
        tracer.writer = writer

        # create the TracedCache instance for a Flask app
        Cache = get_traced_cache(tracer, service=self.SERVICE)
        app = Flask(__name__)
        config = {
            'CACHE_TYPE': 'memcached',
            'CACHE_MEMCACHED_SERVERS': ['localhost:2230'],
        }
        cache = Cache(app, config=config)

        # use a wrong memcached connection
        try:
            cache.get(u'á_complex_operation')
        except Exception:
            pass

        # ensure that the error is not caused by our tracer
        spans = writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.SERVICE
        assert span.resource == 'get'
        assert span.name == 'flask_cache.cmd'
        assert span.span_type == 'cache'
        assert span.meta[CACHE_BACKEND] == 'memcached'
        assert span.meta[net.TARGET_HOST] == 'localhost'
        assert span.metrics[net.TARGET_PORT] == 2230

        # the pylibmc backend raises an exception and memcached backend does
        # not, so don't test anything about the status.
