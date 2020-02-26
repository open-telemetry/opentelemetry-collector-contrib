# 3rd party
from django.test import modify_settings
from django.db import connections

# project
from ddtrace import config
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY, SAMPLING_PRIORITY_KEY
from ddtrace.contrib.django.db import unpatch_conn
from ddtrace.ext import errors, http

# testing
from tests.opentracer.utils import init_tracer
from .compat import reverse
from .utils import DjangoTraceTestCase, override_ddtrace_settings
from ...utils import assert_span_http_status_code


class DjangoMiddlewareTest(DjangoTraceTestCase):
    """
    Ensures that the middleware traces all Django internals
    """
    def test_middleware_trace_request(self, query_string=''):
        # ensures that the internals are properly traced
        url = reverse('users-list')
        if query_string:
            fqs = '?' + query_string
        else:
            fqs = ''
        response = self.client.get(url + fqs)
        assert response.status_code == 200

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        sp_request = spans[0]
        sp_template = spans[1]
        sp_database = spans[2]
        assert sp_database.get_tag('django.db.vendor') == 'sqlite'
        assert sp_template.get_tag('django.template_name') == 'users_list.html'
        assert_span_http_status_code(sp_request, 200)
        assert sp_request.get_tag(http.URL) == 'http://testserver/users/'
        assert sp_request.get_tag('django.user.is_authenticated') == 'False'
        assert sp_request.get_tag('http.method') == 'GET'
        assert sp_request.span_type == 'web'
        assert sp_request.resource == 'tests.contrib.django.app.views.UserList'
        if config.django.trace_query_string:
            assert sp_request.get_tag(http.QUERY_STRING) == query_string
        else:
            assert http.QUERY_STRING not in sp_request.meta

    def test_middleware_trace_request_qs(self):
        return self.test_middleware_trace_request('foo=bar')

    def test_middleware_trace_request_multi_qs(self):
        return self.test_middleware_trace_request('foo=bar&foo=baz&x=y')

    def test_middleware_trace_request_no_qs_trace(self):
        with self.override_global_config(dict(trace_query_string=True)):
            return self.test_middleware_trace_request()

    def test_middleware_trace_request_qs_trace(self):
        with self.override_global_config(dict(trace_query_string=True)):
            return self.test_middleware_trace_request('foo=bar')

    def test_middleware_trace_request_multi_qs_trace(self):
        with self.override_global_config(dict(trace_query_string=True)):
            return self.test_middleware_trace_request('foo=bar&foo=baz&x=y')

    def test_analytics_global_on_integration_default(self):
        """
        When making a request
            When an integration trace search is not event sample rate is not set and globally trace search is enabled
                We expect the root span to have the appropriate tag
        """
        with self.override_global_config(dict(analytics_enabled=True)):
            url = reverse('users-list')
            response = self.client.get(url)
            self.assertEqual(response.status_code, 200)

        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        sp_request = spans[0]
        sp_template = spans[1]
        sp_database = spans[2]
        self.assertEqual(sp_request.name, 'django.request')
        self.assertEqual(sp_request.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 1.0)
        self.assertIsNone(sp_template.get_metric(ANALYTICS_SAMPLE_RATE_KEY))
        self.assertIsNone(sp_database.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    @override_ddtrace_settings(ANALYTICS_ENABLED=True, ANALYTICS_SAMPLE_RATE=0.5)
    def test_analytics_global_on_integration_on(self):
        """
        When making a request
            When an integration trace search is enabled and sample rate is set and globally trace search is enabled
                We expect the root span to have the appropriate tag
        """
        with self.override_global_config(dict(analytics_enabled=True)):
            url = reverse('users-list')
            response = self.client.get(url)
            self.assertEqual(response.status_code, 200)

        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        sp_request = spans[0]
        sp_template = spans[1]
        sp_database = spans[2]
        self.assertEqual(sp_request.name, 'django.request')
        self.assertEqual(sp_request.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)
        self.assertIsNone(sp_template.get_metric(ANALYTICS_SAMPLE_RATE_KEY))
        self.assertIsNone(sp_database.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_global_off_integration_default(self):
        """
        When making a request
            When an integration trace search is not set and sample rate is set and globally trace search is disabled
                We expect the root span to not include tag
        """
        with self.override_global_config(dict(analytics_enabled=False)):
            url = reverse('users-list')
            response = self.client.get(url)
            self.assertEqual(response.status_code, 200)

        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        sp_request = spans[0]
        sp_template = spans[1]
        sp_database = spans[2]
        self.assertEqual(sp_request.name, 'django.request')
        self.assertIsNone(sp_request.get_metric(ANALYTICS_SAMPLE_RATE_KEY))
        self.assertIsNone(sp_template.get_metric(ANALYTICS_SAMPLE_RATE_KEY))
        self.assertIsNone(sp_database.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    @override_ddtrace_settings(ANALYTICS_ENABLED=True, ANALYTICS_SAMPLE_RATE=0.5)
    def test_analytics_global_off_integration_on(self):
        """
        When making a request
            When an integration trace search is enabled and sample rate is set and globally trace search is disabled
                We expect the root span to have the appropriate tag
        """
        with self.override_global_config(dict(analytics_enabled=False)):
            url = reverse('users-list')
            response = self.client.get(url)
            self.assertEqual(response.status_code, 200)

        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        sp_request = spans[0]
        sp_template = spans[1]
        sp_database = spans[2]
        self.assertEqual(sp_request.name, 'django.request')
        self.assertEqual(sp_request.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)
        self.assertIsNone(sp_template.get_metric(ANALYTICS_SAMPLE_RATE_KEY))
        self.assertIsNone(sp_database.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    @override_ddtrace_settings(ANALYTICS_ENABLED=True, ANALYTICS_SAMPLE_RATE=None)
    def test_analytics_global_off_integration_on_and_none(self):
        """
        When making a request
        When an integration trace search is enabled
        Sample rate is set to None
        Globally trace search is disabled
        We expect the root span to have the appropriate tag
        """
        with self.override_global_config(dict(analytics_enabled=False)):
            url = reverse('users-list')
            response = self.client.get(url)
            self.assertEqual(response.status_code, 200)

        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        sp_request = spans[0]
        sp_template = spans[1]
        sp_database = spans[2]
        self.assertEqual(sp_request.name, 'django.request')
        assert sp_request.get_metric(ANALYTICS_SAMPLE_RATE_KEY) is None
        assert sp_template.get_metric(ANALYTICS_SAMPLE_RATE_KEY) is None
        assert sp_database.get_metric(ANALYTICS_SAMPLE_RATE_KEY) is None

    def test_database_patch(self):
        # We want to test that a connection-recreation event causes connections
        # to get repatched. However since django tests are a atomic transaction
        # we can't change the connection. Instead we test that the connection
        # does get repatched if it's not patched.
        for conn in connections.all():
            unpatch_conn(conn)
        # ensures that the internals are properly traced
        url = reverse('users-list')
        response = self.client.get(url)
        assert response.status_code == 200

        # We would be missing span #3, the database span, if the connection
        # wasn't patched.
        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        assert spans[0].name == 'django.request'
        assert spans[1].name == 'django.template'
        assert spans[2].name == 'sqlite.query'

    def test_middleware_trace_errors(self):
        # ensures that the internals are properly traced
        url = reverse('forbidden-view')
        response = self.client.get(url)
        assert response.status_code == 403

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert_span_http_status_code(span, 403)
        assert span.get_tag(http.URL) == 'http://testserver/fail-view/'
        assert span.resource == 'tests.contrib.django.app.views.ForbiddenView'

    def test_middleware_trace_function_based_view(self):
        # ensures that the internals are properly traced when using a function views
        url = reverse('fn-view')
        response = self.client.get(url)
        assert response.status_code == 200

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert_span_http_status_code(span, 200)
        assert span.get_tag(http.URL) == 'http://testserver/fn-view/'
        assert span.resource == 'tests.contrib.django.app.views.function_view'

    def test_middleware_trace_error_500(self):
        # ensures we trace exceptions generated by views
        url = reverse('error-500')
        response = self.client.get(url)
        assert response.status_code == 500

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert span.error == 1
        assert_span_http_status_code(span, 500)
        assert span.get_tag(http.URL) == 'http://testserver/error-500/'
        assert span.resource == 'tests.contrib.django.app.views.error_500'
        assert 'Error 500' in span.get_tag('error.stack')

    def test_middleware_trace_callable_view(self):
        # ensures that the internals are properly traced when using callable views
        url = reverse('feed-view')
        response = self.client.get(url)
        assert response.status_code == 200

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert_span_http_status_code(span, 200)
        assert span.get_tag(http.URL) == 'http://testserver/feed-view/'
        assert span.resource == 'tests.contrib.django.app.views.FeedView'

    def test_middleware_trace_partial_based_view(self):
        # ensures that the internals are properly traced when using a function views
        url = reverse('partial-view')
        response = self.client.get(url)
        assert response.status_code == 200

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert_span_http_status_code(span, 200)
        assert span.get_tag(http.URL) == 'http://testserver/partial-view/'
        assert span.resource == 'partial'

    def test_middleware_trace_lambda_based_view(self):
        # ensures that the internals are properly traced when using a function views
        url = reverse('lambda-view')
        response = self.client.get(url)
        assert response.status_code == 200

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        span = spans[0]
        assert_span_http_status_code(span, 200)
        assert span.get_tag(http.URL) == 'http://testserver/lambda-view/'
        assert span.resource == 'tests.contrib.django.app.views.<lambda>'

    @modify_settings(
        MIDDLEWARE={
            'remove': 'django.contrib.auth.middleware.AuthenticationMiddleware',
        },
        MIDDLEWARE_CLASSES={
            'remove': 'django.contrib.auth.middleware.AuthenticationMiddleware',
        },
    )
    def test_middleware_without_user(self):
        # remove the AuthenticationMiddleware so that the ``request``
        # object doesn't have the ``user`` field
        url = reverse('users-list')
        response = self.client.get(url)
        assert response.status_code == 200

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        sp_request = spans[0]
        assert_span_http_status_code(sp_request, 200)
        assert sp_request.get_tag('django.user.is_authenticated') is None

    def test_middleware_propagation(self):
        # ensures that we properly propagate http context
        url = reverse('users-list')
        headers = {
            'x-datadog-trace-id': '100',
            'x-datadog-parent-id': '42',
            'x-datadog-sampling-priority': '2',
        }
        response = self.client.get(url, **headers)
        assert response.status_code == 200

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        sp_request = spans[0]

        # Check for proper propagated attributes
        assert sp_request.trace_id == 100
        assert sp_request.parent_id == 42
        assert sp_request.get_metric(SAMPLING_PRIORITY_KEY) == 2

    @override_ddtrace_settings(DISTRIBUTED_TRACING=False)
    def test_middleware_no_propagation(self):
        # ensures that we properly propagate http context
        url = reverse('users-list')
        headers = {
            'x-datadog-trace-id': '100',
            'x-datadog-parent-id': '42',
            'x-datadog-sampling-priority': '2',
        }
        response = self.client.get(url, **headers)
        assert response.status_code == 200

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 3
        sp_request = spans[0]

        # Check that propagation didn't happen
        assert sp_request.trace_id != 100
        assert sp_request.parent_id != 42
        assert sp_request.get_metric(SAMPLING_PRIORITY_KEY) != 2

    @modify_settings(
        MIDDLEWARE={
            'append': 'tests.contrib.django.app.middlewares.HandleErrorMiddlewareSuccess',
        },
        MIDDLEWARE_CLASSES={
            'append': 'tests.contrib.django.app.middlewares.HandleErrorMiddlewareSuccess',
        },
    )
    def test_middleware_handled_view_exception_success(self):
        """ Test when an exception is raised in a view and then handled, that
            the resulting span does not possess error properties.
        """
        url = reverse('error-500')
        response = self.client.get(url)
        assert response.status_code == 200

        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        sp_request = spans[0]

        assert sp_request.error == 0
        assert sp_request.get_tag(errors.ERROR_STACK) is None
        assert sp_request.get_tag(errors.ERROR_MSG) is None
        assert sp_request.get_tag(errors.ERROR_TYPE) is None

    @modify_settings(
        MIDDLEWARE={
            'append': 'tests.contrib.django.app.middlewares.HandleErrorMiddlewareClientError',
        },
        MIDDLEWARE_CLASSES={
            'append': 'tests.contrib.django.app.middlewares.HandleErrorMiddlewareClientError',
        },
    )
    def test_middleware_handled_view_exception_client_error(self):
        """ Test the case that when an exception is raised in a view and then
            handled, that the resulting span does not possess error properties.
        """
        url = reverse('error-500')
        response = self.client.get(url)
        assert response.status_code == 404

        spans = self.tracer.writer.pop()
        assert len(spans) == 1

        sp_request = spans[0]

        assert sp_request.error == 0
        assert sp_request.get_tag(errors.ERROR_STACK) is None
        assert sp_request.get_tag(errors.ERROR_MSG) is None
        assert sp_request.get_tag(errors.ERROR_TYPE) is None

    def test_middleware_trace_request_ot(self):
        """OpenTracing version of test_middleware_trace_request."""
        ot_tracer = init_tracer('my_svc', self.tracer)

        # ensures that the internals are properly traced
        url = reverse('users-list')
        with ot_tracer.start_active_span('ot_span'):
            response = self.client.get(url)
        assert response.status_code == 200

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 4
        ot_span = spans[0]
        sp_request = spans[1]
        sp_template = spans[2]
        sp_database = spans[3]

        # confirm parenting
        assert ot_span.parent_id is None
        assert sp_request.parent_id == ot_span.span_id

        assert ot_span.resource == 'ot_span'
        assert ot_span.service == 'my_svc'

        assert sp_database.get_tag('django.db.vendor') == 'sqlite'
        assert sp_template.get_tag('django.template_name') == 'users_list.html'
        assert_span_http_status_code(sp_request, 200)
        assert sp_request.get_tag(http.URL) == 'http://testserver/users/'
        assert sp_request.get_tag('django.user.is_authenticated') == 'False'
        assert sp_request.get_tag('http.method') == 'GET'

    def test_middleware_trace_request_404(self):
        """
        When making a request to an unknown url in django
            when we do not have a 404 view handler set
                we set a resource name for the default view handler
        """
        response = self.client.get('/unknown-url')
        assert response.status_code == 404

        # check for spans
        spans = self.tracer.writer.pop()
        assert len(spans) == 2
        sp_request = spans[0]
        sp_template = spans[1]

        # Template
        # DEV: The template name is `unknown` because unless they define a `404.html`
        #   django generates the template from a string, which will not have a `Template.name` set
        assert sp_template.get_tag('django.template_name') == 'unknown'

        # Request
        assert_span_http_status_code(sp_request, 404)
        assert sp_request.get_tag(http.URL) == 'http://testserver/unknown-url'
        assert sp_request.get_tag('django.user.is_authenticated') == 'False'
        assert sp_request.get_tag('http.method') == 'GET'
        assert sp_request.span_type == 'web'
        assert sp_request.resource == 'django.views.defaults.page_not_found'
