# 3p
import pytest
from ddtrace.vendor import wrapt

# project
import ddtrace
from ddtrace import Pin, config
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.vertica.patch import patch, unpatch
from ddtrace.ext import errors
from ddtrace.utils.merge import deepmerge

# testing
from tests.base import BaseTracerTestCase
from tests.contrib.config import VERTICA_CONFIG
from tests.opentracer.utils import init_tracer
from tests.test_tracer import get_dummy_tracer

TEST_TABLE = 'test_table'


@pytest.fixture(scope='function')
def test_tracer(request):
    request.cls.test_tracer = get_dummy_tracer()
    return request.cls.test_tracer


@pytest.fixture(scope='function')
def test_conn(request, test_tracer):
    ddtrace.tracer = test_tracer
    patch()

    import vertica_python  # must happen AFTER installing with patch()

    conn = vertica_python.connect(**VERTICA_CONFIG)

    cur = conn.cursor()
    cur.execute('DROP TABLE IF EXISTS {}'.format(TEST_TABLE))
    cur.execute(
        """CREATE TABLE {} (
        a INT,
        b VARCHAR(32)
        )
        """.format(
            TEST_TABLE
        )
    )
    test_tracer.writer.pop()

    request.cls.test_conn = (conn, cur)
    return conn, cur


class TestVerticaPatching(BaseTracerTestCase):
    def tearDown(self):
        super(TestVerticaPatching, self).tearDown()
        unpatch()

    def test_not_patched(self):
        """Ensure that vertica is not patched somewhere before our tests."""
        import vertica_python

        assert not isinstance(vertica_python.Connection.cursor, wrapt.ObjectProxy)
        assert not isinstance(
            vertica_python.vertica.cursor.Cursor.execute, wrapt.ObjectProxy
        )

    def test_patch_after_import(self):
        """Patching _after_ the import will not work because we hook into
        the module import system.

        Vertica uses a local reference to `Cursor` which won't get patched
        if we call `patch` after the module has already been imported.
        """
        import vertica_python

        assert not isinstance(vertica_python.vertica.connection.Connection.cursor, wrapt.ObjectProxy)
        assert not isinstance(
            vertica_python.vertica.cursor.Cursor.execute, wrapt.ObjectProxy
        )

        patch()

        conn = vertica_python.connect(**VERTICA_CONFIG)
        cursor = conn.cursor()
        assert not isinstance(cursor, wrapt.ObjectProxy)

    def test_patch_before_import(self):
        patch()
        import vertica_python

        # use a patched method from each class as indicators
        assert isinstance(vertica_python.Connection.cursor, wrapt.ObjectProxy)
        assert isinstance(
            vertica_python.vertica.cursor.Cursor.execute, wrapt.ObjectProxy
        )

    def test_idempotent_patch(self):
        patch()
        patch()
        import vertica_python

        assert not isinstance(
            vertica_python.Connection.cursor.__wrapped__, wrapt.ObjectProxy
        )
        assert not isinstance(
            vertica_python.vertica.cursor.Cursor.execute.__wrapped__, wrapt.ObjectProxy
        )
        assert isinstance(vertica_python.Connection.cursor, wrapt.ObjectProxy)
        assert isinstance(
            vertica_python.vertica.cursor.Cursor.execute, wrapt.ObjectProxy
        )

    def test_unpatch_before_import(self):
        patch()
        unpatch()
        import vertica_python

        assert not isinstance(vertica_python.Connection.cursor, wrapt.ObjectProxy)
        assert not isinstance(
            vertica_python.vertica.cursor.Cursor.execute, wrapt.ObjectProxy
        )

    def test_unpatch_after_import(self):
        patch()
        import vertica_python

        unpatch()
        assert not isinstance(vertica_python.Connection.cursor, wrapt.ObjectProxy)
        assert not isinstance(
            vertica_python.vertica.cursor.Cursor.execute, wrapt.ObjectProxy
        )


