# stdlib
import time
import asyncio

# 3p
import aiopg
from psycopg2 import extras

# project
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.aiopg.patch import patch, unpatch
from ddtrace import Pin

# testing
from tests.opentracer.utils import init_tracer
from tests.contrib.config import POSTGRES_CONFIG
from tests.test_tracer import get_dummy_tracer
from tests.contrib.asyncio.utils import AsyncioTestCase, mark_asyncio


TEST_PORT = POSTGRES_CONFIG['port']


class AiopgTestCase(AsyncioTestCase):
    # default service
    TEST_SERVICE = 'postgres'

    def setUp(self):
        super().setUp()
        self._conn = None
        patch()

    def tearDown(self):
        super().tearDown()
        if self._conn and not self._conn.closed:
            self._conn.close()

        unpatch()

    @asyncio.coroutine
    def _get_conn_and_tracer(self):
        conn = self._conn = yield from aiopg.connect(**POSTGRES_CONFIG)
        Pin.get_from(conn).clone(tracer=self.tracer).onto(conn)

        return conn, self.tracer

    @asyncio.coroutine
    def assert_conn_is_traced(self, tracer, db, service):

        # ensure the trace aiopg client doesn't add non-standard
        # methods
        try:
            yield from db.execute('select \'foobar\'')
        except AttributeError:
            pass

        writer = tracer.writer
        # Ensure we can run a query and it's correctly traced
        q = 'select \'foobarblah\''
        start = time.time()
        cursor = yield from db.cursor()
        yield from cursor.execute(q)
        rows = yield from cursor.fetchall()
        end = time.time()
        assert rows == [('foobarblah',)]
        assert rows
        spans = writer.pop()
        assert spans
        assert len(spans) == 1
        span = spans[0]
        assert span.name == 'postgres.query'
        assert span.resource == q
        assert span.service == service
        assert span.meta['sql.query'] == q
        assert span.error == 0
        assert span.span_type == 'sql'
        assert start <= span.start <= end
        assert span.duration <= end - start

        # Ensure OpenTracing compatibility
        ot_tracer = init_tracer('aiopg_svc', tracer)
        with ot_tracer.start_active_span('aiopg_op'):
            cursor = yield from db.cursor()
            yield from cursor.execute(q)
            rows = yield from cursor.fetchall()
            assert rows == [('foobarblah',)]
        spans = writer.pop()
        assert len(spans) == 2
        ot_span, dd_span = spans
        # confirm the parenting
        assert ot_span.parent_id is None
        assert dd_span.parent_id == ot_span.span_id
        assert ot_span.name == 'aiopg_op'
        assert ot_span.service == 'aiopg_svc'
        assert dd_span.name == 'postgres.query'
        assert dd_span.resource == q
        assert dd_span.service == service
        assert dd_span.meta['sql.query'] == q
        assert dd_span.error == 0
        assert dd_span.span_type == 'sql'

        # run a query with an error and ensure all is well
        q = 'select * from some_non_existant_table'
        cur = yield from db.cursor()
        try:
            yield from cur.execute(q)
        except Exception:
            pass
        else:
            assert 0, 'should have an error'
        spans = writer.pop()
        assert spans, spans
        assert len(spans) == 1
        span = spans[0]
        assert span.name == 'postgres.query'
        assert span.resource == q
        assert span.service == service
        assert span.meta['sql.query'] == q
        assert span.error == 1
        # assert span.meta['out.host'] == 'localhost'
        assert span.metrics['out.port'] == TEST_PORT
        assert span.span_type == 'sql'

    @mark_asyncio
    def test_disabled_execute(self):
        conn, tracer = yield from self._get_conn_and_tracer()
        tracer.enabled = False
        # these calls were crashing with a previous version of the code.
        yield from (yield from conn.cursor()).execute(query='select \'blah\'')
        yield from (yield from conn.cursor()).execute('select \'blah\'')
        assert not tracer.writer.pop()

    @mark_asyncio
    def test_manual_wrap_extension_types(self):
        conn, _ = yield from self._get_conn_and_tracer()
        # NOTE: this will crash if it doesn't work.
        #   _ext.register_type(_ext.UUID, conn_or_curs)
        #   TypeError: argument 2 must be a connection, cursor or None
        extras.register_uuid(conn_or_curs=conn)

    @mark_asyncio
    def test_connect_factory(self):
        tracer = get_dummy_tracer()

        services = ['db', 'another']
        for service in services:
            conn, _ = yield from self._get_conn_and_tracer()
            Pin.get_from(conn).clone(service=service, tracer=tracer).onto(conn)
            yield from self.assert_conn_is_traced(tracer, conn, service)
            conn.close()

        # ensure we have the service types
        service_meta = tracer.writer.pop_services()
        expected = {}
        assert service_meta == expected

    @mark_asyncio
    def test_patch_unpatch(self):
        tracer = get_dummy_tracer()
        writer = tracer.writer

        # Test patch idempotence
        patch()
        patch()

        service = 'fo'

        conn = yield from aiopg.connect(**POSTGRES_CONFIG)
        Pin.get_from(conn).clone(service=service, tracer=tracer).onto(conn)
        yield from (yield from conn.cursor()).execute('select \'blah\'')
        conn.close()

        spans = writer.pop()
        assert spans, spans
        assert len(spans) == 1

        # Test unpatch
        unpatch()

        conn = yield from aiopg.connect(**POSTGRES_CONFIG)
        yield from (yield from conn.cursor()).execute('select \'blah\'')
        conn.close()

        spans = writer.pop()
        assert not spans, spans

        # Test patch again
        patch()

        conn = yield from aiopg.connect(**POSTGRES_CONFIG)
        Pin.get_from(conn).clone(service=service, tracer=tracer).onto(conn)
        yield from (yield from conn.cursor()).execute('select \'blah\'')
        conn.close()

        spans = writer.pop()
        assert spans, spans
        assert len(spans) == 1


class AiopgAnalyticsTestCase(AiopgTestCase):
    @asyncio.coroutine
    def trace_spans(self):
        conn, _ = yield from self._get_conn_and_tracer()

        Pin.get_from(conn).clone(service='db', tracer=self.tracer).onto(conn)

        cursor = yield from conn.cursor()
        yield from cursor.execute('select \'foobar\'')
        rows = yield from cursor.fetchall()
        assert rows

        return self.get_spans()

    @mark_asyncio
    def test_analytics_default(self):
        spans = yield from self.trace_spans()
        self.assertEqual(len(spans), 1)
        self.assertIsNone(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    @mark_asyncio
    def test_analytics_with_rate(self):
        with self.override_config(
            'aiopg',
            dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            spans = yield from self.trace_spans()
            self.assertEqual(len(spans), 1)
            self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    @mark_asyncio
    def test_analytics_without_rate(self):
        with self.override_config(
            'aiopg',
            dict(analytics_enabled=True)
        ):
            spans = yield from self.trace_spans()
            self.assertEqual(len(spans), 1)
            self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)
