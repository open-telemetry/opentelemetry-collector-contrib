import consul
from ddtrace import Pin
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.ext import consul as consulx
from ddtrace.vendor.wrapt import BoundFunctionWrapper
from ddtrace.contrib.consul.patch import patch, unpatch

from ..config import CONSUL_CONFIG
from ...base import BaseTracerTestCase


class TestConsulPatch(BaseTracerTestCase):

    TEST_SERVICE = 'test-consul'

    def setUp(self):
        super(TestConsulPatch, self).setUp()
        patch()
        c = consul.Consul(
            host=CONSUL_CONFIG['host'],
            port=CONSUL_CONFIG['port'],
        )
        Pin.override(consul.Consul, service=self.TEST_SERVICE, tracer=self.tracer)
        Pin.override(consul.Consul.KV, service=self.TEST_SERVICE, tracer=self.tracer)
        self.c = c

    def tearDown(self):
        unpatch()
        super(TestConsulPatch, self).tearDown()

    def test_put(self):
        key = 'test/put/consul'
        value = 'test_value'

        self.c.kv.put(key, value)

        spans = self.get_spans()
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.TEST_SERVICE
        assert span.name == consulx.CMD
        assert span.resource == 'PUT'
        assert span.error == 0
        tags = {
            consulx.KEY: key,
            consulx.CMD: 'PUT',
        }
        for k, v in tags.items():
            assert span.get_tag(k) == v

    def test_get(self):
        key = 'test/get/consul'

        self.c.kv.get(key)

        spans = self.get_spans()
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.TEST_SERVICE
        assert span.name == consulx.CMD
        assert span.resource == 'GET'
        assert span.error == 0
        tags = {
            consulx.KEY: key,
            consulx.CMD: 'GET',
        }
        for k, v in tags.items():
            assert span.get_tag(k) == v

    def test_delete(self):
        key = 'test/delete/consul'

        self.c.kv.delete(key)

        spans = self.get_spans()
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.TEST_SERVICE
        assert span.name == consulx.CMD
        assert span.resource == 'DELETE'
        assert span.error == 0
        tags = {
            consulx.KEY: key,
            consulx.CMD: 'DELETE',
        }
        for k, v in tags.items():
            assert span.get_tag(k) == v

    def test_kwargs(self):
        key = 'test/kwargs/consul'
        value = 'test_value'

        self.c.kv.put(key=key, value=value)

        spans = self.get_spans()
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.TEST_SERVICE
        assert span.name == consulx.CMD
        assert span.resource == 'PUT'
        assert span.error == 0
        tags = {
            consulx.KEY: key,
        }
        for k, v in tags.items():
            assert span.get_tag(k) == v

    def test_patch_idempotence(self):
        key = 'test/patch/idempotence'

        patch()
        patch()

        self.c.kv.get(key)
        assert self.spans
        assert isinstance(self.c.kv.get, BoundFunctionWrapper)

        unpatch()
        self.reset()

        self.c.kv.get(key)
        assert not self.spans
        assert not isinstance(self.c.kv.get, BoundFunctionWrapper)

    def test_patch_preserves_functionality(self):
        key = 'test/functionality'
        value = b'test_value'

        self.c.kv.put(key, value)
        _, data = self.c.kv.get(key)
        assert data['Value'] == value
        self.c.kv.delete(key)
        _, data = self.c.kv.get(key)
        assert data is None

    def test_analytics_without_rate(self):
        with self.override_config('consul', {'analytics_enabled': True}):
            key = 'test/kwargs/consul'
            value = 'test_value'

            self.c.kv.put(key=key, value=value)

            spans = self.get_spans()
            assert len(spans) == 1
            span = spans[0]
            assert span.get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 1.0

    def test_analytics_with_rate(self):
        with self.override_config('consul', {'analytics_enabled': True, 'analytics_sample_rate': 0.5}):
            key = 'test/kwargs/consul'
            value = 'test_value'

            self.c.kv.put(key=key, value=value)

            spans = self.get_spans()
            assert len(spans) == 1
            span = spans[0]
            assert span.get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 0.5

    def test_analytics_disabled(self):
        with self.override_config('consul', {'analytics_enabled': False}):
            key = 'test/kwargs/consul'
            value = 'test_value'

            self.c.kv.put(key=key, value=value)

            spans = self.get_spans()
            assert len(spans) == 1
            span = spans[0]
            assert span.get_metric(ANALYTICS_SAMPLE_RATE_KEY) is None
