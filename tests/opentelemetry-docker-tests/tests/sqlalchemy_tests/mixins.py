# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import contextlib
import logging
import threading

from sqlalchemy import Column, Integer, String, create_engine
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import close_all_sessions, scoped_session, sessionmaker

from opentelemetry import trace
from opentelemetry.instrumentation.sqlalchemy import SQLAlchemyInstrumentor
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase

Base = declarative_base()


def _create_engine(engine_args):
    # create a SQLAlchemy engine
    config = dict(engine_args)
    url = config.pop("url")
    return create_engine(url, **config)


class Player(Base):
    """Player entity used to test SQLAlchemy ORM"""

    __tablename__ = "players"

    id = Column(Integer, primary_key=True)
    name = Column(String(20))


class SQLAlchemyTestMixin(TestBase):
    __test__ = False

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
    * SQL_DB: the `db.type` tag that we expect (it's the name of the database available in the `.env` file)
    * SERVICE: the service that we expect by default
    * ENGINE_ARGS: all arguments required to create the engine

    To check specific tags in each test, you must implement the
    `check_meta(self, span)` method.
    """

    VENDOR = None
    SQL_DB = None
    SERVICE = None
    ENGINE_ARGS = None

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
        """function that can be implemented according to the
        specific engine implementation
        """

    def setUp(self):
        super().setUp()
        # create an engine with the given arguments
        self.engine = _create_engine(self.ENGINE_ARGS)

        # create the database / entities and prepare a session for the test
        Base.metadata.drop_all(bind=self.engine)
        Base.metadata.create_all(self.engine, checkfirst=False)
        self.session = sessionmaker(bind=self.engine)()
        # trace the engine
        SQLAlchemyInstrumentor().instrument(
            engine=self.engine, tracer_provider=self.tracer_provider
        )
        self.memory_exporter.clear()

    def tearDown(self):
        # pylint: disable=invalid-name
        # clear the database and dispose the engine
        self.session.close()
        Base.metadata.drop_all(bind=self.engine)
        self.engine.dispose()
        SQLAlchemyInstrumentor().uninstrument()
        super().tearDown()

    def _check_span(self, span, name):
        if self.SQL_DB:
            name = f"{name} {self.SQL_DB}"
        self.assertEqual(span.name, name)
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_NAME), self.SQL_DB
        )
        self.assertIs(span.status.status_code, trace.StatusCode.UNSET)
        self.assertGreater((span.end_time - span.start_time), 0)

    def test_orm_insert(self):
        # ensures that the ORM session is traced
        wayne = Player(id=1, name="wayne")
        self.session.add(wayne)
        self.session.commit()

        spans = self.memory_exporter.get_finished_spans()
        # one span for the connection and one for the query
        self.assertEqual(len(spans), 2)
        span = spans[1]
        stmt = "INSERT INTO players (id, name) VALUES "
        if span.attributes.get(SpanAttributes.DB_SYSTEM) == "sqlite":
            stmt += "(?, ?)"
        else:
            stmt += "(%(id)s, %(name)s)"
        self._check_span(span, "INSERT")
        self.assertIn(
            "INSERT INTO players",
            span.attributes.get(SpanAttributes.DB_STATEMENT),
        )
        self.check_meta(span)

    def test_session_query(self):
        # ensures that the Session queries are traced
        out = list(self.session.query(Player).filter_by(name="wayne"))
        self.assertEqual(len(out), 0)

        spans = self.memory_exporter.get_finished_spans()
        # one span for the connection and one for the query
        self.assertEqual(len(spans), 2)
        span = spans[1]
        stmt = "SELECT players.id AS players_id, players.name AS players_name \nFROM players \nWHERE players.name = "
        if span.attributes.get(SpanAttributes.DB_SYSTEM) == "sqlite":
            stmt += "?"
        else:
            stmt += "%(name_1)s"
        self._check_span(span, "SELECT")
        self.assertIn(
            "SELECT players.id AS players_id, players.name AS players_name \nFROM players \nWHERE players.name",
            span.attributes.get(SpanAttributes.DB_STATEMENT),
        )
        self.check_meta(span)

    def test_engine_connect_execute(self):
        # ensures that engine.connect() is properly traced
        stmt = "SELECT * FROM players"
        with self.connection() as conn:
            rows = conn.execute(stmt).fetchall()
            self.assertEqual(len(rows), 0)

        spans = self.memory_exporter.get_finished_spans()
        # one span for the connection and one for the query
        self.assertEqual(len(spans), 2)
        span = spans[1]
        self._check_span(span, "SELECT")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT),
            "SELECT * FROM players",
        )
        self.check_meta(span)

    def test_parent(self):
        """Ensure that sqlalchemy works with opentelemetry."""
        stmt = "SELECT * FROM players"
        tracer = self.tracer_provider.get_tracer("sqlalch_svc")

        with tracer.start_as_current_span("sqlalch_op"):
            with self.connection() as conn:
                rows = conn.execute(stmt).fetchall()
                self.assertEqual(len(rows), 0)

        spans = self.memory_exporter.get_finished_spans()
        # one span for the connection and two for the queries
        self.assertEqual(len(spans), 3)
        _, child_span, parent_span = spans

        # confirm the parenting
        self.assertIsNone(parent_span.parent)
        self.assertIs(child_span.parent, parent_span.get_span_context())

        self.assertEqual(parent_span.name, "sqlalch_op")
        self.assertEqual(parent_span.instrumentation_info.name, "sqlalch_svc")

        self.assertEqual(child_span.name, "SELECT " + self.SQL_DB)

    def test_multithreading(self):
        """Ensure spans are captured correctly in a multithreading scenario

        We also expect no logged warnings about calling end() on an ended span.
        """

        if self.VENDOR == "sqlite":
            return

        def insert_player(session):
            _session = session()
            player = Player(name="Player")
            _session.add(player)
            _session.commit()
            _session.query(Player).all()

        def insert_players(session):
            _session = session()
            players = []
            for player_number in range(3):
                players.append(Player(name=f"Player {player_number}"))
            _session.add_all(players)
            _session.commit()

        session_factory = sessionmaker(bind=self.engine)
        # pylint: disable=invalid-name
        Session = scoped_session(session_factory)
        thread_one = threading.Thread(target=insert_player, args=(Session,))
        thread_two = threading.Thread(target=insert_players, args=(Session,))

        logger = logging.getLogger("opentelemetry.sdk.trace")
        with self.assertRaises(AssertionError):
            with self.assertLogs(logger, level="WARNING"):
                thread_one.start()
                thread_two.start()
                thread_one.join()
                thread_two.join()
                close_all_sessions()

        spans = self.memory_exporter.get_finished_spans()

        # SQLAlchemy 1.4 uses the `execute_values` extension of the psycopg2 dialect to
        # batch inserts together which means `insert_players` only generates one span.
        # See https://docs.sqlalchemy.org/en/14/changelog/migration_14.html#orm-batch-inserts-with-psycopg2-now-batch-statements-with-returning-in-most-cases
        self.assertEqual(
            len(spans), 8 if self.VENDOR not in ["postgresql"] else 6
        )
