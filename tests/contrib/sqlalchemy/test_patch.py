import sqlalchemy

from ddtrace import Pin
from ddtrace.contrib.sqlalchemy import patch, unpatch
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY

from ..config import POSTGRES_CONFIG
from ...base import BaseTracerTestCase


class SQLAlchemyPatchTestCase(BaseTracerTestCase):
    """TestCase that checks if the engine is properly traced
    when the `patch()` method is used.
    """
    def setUp(self):
        super(SQLAlchemyPatchTestCase, self).setUp()

        # create a traced engine with the given arguments
        # and configure the current PIN instance
        patch()
        dsn = 'postgresql://%(user)s:%(password)s@%(host)s:%(port)s/%(dbname)s' % POSTGRES_CONFIG
        self.engine = sqlalchemy.create_engine(dsn)
        Pin.override(self.engine, tracer=self.tracer)

        # prepare a connection
        self.conn = self.engine.connect()

    def tearDown(self):
        super(SQLAlchemyPatchTestCase, self).tearDown()

        # clear the database and dispose the engine
        self.conn.close()
        self.engine.dispose()
        unpatch()

    def test_engine_traced(self):
        # ensures that the engine is traced
        rows = self.conn.execute('SELECT 1').fetchall()
        assert len(rows) == 1

        traces = self.tracer.writer.pop_traces()
        # trace composition
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        # check subset of span fields
        assert span.name == 'postgres.query'
        assert span.service == 'postgres'
        assert span.error == 0
        assert span.duration > 0

    def test_engine_pin_service(self):
        # ensures that the engine service is updated with the PIN object
        Pin.override(self.engine, service='replica-db')
        rows = self.conn.execute('SELECT 1').fetchall()
        assert len(rows) == 1

        traces = self.tracer.writer.pop_traces()
        # trace composition
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        # check subset of span fields
        assert span.name == 'postgres.query'
        assert span.service == 'replica-db'
        assert span.error == 0
        assert span.duration > 0

    def test_analytics_sample_rate(self):
        # [ <config>, <analytics sample rate metric value> ]
        matrix = [
            # Default, not enabled, not set
            [dict(), None],

            # Not enabled, but sample rate set
            [dict(analytics_sample_rate=0.5), None],

            # Enabled and rate set
            [dict(analytics_enabled=True, analytics_sample_rate=0.5), 0.5],
            [dict(analytics_enabled=True, analytics_sample_rate=1), 1.0],
            [dict(analytics_enabled=True, analytics_sample_rate=0), 0],
            [dict(analytics_enabled=True, analytics_sample_rate=True), 1.0],
            [dict(analytics_enabled=True, analytics_sample_rate=False), 0],

            # Disabled and rate set
            [dict(analytics_enabled=False, analytics_sample_rate=0.5), None],

            # Enabled and rate not set
            [dict(analytics_enabled=True), 1.0],
        ]
        for config, metric_value in matrix:
            with self.override_config('sqlalchemy', config):
                self.conn.execute('SELECT 1').fetchall()

                root = self.get_root_span()
                root.assert_matches(name='postgres.query')

                # If the value is None assert it was not set, otherwise assert the expected value
                # DEV: root.assert_metrics(metrics, exact=True) won't work here since we have another sample
                #      rate keys getting added
                if metric_value is None:
                    assert ANALYTICS_SAMPLE_RATE_KEY not in root.metrics
                else:
                    assert root.metrics[ANALYTICS_SAMPLE_RATE_KEY] == metric_value
                self.reset()
