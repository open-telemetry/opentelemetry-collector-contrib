from sqlalchemy.exc import ProgrammingError
import pytest

from .mixins import SQLAlchemyTestMixin
from ..config import MYSQL_CONFIG
from ...base import BaseTracerTestCase


class MysqlConnectorTestCase(SQLAlchemyTestMixin, BaseTracerTestCase):
    """TestCase for mysql-connector engine"""
    VENDOR = 'mysql'
    SQL_DB = 'test'
    SERVICE = 'mysql'
    ENGINE_ARGS = {'url': 'mysql+mysqlconnector://%(user)s:%(password)s@%(host)s:%(port)s/%(database)s' % MYSQL_CONFIG}

    def setUp(self):
        super(MysqlConnectorTestCase, self).setUp()

    def tearDown(self):
        super(MysqlConnectorTestCase, self).tearDown()

    def check_meta(self, span):
        # check database connection tags
        self.assertEqual(span.get_tag('out.host'), MYSQL_CONFIG['host'])
        self.assertEqual(span.get_metric('out.port'), MYSQL_CONFIG['port'])

    def test_engine_execute_errors(self):
        # ensures that SQL errors are reported
        with pytest.raises(ProgrammingError):
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
        self.check_meta(span)
        self.assertEqual(span.span_type, 'sql')
        self.assertTrue(span.duration > 0)
        # check the error
        self.assertEqual(span.error, 1)
        self.assertEqual(span.get_tag('error.type'), 'mysql.connector.errors.ProgrammingError')
        self.assertTrue("Table 'test.a_wrong_table' doesn't exist" in span.get_tag('error.msg'))
        self.assertTrue("Table 'test.a_wrong_table' doesn't exist" in span.get_tag('error.stack'))
