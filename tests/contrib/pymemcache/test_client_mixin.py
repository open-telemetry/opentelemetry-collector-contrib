# 3p
import unittest
import pymemcache

# project
from ddtrace import Pin
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.pymemcache.patch import patch, unpatch
from ddtrace.ext import memcached as memcachedx, net
from .utils import MockSocket

from tests.test_tracer import get_dummy_tracer
from ...base import override_config


_Client = pymemcache.client.base.Client

TEST_HOST = 'localhost'
TEST_PORT = 117711


class PymemcacheClientTestCaseMixin(unittest.TestCase):
    """ Tests for a patched pymemcache.client.base.Client. """

    def get_spans(self):
        pin = Pin.get_from(self.client)
        tracer = pin.tracer
        spans = tracer.writer.pop()
        return spans

    def check_spans(self, num_expected, resources_expected, queries_expected):
        """A helper for validating basic span information."""
        spans = self.get_spans()
        self.assertEqual(num_expected, len(spans))

        for span, resource, query in zip(spans, resources_expected, queries_expected):
            self.assertEqual(span.get_tag(net.TARGET_HOST), TEST_HOST)
            self.assertEqual(span.get_metric(net.TARGET_PORT), TEST_PORT)
            self.assertEqual(span.name, memcachedx.CMD)
            self.assertEqual(span.span_type, 'cache')
            self.assertEqual(span.service, memcachedx.SERVICE)
            self.assertEqual(span.get_tag(memcachedx.QUERY), query)
            self.assertEqual(span.resource, resource)

        return spans

    def setUp(self):
        patch()

    def tearDown(self):
        unpatch()

    def make_client(self, mock_socket_values, **kwargs):
        tracer = get_dummy_tracer()
        Pin.override(pymemcache, tracer=tracer)
        self.client = pymemcache.client.base.Client((TEST_HOST, TEST_PORT), **kwargs)
        self.client.sock = MockSocket(list(mock_socket_values))
        return self.client

    def test_set_success(self):
        client = self.make_client([b'STORED\r\n'])
        result = client.set(b'key', b'value', noreply=False)
        assert result is True

        self.check_spans(1, ['set'], ['set key'])

    def test_get_many_none_found(self):
        client = self.make_client([b'END\r\n'])
        result = client.get_many([b'key1', b'key2'])
        assert result == {}

        self.check_spans(1, ['get_many'], ['get_many key1 key2'])

    def test_get_multi_none_found(self):
        client = self.make_client([b'END\r\n'])
        result = client.get_multi([b'key1', b'key2'])
        assert result == {}

        self.check_spans(1, ['get_many'], ['get_many key1 key2'])

    def test_delete_not_found(self):
        client = self.make_client([b'NOT_FOUND\r\n'])
        result = client.delete(b'key', noreply=False)
        assert result is False

        self.check_spans(1, ['delete'], ['delete key'])

    def test_incr_found(self):
        client = self.make_client([b'STORED\r\n', b'1\r\n'])
        client.set(b'key', 0, noreply=False)
        result = client.incr(b'key', 1, noreply=False)
        assert result == 1

        self.check_spans(2, ['set', 'incr'], ['set key', 'incr key'])

    def test_get_found(self):
        client = self.make_client([b'STORED\r\n', b'VALUE key 0 5\r\nvalue\r\nEND\r\n'])
        result = client.set(b'key', b'value', noreply=False)
        result = client.get(b'key')
        assert result == b'value'

        self.check_spans(2, ['set', 'get'], ['set key', 'get key'])

    def test_decr_found(self):
        client = self.make_client([b'STORED\r\n', b'1\r\n'])
        client.set(b'key', 2, noreply=False)
        result = client.decr(b'key', 1, noreply=False)
        assert result == 1

        self.check_spans(2, ['set', 'decr'], ['set key', 'decr key'])

    def test_add_stored(self):
        client = self.make_client([b'STORED\r', b'\n'])
        result = client.add(b'key', b'value', noreply=False)
        assert result is True

        self.check_spans(1, ['add'], ['add key'])

    def test_delete_many_found(self):
        client = self.make_client([b'STORED\r', b'\n', b'DELETED\r\n'])
        result = client.add(b'key', b'value', noreply=False)
        result = client.delete_many([b'key'], noreply=False)
        assert result is True

        self.check_spans(2, ['add', 'delete_many'], ['add key', 'delete_many key'])

    def test_set_many_success(self):
        client = self.make_client([b'STORED\r\n'])
        result = client.set_many({b'key': b'value'}, noreply=False)
        assert result is True

        self.check_spans(1, ['set_many'], ['set_many key'])

    def test_set_multi_success(self):
        # Should just map to set_many
        client = self.make_client([b'STORED\r\n'])
        result = client.set_multi({b'key': b'value'}, noreply=False)
        assert result is True

        self.check_spans(1, ['set_many'], ['set_many key'])

    def test_analytics_default(self):
        client = self.make_client([b'STORED\r\n'])
        result = client.set(b'key', b'value', noreply=False)
        assert result is True

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertIsNone(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_with_rate(self):
        with override_config(
            'pymemcache',
            dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            client = self.make_client([b'STORED\r\n'])
            result = client.set(b'key', b'value', noreply=False)
            assert result is True

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    def test_analytics_without_rate(self):
        with override_config(
            'pymemcache',
            dict(analytics_enabled=True)
        ):
            client = self.make_client([b'STORED\r\n'])
            result = client.set(b'key', b'value', noreply=False)
            assert result is True

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)
