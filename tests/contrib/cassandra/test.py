# stdlib
import contextlib
import logging
import unittest
from threading import Event

# 3p
from cassandra.cluster import Cluster, ResultSet
from cassandra.query import BatchStatement, SimpleStatement

# project
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.cassandra.patch import patch, unpatch
from ddtrace.contrib.cassandra.session import get_traced_cassandra, SERVICE
from ddtrace.ext import net, cassandra as cassx, errors
from ddtrace import config, Pin

# testing
from tests.contrib.config import CASSANDRA_CONFIG
from tests.opentracer.utils import init_tracer
from tests.test_tracer import get_dummy_tracer

# Oftentimes our tests fails because Cassandra connection timeouts during keyspace drop. Slowness in keyspace drop
# is known and is due to 'auto_snapshot' configuration. In our test env we should disable it, but the official cassandra
# image that we are using only allows us to configure a few configs:
# https://github.com/docker-library/cassandra/blob/4474c6c5cc2a81ee57c5615aae00555fca7e26a6/3.11/docker-entrypoint.sh#L51
# So for now we just increase the timeout, if this is not enough we may want to extend the official image with our own
# custom image.
CONNECTION_TIMEOUT_SECS = 20  # override the default value of 5

logging.getLogger('cassandra').setLevel(logging.INFO)


def setUpModule():
    # skip all the modules if the Cluster is not available
    if not Cluster:
        raise unittest.SkipTest('cassandra.cluster.Cluster is not available.')

    # create the KEYSPACE for this test module
    cluster = Cluster(port=CASSANDRA_CONFIG['port'], connect_timeout=CONNECTION_TIMEOUT_SECS)
    session = cluster.connect()
    session.execute('DROP KEYSPACE IF EXISTS test', timeout=10)
    session.execute(
        "CREATE KEYSPACE if not exists test WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor': 1};"
    )
    session.execute('CREATE TABLE if not exists test.person (name text PRIMARY KEY, age int, description text)')
    session.execute('CREATE TABLE if not exists test.person_write (name text PRIMARY KEY, age int, description text)')
    session.execute("INSERT INTO test.person (name, age, description) VALUES ('Cassandra', 100, 'A cruel mistress')")
    session.execute(
        "INSERT INTO test.person (name, age, description) VALUES ('Athena', 100, 'Whose shield is thunder')"
    )
    session.execute("INSERT INTO test.person (name, age, description) VALUES ('Calypso', 100, 'Softly-braided nymph')")


def tearDownModule():
    # destroy the KEYSPACE
    cluster = Cluster(port=CASSANDRA_CONFIG['port'], connect_timeout=CONNECTION_TIMEOUT_SECS)
    session = cluster.connect()
    session.execute('DROP TABLE IF EXISTS test.person')
    session.execute('DROP TABLE IF EXISTS test.person_write')
    session.execute('DROP KEYSPACE IF EXISTS test', timeout=10)


