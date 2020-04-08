import mock

from ddtrace import Pin
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.dbapi import FetchTracedCursor, TracedCursor, TracedConnection
from ddtrace.span import Span
from ...base import BaseTracerTestCase


class TestTracedCursor(BaseTracerTestCase):

    def setUp(self):
        super(TestTracedCursor, self).setUp()
        self.cursor = mock.Mock()

    def test_execute_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.execute.return_value = '__result__'

        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = TracedCursor(cursor, pin)
        # DEV: We always pass through the result
        assert '__result__' == traced_cursor.execute('__query__', 'arg_1', kwarg1='kwarg1')
        cursor.execute.assert_called_once_with('__query__', 'arg_1', kwarg1='kwarg1')

    def test_executemany_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.executemany.return_value = '__result__'

        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = TracedCursor(cursor, pin)
        # DEV: We always pass through the result
        assert '__result__' == traced_cursor.executemany('__query__', 'arg_1', kwarg1='kwarg1')
        cursor.executemany.assert_called_once_with('__query__', 'arg_1', kwarg1='kwarg1')

    def test_fetchone_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.fetchone.return_value = '__result__'
        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = TracedCursor(cursor, pin)
        assert '__result__' == traced_cursor.fetchone('arg_1', kwarg1='kwarg1')
        cursor.fetchone.assert_called_once_with('arg_1', kwarg1='kwarg1')

    def test_fetchall_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.fetchall.return_value = '__result__'
        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = TracedCursor(cursor, pin)
        assert '__result__' == traced_cursor.fetchall('arg_1', kwarg1='kwarg1')
        cursor.fetchall.assert_called_once_with('arg_1', kwarg1='kwarg1')

    def test_fetchmany_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.fetchmany.return_value = '__result__'
        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = TracedCursor(cursor, pin)
        assert '__result__' == traced_cursor.fetchmany('arg_1', kwarg1='kwarg1')
        cursor.fetchmany.assert_called_once_with('arg_1', kwarg1='kwarg1')

    def test_correct_span_names(self):
        cursor = self.cursor
        tracer = self.tracer
        cursor.rowcount = 0
        pin = Pin('pin_name', tracer=tracer)
        traced_cursor = TracedCursor(cursor, pin)

        traced_cursor.execute('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='sql.query'))
        self.reset()

        traced_cursor.executemany('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='sql.query'))
        self.reset()

        traced_cursor.callproc('arg_1', 'arg2')
        self.assert_structure(dict(name='sql.query'))
        self.reset()

        traced_cursor.fetchone('arg_1', kwarg1='kwarg1')
        self.assert_has_no_spans()

        traced_cursor.fetchmany('arg_1', kwarg1='kwarg1')
        self.assert_has_no_spans()

        traced_cursor.fetchall('arg_1', kwarg1='kwarg1')
        self.assert_has_no_spans()

    def test_correct_span_names_can_be_overridden_by_pin(self):
        cursor = self.cursor
        tracer = self.tracer
        cursor.rowcount = 0
        pin = Pin('pin_name', app='changed', tracer=tracer)
        traced_cursor = TracedCursor(cursor, pin)

        traced_cursor.execute('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='changed.query'))
        self.reset()

        traced_cursor.executemany('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='changed.query'))
        self.reset()

        traced_cursor.callproc('arg_1', 'arg2')
        self.assert_structure(dict(name='changed.query'))
        self.reset()

        traced_cursor.fetchone('arg_1', kwarg1='kwarg1')
        self.assert_has_no_spans()

        traced_cursor.fetchmany('arg_1', kwarg1='kwarg1')
        self.assert_has_no_spans()

        traced_cursor.fetchall('arg_1', kwarg1='kwarg1')
        self.assert_has_no_spans()

    def test_when_pin_disabled_then_no_tracing(self):
        cursor = self.cursor
        tracer = self.tracer
        cursor.rowcount = 0
        cursor.execute.return_value = '__result__'
        cursor.executemany.return_value = '__result__'

        tracer.enabled = False
        pin = Pin('pin_name', tracer=tracer)
        traced_cursor = TracedCursor(cursor, pin)

        assert '__result__' == traced_cursor.execute('arg_1', kwarg1='kwarg1')
        assert len(tracer.writer.pop()) == 0

        assert '__result__' == traced_cursor.executemany('arg_1', kwarg1='kwarg1')
        assert len(tracer.writer.pop()) == 0

        cursor.callproc.return_value = 'callproc'
        assert 'callproc' == traced_cursor.callproc('arg_1', 'arg_2')
        assert len(tracer.writer.pop()) == 0

        cursor.fetchone.return_value = 'fetchone'
        assert 'fetchone' == traced_cursor.fetchone('arg_1', 'arg_2')
        assert len(tracer.writer.pop()) == 0

        cursor.fetchmany.return_value = 'fetchmany'
        assert 'fetchmany' == traced_cursor.fetchmany('arg_1', 'arg_2')
        assert len(tracer.writer.pop()) == 0

        cursor.fetchall.return_value = 'fetchall'
        assert 'fetchall' == traced_cursor.fetchall('arg_1', 'arg_2')
        assert len(tracer.writer.pop()) == 0

    def test_span_info(self):
        cursor = self.cursor
        tracer = self.tracer
        cursor.rowcount = 123
        pin = Pin('my_service', app='my_app', tracer=tracer, tags={'pin1': 'value_pin1'})
        traced_cursor = TracedCursor(cursor, pin)

        def method():
            pass

        traced_cursor._trace_method(method, 'my_name', 'my_resource', {'extra1': 'value_extra1'})
        span = tracer.writer.pop()[0]  # type: Span
        assert span.meta['pin1'] == 'value_pin1', 'Pin tags are preserved'
        assert span.meta['extra1'] == 'value_extra1', 'Extra tags are merged into pin tags'
        assert span.name == 'my_name', 'Span name is respected'
        assert span.service == 'my_service', 'Service from pin'
        assert span.resource == 'my_resource', 'Resource is respected'
        assert span.span_type == 'sql', 'Span has the correct span type'
        # Row count
        assert span.get_metric('db.rowcount') == 123, 'Row count is set as a metric'
        assert span.get_metric('sql.rows') == 123, 'Row count is set as a tag (for legacy django cursor replacement)'

    def test_django_traced_cursor_backward_compatibility(self):
        cursor = self.cursor
        tracer = self.tracer
        # Django integration used to have its own TracedCursor implementation. When we replaced such custom
        # implementation with the generic dbapi traced cursor, we had to make sure to add the tag 'sql.rows' that was
        # set by the legacy replaced implementation.
        cursor.rowcount = 123
        pin = Pin('my_service', app='my_app', tracer=tracer, tags={'pin1': 'value_pin1'})
        traced_cursor = TracedCursor(cursor, pin)

        def method():
            pass

        traced_cursor._trace_method(method, 'my_name', 'my_resource', {'extra1': 'value_extra1'})
        span = tracer.writer.pop()[0]  # type: Span
        # Row count
        assert span.get_metric('db.rowcount') == 123, 'Row count is set as a metric'
        assert span.get_metric('sql.rows') == 123, 'Row count is set as a tag (for legacy django cursor replacement)'

    def test_cursor_analytics_default(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.execute.return_value = '__result__'

        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = TracedCursor(cursor, pin)
        # DEV: We always pass through the result
        assert '__result__' == traced_cursor.execute('__query__', 'arg_1', kwarg1='kwarg1')

        span = self.tracer.writer.pop()[0]
        self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_cursor_analytics_with_rate(self):
        with self.override_config(
                'dbapi2',
                dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            cursor = self.cursor
            cursor.rowcount = 0
            cursor.execute.return_value = '__result__'

            pin = Pin('pin_name', tracer=self.tracer)
            traced_cursor = TracedCursor(cursor, pin)
            # DEV: We always pass through the result
            assert '__result__' == traced_cursor.execute('__query__', 'arg_1', kwarg1='kwarg1')

            span = self.tracer.writer.pop()[0]
            self.assertEqual(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    def test_cursor_analytics_without_rate(self):
        with self.override_config(
                'dbapi2',
                dict(analytics_enabled=True)
        ):
            cursor = self.cursor
            cursor.rowcount = 0
            cursor.execute.return_value = '__result__'

            pin = Pin('pin_name', tracer=self.tracer)
            traced_cursor = TracedCursor(cursor, pin)
            # DEV: We always pass through the result
            assert '__result__' == traced_cursor.execute('__query__', 'arg_1', kwarg1='kwarg1')

            span = self.tracer.writer.pop()[0]
            self.assertEqual(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)


class TestFetchTracedCursor(BaseTracerTestCase):

    def setUp(self):
        super(TestFetchTracedCursor, self).setUp()
        self.cursor = mock.Mock()

    def test_execute_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.execute.return_value = '__result__'

        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = FetchTracedCursor(cursor, pin)
        assert '__result__' == traced_cursor.execute('__query__', 'arg_1', kwarg1='kwarg1')
        cursor.execute.assert_called_once_with('__query__', 'arg_1', kwarg1='kwarg1')

    def test_executemany_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.executemany.return_value = '__result__'

        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = FetchTracedCursor(cursor, pin)
        assert '__result__' == traced_cursor.executemany('__query__', 'arg_1', kwarg1='kwarg1')
        cursor.executemany.assert_called_once_with('__query__', 'arg_1', kwarg1='kwarg1')

    def test_fetchone_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.fetchone.return_value = '__result__'
        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = FetchTracedCursor(cursor, pin)
        assert '__result__' == traced_cursor.fetchone('arg_1', kwarg1='kwarg1')
        cursor.fetchone.assert_called_once_with('arg_1', kwarg1='kwarg1')

    def test_fetchall_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.fetchall.return_value = '__result__'
        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = FetchTracedCursor(cursor, pin)
        assert '__result__' == traced_cursor.fetchall('arg_1', kwarg1='kwarg1')
        cursor.fetchall.assert_called_once_with('arg_1', kwarg1='kwarg1')

    def test_fetchmany_wrapped_is_called_and_returned(self):
        cursor = self.cursor
        cursor.rowcount = 0
        cursor.fetchmany.return_value = '__result__'
        pin = Pin('pin_name', tracer=self.tracer)
        traced_cursor = FetchTracedCursor(cursor, pin)
        assert '__result__' == traced_cursor.fetchmany('arg_1', kwarg1='kwarg1')
        cursor.fetchmany.assert_called_once_with('arg_1', kwarg1='kwarg1')

    def test_correct_span_names(self):
        cursor = self.cursor
        tracer = self.tracer
        cursor.rowcount = 0
        pin = Pin('pin_name', tracer=tracer)
        traced_cursor = FetchTracedCursor(cursor, pin)

        traced_cursor.execute('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='sql.query'))
        self.reset()

        traced_cursor.executemany('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='sql.query'))
        self.reset()

        traced_cursor.callproc('arg_1', 'arg2')
        self.assert_structure(dict(name='sql.query'))
        self.reset()

        traced_cursor.fetchone('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='sql.query.fetchone'))
        self.reset()

        traced_cursor.fetchmany('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='sql.query.fetchmany'))
        self.reset()

        traced_cursor.fetchall('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='sql.query.fetchall'))
        self.reset()

    def test_correct_span_names_can_be_overridden_by_pin(self):
        cursor = self.cursor
        tracer = self.tracer
        cursor.rowcount = 0
        pin = Pin('pin_name', app='changed', tracer=tracer)
        traced_cursor = FetchTracedCursor(cursor, pin)

        traced_cursor.execute('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='changed.query'))
        self.reset()

        traced_cursor.executemany('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='changed.query'))
        self.reset()

        traced_cursor.callproc('arg_1', 'arg2')
        self.assert_structure(dict(name='changed.query'))
        self.reset()

        traced_cursor.fetchone('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='changed.query.fetchone'))
        self.reset()

        traced_cursor.fetchmany('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='changed.query.fetchmany'))
        self.reset()

        traced_cursor.fetchall('arg_1', kwarg1='kwarg1')
        self.assert_structure(dict(name='changed.query.fetchall'))
        self.reset()

    def test_when_pin_disabled_then_no_tracing(self):
        cursor = self.cursor
        tracer = self.tracer
        cursor.rowcount = 0
        cursor.execute.return_value = '__result__'
        cursor.executemany.return_value = '__result__'

        tracer.enabled = False
        pin = Pin('pin_name', tracer=tracer)
        traced_cursor = FetchTracedCursor(cursor, pin)

        assert '__result__' == traced_cursor.execute('arg_1', kwarg1='kwarg1')
        assert len(tracer.writer.pop()) == 0

        assert '__result__' == traced_cursor.executemany('arg_1', kwarg1='kwarg1')
        assert len(tracer.writer.pop()) == 0

        cursor.callproc.return_value = 'callproc'
        assert 'callproc' == traced_cursor.callproc('arg_1', 'arg_2')
        assert len(tracer.writer.pop()) == 0

        cursor.fetchone.return_value = 'fetchone'
        assert 'fetchone' == traced_cursor.fetchone('arg_1', 'arg_2')
        assert len(tracer.writer.pop()) == 0

        cursor.fetchmany.return_value = 'fetchmany'
        assert 'fetchmany' == traced_cursor.fetchmany('arg_1', 'arg_2')
        assert len(tracer.writer.pop()) == 0

        cursor.fetchall.return_value = 'fetchall'
        assert 'fetchall' == traced_cursor.fetchall('arg_1', 'arg_2')
        assert len(tracer.writer.pop()) == 0

    def test_span_info(self):
        cursor = self.cursor
        tracer = self.tracer
        cursor.rowcount = 123
        pin = Pin('my_service', app='my_app', tracer=tracer, tags={'pin1': 'value_pin1'})
        traced_cursor = FetchTracedCursor(cursor, pin)

        def method():
            pass

        traced_cursor._trace_method(method, 'my_name', 'my_resource', {'extra1': 'value_extra1'})
        span = tracer.writer.pop()[0]  # type: Span
        assert span.meta['pin1'] == 'value_pin1', 'Pin tags are preserved'
        assert span.meta['extra1'] == 'value_extra1', 'Extra tags are merged into pin tags'
        assert span.name == 'my_name', 'Span name is respected'
        assert span.service == 'my_service', 'Service from pin'
        assert span.resource == 'my_resource', 'Resource is respected'
        assert span.span_type == 'sql', 'Span has the correct span type'
        # Row count
        assert span.get_metric('db.rowcount') == 123, 'Row count is set as a metric'
        assert span.get_metric('sql.rows') == 123, 'Row count is set as a tag (for legacy django cursor replacement)'

    def test_django_traced_cursor_backward_compatibility(self):
        cursor = self.cursor
        tracer = self.tracer
        # Django integration used to have its own FetchTracedCursor implementation. When we replaced such custom
        # implementation with the generic dbapi traced cursor, we had to make sure to add the tag 'sql.rows' that was
        # set by the legacy replaced implementation.
        cursor.rowcount = 123
        pin = Pin('my_service', app='my_app', tracer=tracer, tags={'pin1': 'value_pin1'})
        traced_cursor = FetchTracedCursor(cursor, pin)

        def method():
            pass

        traced_cursor._trace_method(method, 'my_name', 'my_resource', {'extra1': 'value_extra1'})
        span = tracer.writer.pop()[0]  # type: Span
        # Row count
        assert span.get_metric('db.rowcount') == 123, 'Row count is set as a metric'
        assert span.get_metric('sql.rows') == 123, 'Row count is set as a tag (for legacy django cursor replacement)'

    def test_fetch_no_analytics(self):
        """ Confirm fetch* methods do not have analytics sample rate metric """
        with self.override_config(
                'dbapi2',
                dict(analytics_enabled=True)
        ):
            cursor = self.cursor
            cursor.rowcount = 0
            cursor.fetchone.return_value = '__result__'
            pin = Pin('pin_name', tracer=self.tracer)
            traced_cursor = FetchTracedCursor(cursor, pin)
            assert '__result__' == traced_cursor.fetchone('arg_1', kwarg1='kwarg1')

            span = self.tracer.writer.pop()[0]
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

            cursor = self.cursor
            cursor.rowcount = 0
            cursor.fetchall.return_value = '__result__'
            pin = Pin('pin_name', tracer=self.tracer)
            traced_cursor = FetchTracedCursor(cursor, pin)
            assert '__result__' == traced_cursor.fetchall('arg_1', kwarg1='kwarg1')

            span = self.tracer.writer.pop()[0]
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

            cursor = self.cursor
            cursor.rowcount = 0
            cursor.fetchmany.return_value = '__result__'
            pin = Pin('pin_name', tracer=self.tracer)
            traced_cursor = FetchTracedCursor(cursor, pin)
            assert '__result__' == traced_cursor.fetchmany('arg_1', kwarg1='kwarg1')

            span = self.tracer.writer.pop()[0]
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))


class TestTracedConnection(BaseTracerTestCase):
    def setUp(self):
        super(TestTracedConnection, self).setUp()
        self.connection = mock.Mock()

    def test_cursor_class(self):
        pin = Pin('pin_name', tracer=self.tracer)

        # Default
        traced_connection = TracedConnection(self.connection, pin=pin)
        self.assertTrue(traced_connection._self_cursor_cls is TracedCursor)

        # Trace fetched methods
        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            traced_connection = TracedConnection(self.connection, pin=pin)
            self.assertTrue(traced_connection._self_cursor_cls is FetchTracedCursor)

        # Manually provided cursor class
        with self.override_config('dbapi2', dict(trace_fetch_methods=True)):
            traced_connection = TracedConnection(self.connection, pin=pin, cursor_cls=TracedCursor)
            self.assertTrue(traced_connection._self_cursor_cls is TracedCursor)

    def test_commit_is_traced(self):
        connection = self.connection
        tracer = self.tracer
        connection.commit.return_value = None
        pin = Pin('pin_name', tracer=tracer)
        traced_connection = TracedConnection(connection, pin)
        traced_connection.commit()
        assert tracer.writer.pop()[0].name == 'mock.connection.commit'
        connection.commit.assert_called_with()

    def test_rollback_is_traced(self):
        connection = self.connection
        tracer = self.tracer
        connection.rollback.return_value = None
        pin = Pin('pin_name', tracer=tracer)
        traced_connection = TracedConnection(connection, pin)
        traced_connection.rollback()
        assert tracer.writer.pop()[0].name == 'mock.connection.rollback'
        connection.rollback.assert_called_with()

    def test_connection_analytics_with_rate(self):
        with self.override_config(
                'dbapi2',
                dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            connection = self.connection
            tracer = self.tracer
            connection.commit.return_value = None
            pin = Pin('pin_name', tracer=tracer)
            traced_connection = TracedConnection(connection, pin)
            traced_connection.commit()
            span = tracer.writer.pop()[0]
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))
