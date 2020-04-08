# 3p
import mysql

# project
from ddtrace import Pin
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.mysql.patch import patch, unpatch

# tests
from tests.contrib.config import MYSQL_CONFIG
from tests.opentracer.utils import init_tracer
from ...base import BaseTracerTestCase
from ...util import assert_dict_issuperset


class MySQLCore(object):
    """Base test case for MySQL drivers"""
    conn = None
    TEST_SERVICE = 'test-mysql'

    def tearDown(self):
        super(MySQLCore, self).tearDown()

        # Reuse the connection across tests
        if self.conn:
            try:
                self.conn.ping()
            except mysql.InterfaceError:
                pass
            else:
                self.conn.close()
        unpatch()

    def _get_conn_tracer(self):
        # implement me
        pass

    def test_simple_query(self):
        conn, tracer = self._get_conn_tracer()
        writer = tracer.writer
        cursor = conn.cursor()
        cursor.execute('SELECT 1')
        rows = cursor.fetchall()
        assert len(rows) == 1
        spans = writer.pop()
        assert len(spans) == 1

        span = spans[0]
        assert span.service == self.TEST_SERVICE
        assert span.name == 'mysql.query'
        assert span.span_type == 'sql'
        assert span.error == 0
        assert span.get_metric('out.port') == 3306
        assert_dict_issuperset(span.meta, {
            'out.host': u'127.0.0.1',
            'db.name': u'test',
            'db.user': u'test',
        })

    def test_simple_query_fetchll(self):
        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            conn, tracer = self._get_conn_tracer()
            writer = tracer.writer
            cursor = conn.cursor()
            cursor.execute('SELECT 1')
            rows = cursor.fetchall()
            assert len(rows) == 1
            spans = writer.pop()
            assert len(spans) == 2

            span = spans[0]
            assert span.service == self.TEST_SERVICE
            assert span.name == 'mysql.query'
            assert span.span_type == 'sql'
            assert span.error == 0
            assert span.get_metric('out.port') == 3306
            assert_dict_issuperset(span.meta, {
                'out.host': u'127.0.0.1',
                'db.name': u'test',
                'db.user': u'test',
            })

            assert spans[1].name == 'mysql.query.fetchall'

    def test_query_with_several_rows(self):
        conn, tracer = self._get_conn_tracer()
        writer = tracer.writer
        cursor = conn.cursor()
        query = 'SELECT n FROM (SELECT 42 n UNION SELECT 421 UNION SELECT 4210) m'
        cursor.execute(query)
        rows = cursor.fetchall()
        assert len(rows) == 3
        spans = writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert span.get_tag('sql.query') is None

    def test_query_with_several_rows_fetchall(self):
        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            conn, tracer = self._get_conn_tracer()
            writer = tracer.writer
            cursor = conn.cursor()
            query = 'SELECT n FROM (SELECT 42 n UNION SELECT 421 UNION SELECT 4210) m'
            cursor.execute(query)
            rows = cursor.fetchall()
            assert len(rows) == 3
            spans = writer.pop()
            assert len(spans) == 2
            span = spans[0]
            assert span.get_tag('sql.query') is None
            assert spans[1].name == 'mysql.query.fetchall'

    def test_query_many(self):
        # tests that the executemany method is correctly wrapped.
        conn, tracer = self._get_conn_tracer()
        writer = tracer.writer
        tracer.enabled = False
        cursor = conn.cursor()

        cursor.execute("""
            create table if not exists dummy (
                dummy_key VARCHAR(32) PRIMARY KEY,
                dummy_value TEXT NOT NULL)""")
        tracer.enabled = True

        stmt = 'INSERT INTO dummy (dummy_key, dummy_value) VALUES (%s, %s)'
        data = [
            ('foo', 'this is foo'),
            ('bar', 'this is bar'),
        ]
        cursor.executemany(stmt, data)
        query = 'SELECT dummy_key, dummy_value FROM dummy ORDER BY dummy_key'
        cursor.execute(query)
        rows = cursor.fetchall()
        assert len(rows) == 2
        assert rows[0][0] == 'bar'
        assert rows[0][1] == 'this is bar'
        assert rows[1][0] == 'foo'
        assert rows[1][1] == 'this is foo'

        spans = writer.pop()
        assert len(spans) == 2
        span = spans[-1]
        assert span.get_tag('sql.query') is None
        cursor.execute('drop table if exists dummy')

    def test_query_many_fetchall(self):
        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            # tests that the executemany method is correctly wrapped.
            conn, tracer = self._get_conn_tracer()
            writer = tracer.writer
            tracer.enabled = False
            cursor = conn.cursor()

            cursor.execute("""
                create table if not exists dummy (
                    dummy_key VARCHAR(32) PRIMARY KEY,
                    dummy_value TEXT NOT NULL)""")
            tracer.enabled = True

            stmt = 'INSERT INTO dummy (dummy_key, dummy_value) VALUES (%s, %s)'
            data = [
                ('foo', 'this is foo'),
                ('bar', 'this is bar'),
            ]
            cursor.executemany(stmt, data)
            query = 'SELECT dummy_key, dummy_value FROM dummy ORDER BY dummy_key'
            cursor.execute(query)
            rows = cursor.fetchall()
            assert len(rows) == 2
            assert rows[0][0] == 'bar'
            assert rows[0][1] == 'this is bar'
            assert rows[1][0] == 'foo'
            assert rows[1][1] == 'this is foo'

            spans = writer.pop()
            assert len(spans) == 3
            span = spans[-1]
            assert span.get_tag('sql.query') is None
            cursor.execute('drop table if exists dummy')

            assert spans[2].name == 'mysql.query.fetchall'

    def test_query_proc(self):
        conn, tracer = self._get_conn_tracer()
        writer = tracer.writer

        # create a procedure
        tracer.enabled = False
        cursor = conn.cursor()
        cursor.execute('DROP PROCEDURE IF EXISTS sp_sum')
        cursor.execute("""
            CREATE PROCEDURE sp_sum (IN p1 INTEGER, IN p2 INTEGER, OUT p3 INTEGER)
            BEGIN
                SET p3 := p1 + p2;
            END;""")

        tracer.enabled = True
        proc = 'sp_sum'
        data = (40, 2, None)
        output = cursor.callproc(proc, data)
        assert len(output) == 3
        assert output[2] == 42

        spans = writer.pop()
        assert spans, spans

        # number of spans depends on MySQL implementation details,
        # typically, internal calls to execute, but at least we
        # can expect the last closed span to be our proc.
        span = spans[len(spans) - 1]
        assert span.service == self.TEST_SERVICE
        assert span.name == 'mysql.query'
        assert span.span_type == 'sql'
        assert span.error == 0
        assert span.get_metric('out.port') == 3306
        assert_dict_issuperset(span.meta, {
            'out.host': u'127.0.0.1',
            'db.name': u'test',
            'db.user': u'test',
        })
        assert span.get_tag('sql.query') is None

    def test_simple_query_ot(self):
        """OpenTracing version of test_simple_query."""
        conn, tracer = self._get_conn_tracer()
        writer = tracer.writer

        ot_tracer = init_tracer('mysql_svc', tracer)

        with ot_tracer.start_active_span('mysql_op'):
            cursor = conn.cursor()
            cursor.execute('SELECT 1')
            rows = cursor.fetchall()
            assert len(rows) == 1

        spans = writer.pop()
        assert len(spans) == 2

        ot_span, dd_span = spans

        # confirm parenting
        assert ot_span.parent_id is None
        assert dd_span.parent_id == ot_span.span_id

        assert ot_span.service == 'mysql_svc'
        assert ot_span.name == 'mysql_op'

        assert dd_span.service == self.TEST_SERVICE
        assert dd_span.name == 'mysql.query'
        assert dd_span.span_type == 'sql'
        assert dd_span.error == 0
        assert dd_span.get_metric('out.port') == 3306
        assert_dict_issuperset(dd_span.meta, {
            'out.host': u'127.0.0.1',
            'db.name': u'test',
            'db.user': u'test',
        })

    def test_simple_query_ot_fetchall(self):
        """OpenTracing version of test_simple_query."""
        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            conn, tracer = self._get_conn_tracer()
            writer = tracer.writer

            ot_tracer = init_tracer('mysql_svc', tracer)

            with ot_tracer.start_active_span('mysql_op'):
                cursor = conn.cursor()
                cursor.execute('SELECT 1')
                rows = cursor.fetchall()
                assert len(rows) == 1

            spans = writer.pop()
            assert len(spans) == 3

            ot_span, dd_span, fetch_span = spans

            # confirm parenting
            assert ot_span.parent_id is None
            assert dd_span.parent_id == ot_span.span_id

            assert ot_span.service == 'mysql_svc'
            assert ot_span.name == 'mysql_op'

            assert dd_span.service == self.TEST_SERVICE
            assert dd_span.name == 'mysql.query'
            assert dd_span.span_type == 'sql'
            assert dd_span.error == 0
            assert dd_span.get_metric('out.port') == 3306
            assert_dict_issuperset(dd_span.meta, {
                'out.host': u'127.0.0.1',
                'db.name': u'test',
                'db.user': u'test',
            })

            assert fetch_span.name == 'mysql.query.fetchall'

    def test_commit(self):
        conn, tracer = self._get_conn_tracer()
        writer = tracer.writer
        conn.commit()
        spans = writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.TEST_SERVICE
        assert span.name == 'mysql.connection.commit'

    def test_rollback(self):
        conn, tracer = self._get_conn_tracer()
        writer = tracer.writer
        conn.rollback()
        spans = writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert span.service == self.TEST_SERVICE
        assert span.name == 'mysql.connection.rollback'

    def test_analytics_default(self):
        conn, tracer = self._get_conn_tracer()
        writer = tracer.writer
        cursor = conn.cursor()
        cursor.execute('SELECT 1')
        rows = cursor.fetchall()
        assert len(rows) == 1
        spans = writer.pop()

        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_with_rate(self):
        with self.override_config(
                'dbapi2',
                dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            conn, tracer = self._get_conn_tracer()
            writer = tracer.writer
            cursor = conn.cursor()
            cursor.execute('SELECT 1')
            rows = cursor.fetchall()
            assert len(rows) == 1
            spans = writer.pop()

            self.assertEqual(len(spans), 1)
            span = spans[0]
            self.assertEqual(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    def test_analytics_without_rate(self):
        with self.override_config(
                'dbapi2',
                dict(analytics_enabled=True)
        ):
            conn, tracer = self._get_conn_tracer()
            writer = tracer.writer
            cursor = conn.cursor()
            cursor.execute('SELECT 1')
            rows = cursor.fetchall()
            assert len(rows) == 1
            spans = writer.pop()

            self.assertEqual(len(spans), 1)
            span = spans[0]
            self.assertEqual(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)


class TestMysqlPatch(MySQLCore, BaseTracerTestCase):

    def setUp(self):
        super(TestMysqlPatch, self).setUp()
        patch()

    def tearDown(self):
        super(TestMysqlPatch, self).tearDown()
        unpatch()

    def _get_conn_tracer(self):
        if not self.conn:
            self.conn = mysql.connector.connect(**MYSQL_CONFIG)
            assert self.conn.is_connected()
            # Ensure that the default pin is there, with its default value
            pin = Pin.get_from(self.conn)
            assert pin
            assert pin.service == 'mysql'
            # Customize the service
            # we have to apply it on the existing one since new one won't inherit `app`
            pin.clone(
                service=self.TEST_SERVICE, tracer=self.tracer).onto(self.conn)

            return self.conn, self.tracer

    def test_patch_unpatch(self):
        unpatch()
        # assert we start unpatched
        conn = mysql.connector.connect(**MYSQL_CONFIG)
        assert not Pin.get_from(conn)
        conn.close()

        patch()
        try:
            writer = self.tracer.writer
            conn = mysql.connector.connect(**MYSQL_CONFIG)
            pin = Pin.get_from(conn)
            assert pin
            pin.clone(
                service=self.TEST_SERVICE, tracer=self.tracer).onto(conn)
            assert conn.is_connected()

            cursor = conn.cursor()
            cursor.execute('SELECT 1')
            rows = cursor.fetchall()
            assert len(rows) == 1
            spans = writer.pop()
            assert len(spans) == 1

            span = spans[0]
            assert span.service == self.TEST_SERVICE
            assert span.name == 'mysql.query'
            assert span.span_type == 'sql'
            assert span.error == 0
            assert span.get_metric('out.port') == 3306
            assert_dict_issuperset(span.meta, {
                'out.host': u'127.0.0.1',
                'db.name': u'test',
                'db.user': u'test',
            })
            assert span.get_tag('sql.query') is None

        finally:
            unpatch()

            # assert we finish unpatched
            conn = mysql.connector.connect(**MYSQL_CONFIG)
            assert not Pin.get_from(conn)
            conn.close()

        patch()
