import django
from django.apps import apps
from unittest import skipIf

from tests.contrib.django.utils import DjangoTraceTestCase
from ...utils import assert_span_http_status_code


@skipIf(django.VERSION < (1, 10), 'requires django version >= 1.10')
class RestFrameworkTest(DjangoTraceTestCase):
    def setUp(self):
        super(RestFrameworkTest, self).setUp()

        # would raise an exception
        from rest_framework.views import APIView
        from ddtrace.contrib.django.restframework import unpatch_restframework

        self.APIView = APIView
        self.unpatch_restframework = unpatch_restframework

    def test_setup(self):
        assert apps.is_installed('rest_framework')
        assert hasattr(self.APIView, '_datadog_patch')

    def test_unpatch(self):
        self.unpatch_restframework()
        assert not getattr(self.APIView, '_datadog_patch')

        response = self.client.get('/users/')

        # Our custom exception handler is setting the status code to 500
        assert response.status_code == 500

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        sp = spans[0]
        assert sp.name == 'django.request'
        assert sp.resource == 'tests.contrib.djangorestframework.app.views.UserViewSet'
        assert sp.error == 0
        assert sp.span_type == 'web'
        assert_span_http_status_code(sp, 500)
        assert sp.get_tag('error.msg') is None

    def test_trace_exceptions(self):
        response = self.client.get('/users/')

        # Our custom exception handler is setting the status code to 500
        assert response.status_code == 500

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        sp = spans[0]
        assert sp.name == 'django.request'
        assert sp.resource == 'tests.contrib.djangorestframework.app.views.UserViewSet'
        assert sp.error == 1
        assert sp.span_type == 'web'
        assert sp.get_tag('http.method') == 'GET'
        assert_span_http_status_code(sp, 500)
        assert sp.get_tag('error.msg') == 'Authentication credentials were not provided.'
        assert 'NotAuthenticated' in sp.get_tag('error.stack')
