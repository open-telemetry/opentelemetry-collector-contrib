import mock
import time

# 3rd party
from django.contrib.auth.models import User

from ddtrace.contrib.django.conf import settings
from ddtrace.contrib.django.patch import apply_django_patches, connections

# testing
from .utils import DjangoTraceTestCase, override_ddtrace_settings


class DjangoConnectionTest(DjangoTraceTestCase):
    """
    Ensures that database connections are properly traced
    """
    def test_connection(self):
        # trace a simple query
        start = time.time()
        users = User.objects.count()
        assert users == 0
        end = time.time()

        # tests
        spans = self.tracer.writer.pop()
        assert spans, spans
        assert len(spans) == 1

        span = spans[0]
        assert span.name == 'sqlite.query'
        assert span.service == 'defaultdb'
        assert span.span_type == 'sql'
        assert span.get_tag('django.db.vendor') == 'sqlite'
        assert span.get_tag('django.db.alias') == 'default'
        assert start < span.start < span.start + span.duration < end

    def test_django_db_query_in_resource_not_in_tags(self):
        User.objects.count()
        spans = self.tracer.writer.pop()
        assert spans[0].name == 'sqlite.query'
        assert spans[0].resource == 'SELECT COUNT(*) AS "__count" FROM "auth_user"'
        assert spans[0].get_tag('sql.query') is None

    @override_ddtrace_settings(INSTRUMENT_DATABASE=False)
    def test_connection_disabled(self):
        # trace a simple query
        users = User.objects.count()
        assert users == 0

        # tests
        spans = self.tracer.writer.pop()
        assert len(spans) == 0

    def test_should_append_database_prefix(self):
        # trace a simple query and check if the prefix is correctly
        # loaded from Django settings
        settings.DEFAULT_DATABASE_PREFIX = 'my_prefix_db'
        User.objects.count()

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.service == 'my_prefix_db-defaultdb'

    def test_apply_django_patches_calls_connections_all(self):
        with mock.patch.object(connections, 'all') as mock_connections:
            apply_django_patches(patch_rest_framework=False)

        assert mock_connections.call_count == 1
        assert mock_connections.mock_calls == [mock.call()]
