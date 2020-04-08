# stdlib
import time
from unittest.case import SkipTest

# 3p
import pylibmc

# project
from ddtrace import Pin
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.pylibmc import TracedClient
from ddtrace.contrib.pylibmc.patch import patch, unpatch
from ddtrace.ext import memcached

# testing
from ...opentracer.utils import init_tracer
from ...contrib.config import MEMCACHED_CONFIG as cfg
from ...base import BaseTracerTestCase


class PylibmcCore(object):
    """Core of the test suite for pylibmc

    Shared tests between the patch and TracedClient interface.
    Will be merge back to a single class once the TracedClient is deprecated.
    """

    TEST_SERVICE = memcached.SERVICE

    def get_client(self):
        # Implement me
        pass

    def test_upgrade(self):
        raise SkipTest('upgrade memcached')
        # add tests for touch, cas, gets etc

    def test_append_prepend(self):
        client, tracer = self.get_client()
        # test
        start = time.time()
        client.set('a', 'crow')
        client.prepend('a', 'holy ')
        client.append('a', '!')

        # FIXME[matt] there is a bug in pylibmc & python 3 (perhaps with just
        # some versions of the libmemcache?) where append/prepend are replaced
        # with get. our traced versions do the right thing, so skipping this
        # test.
        try:
            assert client.get('a') == 'holy crow!'
        except AssertionError:
            pass

        end = time.time()
        # verify spans
        spans = tracer.writer.pop()
        for s in spans:
            self._verify_cache_span(s, start, end)
        expected_resources = sorted(['append', 'prepend', 'get', 'set'])
        resources = sorted(s.resource for s in spans)
        assert expected_resources == resources

    def test_incr_decr(self):
        client, tracer = self.get_client()
        # test
        start = time.time()
        client.set('a', 1)
        client.incr('a', 2)
        client.decr('a', 1)
        v = client.get('a')
        assert v == 2
        end = time.time()
        # verify spans
        spans = tracer.writer.pop()
        for s in spans:
            self._verify_cache_span(s, start, end)
        expected_resources = sorted(['get', 'set', 'incr', 'decr'])
        resources = sorted(s.resource for s in spans)
        assert expected_resources == resources

    def test_incr_decr_ot(self):
        """OpenTracing version of test_incr_decr."""
        client, tracer = self.get_client()
        ot_tracer = init_tracer('memcached', tracer)

        start = time.time()
        with ot_tracer.start_active_span('mc_ops'):
            client.set('a', 1)
            client.incr('a', 2)
            client.decr('a', 1)
            v = client.get('a')
            assert v == 2
        end = time.time()

        # verify spans
        spans = tracer.writer.pop()
        ot_span = spans[0]

        assert ot_span.name == 'mc_ops'

        for s in spans[1:]:
            assert s.parent_id == ot_span.span_id
            self._verify_cache_span(s, start, end)
        expected_resources = sorted(['get', 'set', 'incr', 'decr'])
        resources = sorted(s.resource for s in spans[1:])
        assert expected_resources == resources

    def test_clone(self):
        # ensure cloned connections are traced as well.
        client, tracer = self.get_client()
        cloned = client.clone()
        start = time.time()
        cloned.get('a')
        end = time.time()
        spans = tracer.writer.pop()
        for s in spans:
            self._verify_cache_span(s, start, end)
        expected_resources = ['get']
        resources = sorted(s.resource for s in spans)
        assert expected_resources == resources

    def test_get_set_multi(self):
        client, tracer = self.get_client()
        # test
        start = time.time()
        client.set_multi({'a': 1, 'b': 2})
        out = client.get_multi(['a', 'c'])
        assert out == {'a': 1}
        client.delete_multi(['a', 'c'])
        end = time.time()
        # verify
        spans = tracer.writer.pop()
        for s in spans:
            self._verify_cache_span(s, start, end)
        expected_resources = sorted(['get_multi', 'set_multi', 'delete_multi'])
        resources = sorted(s.resource for s in spans)
        assert expected_resources == resources

    def test_get_set_multi_prefix(self):
        client, tracer = self.get_client()
        # test
        start = time.time()
        client.set_multi({'a': 1, 'b': 2}, key_prefix='foo')
        out = client.get_multi(['a', 'c'], key_prefix='foo')
        assert out == {'a': 1}
        client.delete_multi(['a', 'c'], key_prefix='foo')
        end = time.time()
        # verify
        spans = tracer.writer.pop()
        for s in spans:
            self._verify_cache_span(s, start, end)
            assert s.get_tag('memcached.query') == '%s foo' % s.resource
        expected_resources = sorted(['get_multi', 'set_multi', 'delete_multi'])
        resources = sorted(s.resource for s in spans)
        assert expected_resources == resources

    def test_get_set_delete(self):
        client, tracer = self.get_client()
        # test
        k = u'cafe'
        v = 'val-foo'
        start = time.time()
        client.delete(k)  # just in case
        out = client.get(k)
        assert out is None, out
        client.set(k, v)
        out = client.get(k)
        assert out == v
        end = time.time()
        # verify
        spans = tracer.writer.pop()
        for s in spans:
            self._verify_cache_span(s, start, end)
            assert s.get_tag('memcached.query') == '%s %s' % (s.resource, k)
        expected_resources = sorted(['get', 'get', 'delete', 'set'])
        resources = sorted(s.resource for s in spans)
        assert expected_resources == resources

    def _verify_cache_span(self, s, start, end):
        assert s.start > start
        assert s.start + s.duration < end
        assert s.service == self.TEST_SERVICE
        assert s.span_type == 'cache'
        assert s.name == 'memcached.cmd'
        assert s.get_tag('out.host') == cfg['host']
        assert s.get_metric('out.port') == cfg['port']

    def test_analytics_default(self):
        client, tracer = self.get_client()
        client.set('a', 'crow')

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertIsNone(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_with_rate(self):
        with self.override_config(
            'pylibmc',
            dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            client, tracer = self.get_client()
            client.set('a', 'crow')

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    def test_analytics_without_rate(self):
        with self.override_config(
            'pylibmc',
            dict(analytics_enabled=True)
        ):
            client, tracer = self.get_client()
            client.set('a', 'crow')

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)

    def test_disabled(self):
        """
        Ensure client works when the tracer is disabled
        """
        client, tracer = self.get_client()
        try:
            tracer.enabled = False

            client.set('a', 'crow')

            spans = self.get_spans()
            assert len(spans) == 0
        finally:
            tracer.enabled = True


class TestPylibmcLegacy(BaseTracerTestCase, PylibmcCore):
    """Test suite for the tracing of pylibmc with the legacy TracedClient interface"""

    TEST_SERVICE = 'mc-legacy'

    def get_client(self):
        url = '%s:%s' % (cfg['host'], cfg['port'])
        raw_client = pylibmc.Client([url])
        raw_client.flush_all()

        client = TracedClient(raw_client, tracer=self.tracer, service=self.TEST_SERVICE)
        return client, self.tracer


class TestPylibmcPatchDefault(BaseTracerTestCase, PylibmcCore):
    """Test suite for the tracing of pylibmc with the default lib patching"""

    def setUp(self):
        super(TestPylibmcPatchDefault, self).setUp()
        patch()

    def tearDown(self):
        unpatch()
        super(TestPylibmcPatchDefault, self).tearDown()

    def get_client(self):
        url = '%s:%s' % (cfg['host'], cfg['port'])
        client = pylibmc.Client([url])
        client.flush_all()

        Pin.get_from(client).clone(tracer=self.tracer).onto(client)

        return client, self.tracer


class TestPylibmcPatch(TestPylibmcPatchDefault):
    """Test suite for the tracing of pylibmc with a configured lib patching"""

    TEST_SERVICE = 'mc-custom-patch'

    def get_client(self):
        client, tracer = TestPylibmcPatchDefault.get_client(self)

        Pin.get_from(client).clone(service=self.TEST_SERVICE).onto(client)

        return client, tracer

    def test_patch_unpatch(self):
        url = '%s:%s' % (cfg['host'], cfg['port'])

        # Test patch idempotence
        patch()
        patch()

        client = pylibmc.Client([url])
        Pin.get_from(client).clone(
            service=self.TEST_SERVICE,
            tracer=self.tracer).onto(client)

        client.set('a', 1)

        spans = self.tracer.writer.pop()
        assert spans, spans
        assert len(spans) == 1

        # Test unpatch
        unpatch()

        client = pylibmc.Client([url])
        client.set('a', 1)

        spans = self.tracer.writer.pop()
        assert not spans, spans

        # Test patch again
        patch()

        client = pylibmc.Client([url])
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(client)
        client.set('a', 1)

        spans = self.tracer.writer.pop()
        assert spans, spans
        assert len(spans) == 1
