import pytest

from sqlalchemy.exc import OperationalError

from .mixins import SQLAlchemyTestMixin
from ...base import BaseTracerTestCase


class SQLiteTestCase(SQLAlchemyTestMixin, BaseTracerTestCase):
    """TestCase for the SQLite engine"""
    VENDOR = 'sqlite'
    SQL_DB = ':memory:'
    SERVICE = 'sqlite'
    ENGINE_ARGS = {'url': 'sqlite:///:memory:'}

    def setUp(self):
        super(SQLiteTestCase, self).setUp()

    def tearDown(self):
        super(SQLiteTestCase, self).tearDown()

    def test_engine_execute_errors(self):
        # ensures that SQL errors are reported
        with pytest.raises(OperationalError):
            with self.connection() as conn:
                conn.execute('SELECT * FROM a_wrong_table').fetchall()

        traces = self.tracer.writer.pop_traces()
        # trace composition
        self.assertEqual(len(traces), 1)
        self.assertEqual(len(traces[0]), 1)
        span = traces[0][0]
        # span fields
        self.assertEqual(span.name, '{}.query'.format(self.VENDOR))
        self.assertEqual(span.service, self.SERVICE)
        self.assertEqual(span.resource, 'SELECT * FROM a_wrong_table')
        self.assertEqual(span.get_tag('sql.db'), self.SQL_DB)
        self.assertIsNone(span.get_tag('sql.rows') or span.get_metric('sql.rows'))
        self.assertEqual(span.span_type, 'sql')
        self.assertTrue(span.duration > 0)
        # check the error
        self.assertEqual(span.error, 1)
        self.assertEqual(span.get_tag('error.msg'), 'no such table: a_wrong_table')
        self.assertTrue('OperationalError' in span.get_tag('error.type'))
        self.assertTrue('OperationalError: no such table: a_wrong_table' in span.get_tag('error.stack'))
