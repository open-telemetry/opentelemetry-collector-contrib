# stdlib
import contextlib

# 3rd party
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker
from sqlalchemy import (
    create_engine,
    Column,
    Integer,
    String,
)

# project
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.sqlalchemy import trace_engine

# testing
from tests.opentracer.utils import init_tracer


Base = declarative_base()


class Player(Base):
    """Player entity used to test SQLAlchemy ORM"""
    __tablename__ = 'players'

    id = Column(Integer, primary_key=True)
    name = Column(String(20))


class SQLAlchemyTestMixin(object):
    """SQLAlchemy test mixin that includes a complete set of tests
    that must be executed for different engine. When a new test (or
    a regression test) should be added to SQLAlchemy test suite, a new
    entry must be appended here so that it will be executed for all
    available and supported engines. If the test is specific to only
    one engine, that test must be added to the specific `TestCase`
    implementation.

    To support a new engine, create a new `TestCase` that inherits from
    `SQLAlchemyTestMixin` and `TestCase`. Then you must define the following
    static class variables:
    * VENDOR: the database vendor name
    * SQL_DB: the `sql.db` tag that we expect (it's the name of the database available in the `.env` file)
    * SERVICE: the service that we expect by default
    * ENGINE_ARGS: all arguments required to create the engine

    To check specific tags in each test, you must implement the
    `check_meta(self, span)` method.
    """
    VENDOR = None
    SQL_DB = None
    SERVICE = None
    ENGINE_ARGS = None

    def create_engine(self, engine_args):
        # create a SQLAlchemy engine
        config = dict(engine_args)
        url = config.pop('url')
        return create_engine(url, **config)

    @contextlib.contextmanager
    def connection(self):
        # context manager that provides a connection
        # to the underlying database
        try:
            conn = self.engine.connect()
            yield conn
        finally:
            conn.close()

    def check_meta(self, span):
        # function that can be implemented according to the
        # specific engine implementation
        return

    def setUp(self):
        super(SQLAlchemyTestMixin, self).setUp()

        # create an engine with the given arguments
        self.engine = self.create_engine(self.ENGINE_ARGS)

        # create the database / entities and prepare a session for the test
        Base.metadata.drop_all(bind=self.engine)
        Base.metadata.create_all(self.engine, checkfirst=False)
        Session = sessionmaker(bind=self.engine)
        self.session = Session()

        # trace the engine
        trace_engine(self.engine, self.tracer)

    def tearDown(self):
        # clear the database and dispose the engine
        self.session.close()
        Base.metadata.drop_all(bind=self.engine)
        self.engine.dispose()
        super(SQLAlchemyTestMixin, self).tearDown()

    def test_orm_insert(self):
        # ensures that the ORM session is traced
        wayne = Player(id=1, name='wayne')
        self.session.add(wayne)
        self.session.commit()

        traces = self.tracer.writer.pop_traces()
        # trace composition
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        # span fields
        assert span.name == '{}.query'.format(self.VENDOR)
        assert span.service == self.SERVICE
        assert 'INSERT INTO players' in span.resource
        assert span.get_tag('sql.db') == self.SQL_DB
        assert span.get_metric('sql.rows') == 1
        self.check_meta(span)
        assert span.span_type == 'sql'
        assert span.error == 0
        assert span.duration > 0

    def test_session_query(self):
        # ensures that the Session queries are traced
        out = list(self.session.query(Player).filter_by(name='wayne'))
        assert len(out) == 0

        traces = self.tracer.writer.pop_traces()
        # trace composition
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        # span fields
        assert span.name == '{}.query'.format(self.VENDOR)
        assert span.service == self.SERVICE
        assert 'SELECT players.id AS players_id, players.name AS players_name \nFROM players \nWHERE players.name' \
            in span.resource
        assert span.get_tag('sql.db') == self.SQL_DB
        self.check_meta(span)
        assert span.span_type == 'sql'
        assert span.error == 0
        assert span.duration > 0

    def test_engine_connect_execute(self):
        # ensures that engine.connect() is properly traced
        with self.connection() as conn:
            rows = conn.execute('SELECT * FROM players').fetchall()
            assert len(rows) == 0

        traces = self.tracer.writer.pop_traces()
        # trace composition
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        # span fields
        assert span.name == '{}.query'.format(self.VENDOR)
        assert span.service == self.SERVICE
        assert span.resource == 'SELECT * FROM players'
        assert span.get_tag('sql.db') == self.SQL_DB
        self.check_meta(span)
        assert span.span_type == 'sql'
        assert span.error == 0
        assert span.duration > 0

    def test_traced_service(self):
        # ensures that the service is set as expected
        services = self.tracer.writer.pop_services()
        expected = {}
        assert services == expected

    def test_opentracing(self):
        """Ensure that sqlalchemy works with the opentracer."""
        ot_tracer = init_tracer('sqlalch_svc', self.tracer)

        with ot_tracer.start_active_span('sqlalch_op'):
            with self.connection() as conn:
                rows = conn.execute('SELECT * FROM players').fetchall()
                assert len(rows) == 0

        traces = self.tracer.writer.pop_traces()
        # trace composition
        assert len(traces) == 1
        assert len(traces[0]) == 2
        ot_span, dd_span = traces[0]

        # confirm the parenting
        assert ot_span.parent_id is None
        assert dd_span.parent_id == ot_span.span_id

        assert ot_span.name == 'sqlalch_op'
        assert ot_span.service == 'sqlalch_svc'

        # span fields
        assert dd_span.name == '{}.query'.format(self.VENDOR)
        assert dd_span.service == self.SERVICE
        assert dd_span.resource == 'SELECT * FROM players'
        assert dd_span.get_tag('sql.db') == self.SQL_DB
        assert dd_span.span_type == 'sql'
        assert dd_span.error == 0
        assert dd_span.duration > 0

    def test_analytics_default(self):
        # ensures that the ORM session is traced
        wayne = Player(id=1, name='wayne')
        self.session.add(wayne)
        self.session.commit()

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertIsNone(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_with_rate(self):
        with self.override_config(
            'sqlalchemy',
            dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            wayne = Player(id=1, name='wayne')
            self.session.add(wayne)
            self.session.commit()

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    def test_analytics_without_rate(self):
        with self.override_config(
            'sqlalchemy',
            dict(analytics_enabled=True)
        ):
            wayne = Player(id=1, name='wayne')
            self.session.add(wayne)
            self.session.commit()

        spans = self.get_spans()
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)