@pytest.mark.usefixtures('test_tracer', 'test_conn')
class TestVertica(BaseTracerTestCase):
    def tearDown(self):
        super(TestVertica, self).tearDown()

        unpatch()

    def test_configuration_service_name(self):
        """Ensure that the integration can be configured."""
        with self.override_config('vertica', dict(service_name='test_svc_name')):
            patch()
            import vertica_python

            test_tracer = get_dummy_tracer()

            conn = vertica_python.connect(**VERTICA_CONFIG)
            cur = conn.cursor()
            Pin.override(cur, tracer=test_tracer)
            with conn:
                cur.execute('DROP TABLE IF EXISTS {}'.format(TEST_TABLE))
        spans = test_tracer.writer.pop()
        assert len(spans) == 1
        assert spans[0].service == 'test_svc_name'

    def test_configuration_routine(self):
        """Ensure that the integration routines can be configured."""
        routine_config = dict(
            patch={
                'vertica_python.vertica.connection.Connection': dict(
                    routines=dict(
                        cursor=dict(
                            operation_name='get_cursor',
                            trace_enabled=True,
                        ),
                    ),
                ),
            },
        )

        # Make a copy of the vertica config first before we merge our settings over
        # DEV: First argument gets merged into the second
        copy = deepmerge(config.vertica, dict())
        overrides = deepmerge(routine_config, copy)
        with self.override_config('vertica', overrides):
            patch()
            import vertica_python

            test_tracer = get_dummy_tracer()

            conn = vertica_python.connect(**VERTICA_CONFIG)
            Pin.override(conn, service='mycustomservice', tracer=test_tracer)
            conn.cursor()  # should be traced now
            conn.close()
        spans = test_tracer.writer.pop()
        assert len(spans) == 1
        assert spans[0].name == 'get_cursor'
        assert spans[0].service == 'mycustomservice'

    def test_execute_metadata(self):
        """Metadata related to an `execute` call should be captured."""
        conn, cur = self.test_conn

        Pin.override(cur, tracer=self.test_tracer)

        with conn:
            cur.execute("INSERT INTO {} (a, b) VALUES (1, 'aa');".format(TEST_TABLE))
            cur.execute('SELECT * FROM {};'.format(TEST_TABLE))

        spans = self.test_tracer.writer.pop()
        assert len(spans) == 2

        # check all the metadata
        assert spans[0].service == 'vertica'
        assert spans[0].span_type == 'sql'
        assert spans[0].name == 'vertica.query'
        assert spans[0].get_metric('db.rowcount') == -1
        query = "INSERT INTO test_table (a, b) VALUES (1, 'aa');"
        assert spans[0].resource == query
        assert spans[0].get_tag('out.host') == '127.0.0.1'
        assert spans[0].get_metric('out.port') == 5433
        assert spans[0].get_tag('db.name') == 'docker'
        assert spans[0].get_tag('db.user') == 'dbadmin'

        assert spans[1].resource == 'SELECT * FROM test_table;'

    def test_cursor_override(self):
        """Test overriding the tracer with our own."""
        conn, cur = self.test_conn

        Pin.override(cur, tracer=self.test_tracer)

        with conn:
            cur.execute("INSERT INTO {} (a, b) VALUES (1, 'aa');".format(TEST_TABLE))
            cur.execute('SELECT * FROM {};'.format(TEST_TABLE))

        spans = self.test_tracer.writer.pop()
        assert len(spans) == 2

        # check all the metadata
        assert spans[0].service == 'vertica'
        assert spans[0].span_type == 'sql'
        assert spans[0].name == 'vertica.query'
        assert spans[0].get_metric('db.rowcount') == -1
        query = "INSERT INTO test_table (a, b) VALUES (1, 'aa');"
        assert spans[0].resource == query
        assert spans[0].get_tag('out.host') == '127.0.0.1'
        assert spans[0].get_metric('out.port') == 5433

        assert spans[1].resource == 'SELECT * FROM test_table;'

    def test_execute_exception(self):
        """Exceptions should result in appropriate span tagging."""
        from vertica_python.errors import VerticaSyntaxError

        conn, cur = self.test_conn

        with conn, pytest.raises(VerticaSyntaxError):
            cur.execute('INVALID QUERY')

        spans = self.test_tracer.writer.pop()
        assert len(spans) == 2

        # check all the metadata
        assert spans[0].service == 'vertica'
        assert spans[0].error == 1
        assert 'INVALID QUERY' in spans[0].get_tag(errors.ERROR_MSG)
        error_type = 'vertica_python.errors.VerticaSyntaxError'
        assert spans[0].get_tag(errors.ERROR_TYPE) == error_type
        assert spans[0].get_tag(errors.ERROR_STACK)

        assert spans[1].resource == 'COMMIT;'

    def test_rowcount_oddity(self):
        """Vertica treats rowcount specially. Ensure we handle it.

        See https://github.com/vertica/vertica-python/tree/029a65a862da893e7bd641a68f772019fd9ecc99#rowcount-oddities
        """
        conn, cur = self.test_conn

        with conn:
            cur.execute(
                """
                INSERT INTO {} (a, b)
                SELECT 1, 'a'
                UNION ALL
                SELECT 2, 'b'
                UNION ALL
                SELECT 3, 'c'
                UNION ALL
                SELECT 4, 'd'
                UNION ALL
                SELECT 5, 'e'
                """.format(
                    TEST_TABLE
                )
            )
            assert cur.rowcount == -1

            cur.execute('SELECT * FROM {};'.format(TEST_TABLE))
            cur.fetchone()
            cur.rowcount == 1
            cur.fetchone()
            cur.rowcount == 2
            # fetchall just calls fetchone for each remaining row
            cur.fetchall()
            cur.rowcount == 5

        spans = self.test_tracer.writer.pop()
        assert len(spans) == 9

        # check all the rowcounts
        assert spans[0].name == 'vertica.query'
        assert spans[1].get_metric('db.rowcount') == -1
        assert spans[1].name == 'vertica.query'
        assert spans[1].get_metric('db.rowcount') == -1
        assert spans[2].name == 'vertica.fetchone'
        assert spans[2].get_tag('out.host') == '127.0.0.1'
        assert spans[2].get_metric('out.port') == 5433
        assert spans[2].get_metric('db.rowcount') == 1
        assert spans[3].name == 'vertica.fetchone'
        assert spans[3].get_metric('db.rowcount') == 2
        assert spans[4].name == 'vertica.fetchall'
        assert spans[4].get_metric('db.rowcount') == 5

    def test_nextset(self):
        """cursor.nextset() should be traced."""
        conn, cur = self.test_conn

        with conn:
            cur.execute('SELECT * FROM {0}; SELECT * FROM {0}'.format(TEST_TABLE))
            cur.nextset()

        spans = self.test_tracer.writer.pop()
        assert len(spans) == 3

        # check all the rowcounts
        assert spans[0].name == 'vertica.query'
        assert spans[1].get_metric('db.rowcount') == -1
        assert spans[1].name == 'vertica.nextset'
        assert spans[1].get_metric('db.rowcount') == -1
        assert spans[2].name == 'vertica.query'
        assert spans[2].resource == 'COMMIT;'

    def test_copy(self):
        """cursor.copy() should be traced."""
        conn, cur = self.test_conn

        with conn:
            cur.copy(
                "COPY {0} (a, b) FROM STDIN DELIMITER ','".format(TEST_TABLE),
                '1,foo\n2,bar',
            )

        spans = self.test_tracer.writer.pop()
        assert len(spans) == 2

        # check all the rowcounts
        assert spans[0].name == 'vertica.copy'
        query = "COPY test_table (a, b) FROM STDIN DELIMITER ','"
        assert spans[0].resource == query
        assert spans[1].name == 'vertica.query'
        assert spans[1].resource == 'COMMIT;'

    def test_opentracing(self):
        """Ensure OpenTracing works with vertica."""
        conn, cur = self.test_conn

        ot_tracer = init_tracer('vertica_svc', self.test_tracer)

        with ot_tracer.start_active_span('vertica_execute'):
            cur.execute("INSERT INTO {} (a, b) VALUES (1, 'aa');".format(TEST_TABLE))
            conn.close()

        spans = self.test_tracer.writer.pop()
        assert len(spans) == 2
        ot_span, dd_span = spans

        # confirm the parenting
        assert ot_span.parent_id is None
        assert dd_span.parent_id == ot_span.span_id

        assert dd_span.service == 'vertica'
        assert dd_span.span_type == 'sql'
        assert dd_span.name == 'vertica.query'
        assert dd_span.get_metric('db.rowcount') == -1
        query = "INSERT INTO test_table (a, b) VALUES (1, 'aa');"
        assert dd_span.resource == query
        assert dd_span.get_tag('out.host') == '127.0.0.1'
        assert dd_span.get_metric('out.port') == 5433

    def test_analytics_default(self):
        conn, cur = self.test_conn

        Pin.override(cur, tracer=self.test_tracer)

        with conn:
            cur.execute("INSERT INTO {} (a, b) VALUES (1, 'aa');".format(TEST_TABLE))
            cur.execute('SELECT * FROM {};'.format(TEST_TABLE))

        spans = self.test_tracer.writer.pop()
        self.assertEqual(len(spans), 2)
        self.assertIsNone(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_with_rate(self):
        with self.override_config(
            'vertica',
            dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            conn, cur = self.test_conn

            Pin.override(cur, tracer=self.test_tracer)

            with conn:
                cur.execute("INSERT INTO {} (a, b) VALUES (1, 'aa');".format(TEST_TABLE))
                cur.execute('SELECT * FROM {};'.format(TEST_TABLE))

        spans = self.test_tracer.writer.pop()
        self.assertEqual(len(spans), 2)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    def test_analytics_without_rate(self):
        with self.override_config(
            'vertica',
            dict(analytics_enabled=True)
        ):
            conn, cur = self.test_conn

            Pin.override(cur, tracer=self.test_tracer)

            with conn:
                cur.execute("INSERT INTO {} (a, b) VALUES (1, 'aa');".format(TEST_TABLE))
                cur.execute('SELECT * FROM {};'.format(TEST_TABLE))

        spans = self.test_tracer.writer.pop()
        self.assertEqual(len(spans), 2)
        self.assertEqual(spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)