class CassandraBase(object):
    """
    Needs a running Cassandra
    """
    TEST_QUERY = "SELECT * from test.person WHERE name = 'Cassandra'"
    TEST_QUERY_PAGINATED = 'SELECT * from test.person'
    TEST_KEYSPACE = 'test'
    TEST_PORT = CASSANDRA_CONFIG['port']
    TEST_SERVICE = 'test-cassandra'

    def _traced_session(self):
        # implement me
        pass

    @contextlib.contextmanager
    def override_config(self, integration, values):
        """
        Temporarily override an integration configuration value
        >>> with self.override_config('flask', dict(service_name='test-service')):
        ... # Your test
        """
        options = getattr(config, integration)

        original = dict(
            (key, options.get(key))
            for key in values.keys()
        )

        options.update(values)
        try:
            yield
        finally:
            options.update(original)

    def setUp(self):
        self.cluster = Cluster(port=CASSANDRA_CONFIG['port'])
        self.session = self.cluster.connect()

    def _assert_result_correct(self, result):
        assert len(result.current_rows) == 1
        for r in result:
            assert r.name == 'Cassandra'
            assert r.age == 100
            assert r.description == 'A cruel mistress'

    def _test_query_base(self, execute_fn):
        session, tracer = self._traced_session()
        writer = tracer.writer
        result = execute_fn(session, self.TEST_QUERY)
        self._assert_result_correct(result)

        spans = writer.pop()
        assert spans, spans

        # another for the actual query
        assert len(spans) == 1

        query = spans[0]
        assert query.service == self.TEST_SERVICE
        assert query.resource == self.TEST_QUERY
        assert query.span_type == 'cassandra'

        assert query.get_tag(cassx.KEYSPACE) == self.TEST_KEYSPACE
        assert query.get_metric(net.TARGET_PORT) == self.TEST_PORT
        assert query.get_metric(cassx.ROW_COUNT) == 1
        assert query.get_tag(cassx.PAGE_NUMBER) is None
        assert query.get_tag(cassx.PAGINATED) == 'False'
        assert query.get_tag(net.TARGET_HOST) == '127.0.0.1'

        # confirm no analytics sample rate set by default
        assert query.get_metric(ANALYTICS_SAMPLE_RATE_KEY) is None

    def test_query(self):
        def execute_fn(session, query):
            return session.execute(query)
        self._test_query_base(execute_fn)

    def test_query_analytics_with_rate(self):
        with self.override_config(
                'cassandra',
                dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            session, tracer = self._traced_session()
            session.execute(self.TEST_QUERY)

            writer = tracer.writer
            spans = writer.pop()
            assert spans, spans
            # another for the actual query
            assert len(spans) == 1
            query = spans[0]
            # confirm no analytics sample rate set by default
            assert query.get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 0.5

    def test_query_analytics_without_rate(self):
        with self.override_config(
                'cassandra',
                dict(analytics_enabled=True)
        ):
            session, tracer = self._traced_session()
            session.execute(self.TEST_QUERY)

            writer = tracer.writer
            spans = writer.pop()
            assert spans, spans
            # another for the actual query
            assert len(spans) == 1
            query = spans[0]
            # confirm no analytics sample rate set by default
            assert query.get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 1.0

    def test_query_ot(self):
        """Ensure that cassandra works with the opentracer."""
        def execute_fn(session, query):
            return session.execute(query)

        session, tracer = self._traced_session()
        ot_tracer = init_tracer('cass_svc', tracer)
        writer = tracer.writer

        with ot_tracer.start_active_span('cass_op'):
            result = execute_fn(session, self.TEST_QUERY)
            self._assert_result_correct(result)

        spans = writer.pop()
        assert spans, spans

        # another for the actual query
        assert len(spans) == 2
        ot_span, dd_span = spans

        # confirm parenting
        assert ot_span.parent_id is None
        assert dd_span.parent_id == ot_span.span_id

        assert ot_span.name == 'cass_op'
        assert ot_span.service == 'cass_svc'

        assert dd_span.service == self.TEST_SERVICE
        assert dd_span.resource == self.TEST_QUERY
        assert dd_span.span_type == 'cassandra'

        assert dd_span.get_tag(cassx.KEYSPACE) == self.TEST_KEYSPACE
        assert dd_span.get_metric(net.TARGET_PORT) == self.TEST_PORT
        assert dd_span.get_metric(cassx.ROW_COUNT) == 1
        assert dd_span.get_tag(cassx.PAGE_NUMBER) is None
        assert dd_span.get_tag(cassx.PAGINATED) == 'False'
        assert dd_span.get_tag(net.TARGET_HOST) == '127.0.0.1'

    def test_query_async(self):
        def execute_fn(session, query):
            event = Event()
            result = []
            future = session.execute_async(query)

            def callback(results):
                result.append(ResultSet(future, results))
                event.set()

            future.add_callback(callback)
            event.wait()
            return result[0]
        self._test_query_base(execute_fn)

    def test_query_async_clearing_callbacks(self):
        def execute_fn(session, query):
            future = session.execute_async(query)
            future.clear_callbacks()
            return future.result()
        self._test_query_base(execute_fn)

    def test_span_is_removed_from_future(self):
        session, tracer = self._traced_session()
        future = session.execute_async(self.TEST_QUERY)
        future.result()
        span = getattr(future, '_ddtrace_current_span', None)
        assert span is None

    def test_paginated_query(self):
        session, tracer = self._traced_session()
        writer = tracer.writer
        statement = SimpleStatement(self.TEST_QUERY_PAGINATED, fetch_size=1)
        result = session.execute(statement)
        # iterate over all pages
        results = list(result)
        assert len(results) == 3

        spans = writer.pop()
        assert spans, spans

        # There are 4 spans for 3 results since the driver makes a request with
        # no result to check that it has reached the last page
        assert len(spans) == 4

        for i in range(4):
            query = spans[i]
            assert query.service == self.TEST_SERVICE
            assert query.resource == self.TEST_QUERY_PAGINATED
            assert query.span_type == 'cassandra'

            assert query.get_tag(cassx.KEYSPACE) == self.TEST_KEYSPACE
            assert query.get_metric(net.TARGET_PORT) == self.TEST_PORT
            if i == 3:
                assert query.get_metric(cassx.ROW_COUNT) == 0
            else:
                assert query.get_metric(cassx.ROW_COUNT) == 1
            assert query.get_tag(net.TARGET_HOST) == '127.0.0.1'
            assert query.get_tag(cassx.PAGINATED) == 'True'
            assert query.get_metric(cassx.PAGE_NUMBER) == i + 1

    def test_trace_with_service(self):
        session, tracer = self._traced_session()
        writer = tracer.writer
        session.execute(self.TEST_QUERY)
        spans = writer.pop()
        assert spans
        assert len(spans) == 1
        query = spans[0]
        assert query.service == self.TEST_SERVICE

    def test_trace_error(self):
        session, tracer = self._traced_session()
        writer = tracer.writer

        try:
            session.execute('select * from test.i_dont_exist limit 1')
        except Exception:
            pass
        else:
            assert 0

        spans = writer.pop()
        assert spans
        query = spans[0]
        assert query.error == 1
        for k in (errors.ERROR_MSG, errors.ERROR_TYPE):
            assert query.get_tag(k)

    def test_bound_statement(self):
        session, tracer = self._traced_session()
        writer = tracer.writer

        query = 'INSERT INTO test.person_write (name, age, description) VALUES (?, ?, ?)'
        prepared = session.prepare(query)
        session.execute(prepared, ('matt', 34, 'can'))

        prepared = session.prepare(query)
        bound_stmt = prepared.bind(('leo', 16, 'fr'))
        session.execute(bound_stmt)

        spans = writer.pop()
        assert len(spans) == 2
        for s in spans:
            assert s.resource == query

    def test_batch_statement(self):
        session, tracer = self._traced_session()
        writer = tracer.writer

        batch = BatchStatement()
        batch.add(
            SimpleStatement('INSERT INTO test.person_write (name, age, description) VALUES (%s, %s, %s)'),
            ('Joe', 1, 'a'),
        )
        batch.add(
            SimpleStatement('INSERT INTO test.person_write (name, age, description) VALUES (%s, %s, %s)'),
            ('Jane', 2, 'b'),
        )
        session.execute(batch)

        spans = writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.resource == 'BatchStatement'
        assert s.get_metric('cassandra.batch_size') == 2
        assert 'test.person' in s.get_tag('cassandra.query')

    def test_batched_bound_statement(self):
        session, tracer = self._traced_session()
        writer = tracer.writer

        batch = BatchStatement()

        prepared_statement = session.prepare('INSERT INTO test.person_write (name, age, description) VALUES (?, ?, ?)')
        batch.add(
            prepared_statement.bind(('matt', 34, 'can'))
        )
        session.execute(batch)

        spans = writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.resource == 'BatchStatement'
        assert s.get_tag('cassandra.query') == ''


class TestCassPatchDefault(unittest.TestCase, CassandraBase):
    """Test Cassandra instrumentation with patching and default configuration"""

    TEST_SERVICE = SERVICE

    def tearDown(self):
        unpatch()

    def setUp(self):
        CassandraBase.setUp(self)
        patch()

    def _traced_session(self):
        tracer = get_dummy_tracer()
        Pin.get_from(self.cluster).clone(tracer=tracer).onto(self.cluster)
        return self.cluster.connect(self.TEST_KEYSPACE), tracer


class TestCassPatchAll(TestCassPatchDefault):
    """Test Cassandra instrumentation with patching and custom service on all clusters"""

    TEST_SERVICE = 'test-cassandra-patch-all'

    def tearDown(self):
        unpatch()

    def setUp(self):
        CassandraBase.setUp(self)
        patch()

    def _traced_session(self):
        tracer = get_dummy_tracer()
        # pin the global Cluster to test if they will conflict
        Pin(service=self.TEST_SERVICE, tracer=tracer).onto(Cluster)
        self.cluster = Cluster(port=CASSANDRA_CONFIG['port'])

        return self.cluster.connect(self.TEST_KEYSPACE), tracer


class TestCassPatchOne(TestCassPatchDefault):
    """Test Cassandra instrumentation with patching and custom service on one cluster"""

    TEST_SERVICE = 'test-cassandra-patch-one'

    def tearDown(self):
        unpatch()

    def setUp(self):
        CassandraBase.setUp(self)
        patch()

    def _traced_session(self):
        tracer = get_dummy_tracer()
        # pin the global Cluster to test if they will conflict
        Pin(service='not-%s' % self.TEST_SERVICE).onto(Cluster)
        self.cluster = Cluster(port=CASSANDRA_CONFIG['port'])

        Pin(service=self.TEST_SERVICE, tracer=tracer).onto(self.cluster)
        return self.cluster.connect(self.TEST_KEYSPACE), tracer

    def test_patch_unpatch(self):
        # Test patch idempotence
        patch()
        patch()

        tracer = get_dummy_tracer()
        Pin.get_from(Cluster).clone(tracer=tracer).onto(Cluster)

        session = Cluster(port=CASSANDRA_CONFIG['port']).connect(self.TEST_KEYSPACE)
        session.execute(self.TEST_QUERY)

        spans = tracer.writer.pop()
        assert spans, spans
        assert len(spans) == 1

        # Test unpatch
        unpatch()

        session = Cluster(port=CASSANDRA_CONFIG['port']).connect(self.TEST_KEYSPACE)
        session.execute(self.TEST_QUERY)

        spans = tracer.writer.pop()
        assert not spans, spans

        # Test patch again
        patch()
        Pin.get_from(Cluster).clone(tracer=tracer).onto(Cluster)

        session = Cluster(port=CASSANDRA_CONFIG['port']).connect(self.TEST_KEYSPACE)
        session.execute(self.TEST_QUERY)

        spans = tracer.writer.pop()
        assert spans, spans


def test_backwards_compat_get_traced_cassandra():
    cluster = get_traced_cassandra()
    session = cluster(port=CASSANDRA_CONFIG['port']).connect()
    session.execute('drop table if exists test.person')
