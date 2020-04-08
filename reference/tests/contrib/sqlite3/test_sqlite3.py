# stdlib
import sqlite3
import time

# project
import ddtrace
from ddtrace import Pin
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.sqlite3 import connection_factory
from ddtrace.contrib.sqlite3.patch import patch, unpatch, TracedSQLiteCursor
from ddtrace.ext import errors

# testing
from tests.opentracer.utils import init_tracer
from ...base import BaseTracerTestCase


class TestSQLite(BaseTracerTestCase):
    def setUp(self):
        super(TestSQLite, self).setUp()
        patch()

    def tearDown(self):
        unpatch()
        super(TestSQLite, self).tearDown()

    def test_backwards_compat(self):
        # a small test to ensure that if the previous interface is used
        # things still work
        factory = connection_factory(self.tracer, service='my_db_service')
        conn = sqlite3.connect(':memory:', factory=factory)
        q = 'select * from sqlite_master'
        cursor = conn.execute(q)
        self.assertIsInstance(cursor, TracedSQLiteCursor)
        assert not cursor.fetchall()
        assert not self.spans

    def test_service_info(self):
        backup_tracer = ddtrace.tracer
        ddtrace.tracer = self.tracer

        sqlite3.connect(':memory:')

        services = self.tracer.writer.pop_services()
        self.assertEqual(services, {})

        ddtrace.tracer = backup_tracer

    def test_sqlite(self):
        # ensure we can trace multiple services without stomping
        services = ['db', 'another']
        for service in services:
            db = sqlite3.connect(':memory:')
            pin = Pin.get_from(db)
            assert pin
            pin.clone(
                service=service,
                tracer=self.tracer).onto(db)

            # Ensure we can run a query and it's correctly traced
            q = 'select * from sqlite_master'
            start = time.time()
            cursor = db.execute(q)
            self.assertIsInstance(cursor, TracedSQLiteCursor)
            rows = cursor.fetchall()
            end = time.time()
            assert not rows
            self.assert_structure(
                dict(name='sqlite.query', span_type='sql', resource=q, service=service, error=0),
            )
            root = self.get_root_span()
            self.assertIsNone(root.get_tag('sql.query'))
            assert start <= root.start <= end
            assert root.duration <= end - start
            self.reset()

            # run a query with an error and ensure all is well
            q = 'select * from some_non_existant_table'
            try:
                db.execute(q)
            except Exception:
                pass
            else:
                assert 0, 'should have an error'

            self.assert_structure(
                dict(name='sqlite.query', span_type='sql', resource=q, service=service, error=1),
            )
            root = self.get_root_span()
            self.assertIsNone(root.get_tag('sql.query'))
            self.assertIsNotNone(root.get_tag(errors.ERROR_STACK))
            self.assertIn('OperationalError', root.get_tag(errors.ERROR_TYPE))
            self.assertIn('no such table', root.get_tag(errors.ERROR_MSG))
            self.reset()

    def test_sqlite_fetchall_is_traced(self):
        q = 'select * from sqlite_master'

        # Not traced by default
        connection = self._given_a_traced_connection(self.tracer)
        cursor = connection.execute(q)
        cursor.fetchall()
        self.assert_structure(dict(name='sqlite.query', resource=q))
        self.reset()

        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            connection = self._given_a_traced_connection(self.tracer)
            cursor = connection.execute(q)
            cursor.fetchall()

            # We have two spans side by side
            query_span, fetchall_span = self.get_root_spans()

            # Assert query
            query_span.assert_structure(dict(name='sqlite.query', resource=q))

            # Assert fetchall
            fetchall_span.assert_structure(dict(name='sqlite.query.fetchall', resource=q, span_type='sql', error=0))
            self.assertIsNone(fetchall_span.get_tag('sql.query'))

    def test_sqlite_fetchone_is_traced(self):
        q = 'select * from sqlite_master'

        # Not traced by default
        connection = self._given_a_traced_connection(self.tracer)
        cursor = connection.execute(q)
        cursor.fetchone()
        self.assert_structure(dict(name='sqlite.query', resource=q))
        self.reset()

        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            connection = self._given_a_traced_connection(self.tracer)
            cursor = connection.execute(q)
            cursor.fetchone()

            # We have two spans side by side
            query_span, fetchone_span = self.get_root_spans()

            # Assert query
            query_span.assert_structure(dict(name='sqlite.query', resource=q))

            # Assert fetchone
            fetchone_span.assert_structure(
                dict(
                    name='sqlite.query.fetchone',
                    resource=q,
                    span_type='sql',
                    error=0,
                ),
            )
            self.assertIsNone(fetchone_span.get_tag('sql.query'))

    def test_sqlite_fetchmany_is_traced(self):
        q = 'select * from sqlite_master'

        # Not traced by default
        connection = self._given_a_traced_connection(self.tracer)
        cursor = connection.execute(q)
        cursor.fetchmany(123)
        self.assert_structure(dict(name='sqlite.query', resource=q))
        self.reset()

        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            connection = self._given_a_traced_connection(self.tracer)
            cursor = connection.execute(q)
            cursor.fetchmany(123)

            # We have two spans side by side
            query_span, fetchmany_span = self.get_root_spans()

            # Assert query
            query_span.assert_structure(dict(name='sqlite.query', resource=q))

            # Assert fetchmany
            fetchmany_span.assert_structure(
                dict(
                    name='sqlite.query.fetchmany',
                    resource=q,
                    span_type='sql',
                    error=0,
                    metrics={'db.fetch.size': 123},
                ),
            )
            self.assertIsNone(fetchmany_span.get_tag('sql.query'))

    def test_sqlite_ot(self):
        """Ensure sqlite works with the opentracer."""
        ot_tracer = init_tracer('sqlite_svc', self.tracer)

        # Ensure we can run a query and it's correctly traced
        q = 'select * from sqlite_master'
        with ot_tracer.start_active_span('sqlite_op'):
            db = sqlite3.connect(':memory:')
            pin = Pin.get_from(db)
            assert pin
            pin.clone(tracer=self.tracer).onto(db)
            cursor = db.execute(q)
            rows = cursor.fetchall()
        assert not rows

        self.assert_structure(
            dict(name='sqlite_op', service='sqlite_svc'),
            (
                dict(name='sqlite.query', span_type='sql', resource=q, error=0),
            )
        )
        self.reset()

        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            with ot_tracer.start_active_span('sqlite_op'):
                db = sqlite3.connect(':memory:')
                pin = Pin.get_from(db)
                assert pin
                pin.clone(tracer=self.tracer).onto(db)
                cursor = db.execute(q)
                rows = cursor.fetchall()
                assert not rows

            self.assert_structure(
                dict(name='sqlite_op', service='sqlite_svc'),
                (
                    dict(name='sqlite.query', span_type='sql', resource=q, error=0),
                    dict(name='sqlite.query.fetchall', span_type='sql', resource=q, error=0),
                ),
            )

    def test_commit(self):
        connection = self._given_a_traced_connection(self.tracer)
        connection.commit()
        self.assertEqual(len(self.spans), 1)
        span = self.spans[0]
        self.assertEqual(span.service, 'sqlite')
        self.assertEqual(span.name, 'sqlite.connection.commit')

    def test_rollback(self):
        connection = self._given_a_traced_connection(self.tracer)
        connection.rollback()
        self.assert_structure(
            dict(name='sqlite.connection.rollback', service='sqlite'),
        )

    def test_patch_unpatch(self):
        # Test patch idempotence
        patch()
        patch()

        db = sqlite3.connect(':memory:')
        pin = Pin.get_from(db)
        assert pin
        pin.clone(tracer=self.tracer).onto(db)
        db.cursor().execute('select \'blah\'').fetchall()

        self.assert_structure(
            dict(name='sqlite.query'),
        )
        self.reset()

        # Test unpatch
        unpatch()

        db = sqlite3.connect(':memory:')
        db.cursor().execute('select \'blah\'').fetchall()

        self.assert_has_no_spans()

        # Test patch again
        patch()

        db = sqlite3.connect(':memory:')
        pin = Pin.get_from(db)
        assert pin
        pin.clone(tracer=self.tracer).onto(db)
        db.cursor().execute('select \'blah\'').fetchall()

        self.assert_structure(
            dict(name='sqlite.query'),
        )

    def _given_a_traced_connection(self, tracer):
        db = sqlite3.connect(':memory:')
        Pin.get_from(db).clone(tracer=tracer).onto(db)
        return db

    def test_analytics_default(self):
        q = 'select * from sqlite_master'
        connection = self._given_a_traced_connection(self.tracer)
        cursor = connection.execute(q)
        cursor.fetchall()

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_with_rate(self):
        with self.override_config(
                'dbapi2',
                dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            q = 'select * from sqlite_master'
            connection = self._given_a_traced_connection(self.tracer)
            cursor = connection.execute(q)
            cursor.fetchall()

            spans = self.get_spans()
            self.assertEqual(len(spans), 1)
            span = spans[0]
            self.assertEqual(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    def test_analytics_without_rate(self):
        with self.override_config(
                'dbapi2',
                dict(analytics_enabled=True)
        ):
            q = 'select * from sqlite_master'
            connection = self._given_a_traced_connection(self.tracer)
            cursor = connection.execute(q)
            cursor.fetchall()

            spans = self.get_spans()

            self.assertEqual(len(spans), 1)
            span = spans[0]
            self.assertEqual(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)
