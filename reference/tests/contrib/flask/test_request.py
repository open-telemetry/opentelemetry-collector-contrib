# -*- coding: utf-8 -*-
from ddtrace.compat import PY2
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.flask.patch import flask_version
from ddtrace.ext import http
from ddtrace.propagation.http import HTTP_HEADER_TRACE_ID, HTTP_HEADER_PARENT_ID
from flask import abort

from . import BaseFlaskTestCase
from ...utils import assert_span_http_status_code


base_exception_name = 'builtins.Exception'
if PY2:
    base_exception_name = 'exceptions.Exception'


class FlaskRequestTestCase(BaseFlaskTestCase):
    def test_request(self):
        """
        When making a request
            We create the expected spans
        """
        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        res = self.client.get('/')
        self.assertEqual(res.status_code, 200)
        self.assertEqual(res.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 8)

        # Assert the order of the spans created
        self.assertListEqual(
            [
                'flask.request',
                'flask.try_trigger_before_first_request_functions',
                'flask.preprocess_request',
                'flask.dispatch_request',
                'tests.contrib.flask.test_request.index',
                'flask.process_response',
                'flask.do_teardown_request',
                'flask.do_teardown_appcontext',
            ],
            [s.name for s in spans],
        )

        # Assert span services
        for span in spans:
            self.assertEqual(span.service, 'flask')

        # Root request span
        req_span = spans[0]
        self.assertEqual(req_span.service, 'flask')
        self.assertEqual(req_span.name, 'flask.request')
        self.assertEqual(req_span.resource, 'GET /')
        self.assertEqual(req_span.span_type, 'web')
        self.assertEqual(req_span.error, 0)
        self.assertIsNone(req_span.parent_id)

        # Request tags
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'index')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/')
        self.assertEqual(req_span.get_tag('http.method'), 'GET')
        self.assertEqual(req_span.get_tag(http.URL), 'http://localhost/')
        assert_span_http_status_code(req_span, 200)
        assert http.QUERY_STRING not in req_span.meta

        # Handler span
        handler_span = spans[4]
        self.assertEqual(handler_span.service, 'flask')
        self.assertEqual(handler_span.name, 'tests.contrib.flask.test_request.index')
        self.assertEqual(handler_span.resource, '/')
        self.assertEqual(req_span.error, 0)

    def test_request_query_string_trace(self):
        """Make sure when making a request that we create the expected spans and capture the query string."""
        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        with self.override_http_config('flask', dict(trace_query_string=True)):
            self.client.get('/?foo=bar&baz=biz')
        spans = self.get_spans()

        # Request tags
        assert spans[0].get_tag(http.QUERY_STRING) == 'foo=bar&baz=biz'

    def test_analytics_global_on_integration_default(self):
        """
        When making a request
            When an integration trace search is not event sample rate is not set and globally trace search is enabled
                We expect the root span to have the appropriate tag
        """
        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        with self.override_global_config(dict(analytics_enabled=True)):
            res = self.client.get('/')
            self.assertEqual(res.status_code, 200)
            self.assertEqual(res.data, b'Hello Flask')

        root = self.get_root_span()
        root.assert_matches(
            name='flask.request',
            metrics={
                ANALYTICS_SAMPLE_RATE_KEY: 1.0,
            },
        )

        for span in self.spans:
            if span == root:
                continue
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_global_on_integration_on(self):
        """
        When making a request
            When an integration trace search is enabled and sample rate is set and globally trace search is enabled
                We expect the root span to have the appropriate tag
        """
        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        with self.override_global_config(dict(analytics_enabled=True)):
            with self.override_config('flask', dict(analytics_enabled=True, analytics_sample_rate=0.5)):
                res = self.client.get('/')
                self.assertEqual(res.status_code, 200)
                self.assertEqual(res.data, b'Hello Flask')

        root = self.get_root_span()
        root.assert_matches(
            name='flask.request',
            metrics={
                ANALYTICS_SAMPLE_RATE_KEY: 0.5,
            },
        )

        for span in self.spans:
            if span == root:
                continue
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_global_off_integration_default(self):
        """
        When making a request
            When an integration trace search is not set and sample rate is set and globally trace search is disabled
                We expect the root span to not include tag
        """
        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        with self.override_global_config(dict(analytics_enabled=False)):
            res = self.client.get('/')
            self.assertEqual(res.status_code, 200)
            self.assertEqual(res.data, b'Hello Flask')

        root = self.get_root_span()
        self.assertIsNone(root.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

        for span in self.spans:
            if span == root:
                continue
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_global_off_integration_on(self):
        """
        When making a request
            When an integration trace search is enabled and sample rate is set and globally trace search is disabled
                We expect the root span to have the appropriate tag
        """
        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        with self.override_global_config(dict(analytics_enabled=False)):
            with self.override_config('flask', dict(analytics_enabled=True, analytics_sample_rate=0.5)):
                res = self.client.get('/')
                self.assertEqual(res.status_code, 200)
                self.assertEqual(res.data, b'Hello Flask')
        root = self.get_root_span()
        root.assert_matches(
            name='flask.request',
            metrics={
                ANALYTICS_SAMPLE_RATE_KEY: 0.5,
            },
        )

        for span in self.spans:
            if span == root:
                continue
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_distributed_tracing(self):
        """
        When making a request
            When distributed tracing headers are present
                We create the expected spans
        """
        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        # Default: distributed tracing enabled
        res = self.client.get('/', headers={
            HTTP_HEADER_PARENT_ID: '12345',
            HTTP_HEADER_TRACE_ID: '678910',
        })
        self.assertEqual(res.status_code, 200)
        self.assertEqual(res.data, b'Hello Flask')

        # Assert parent and trace id are properly set on the root span
        span = self.find_span_by_name(self.get_spans(), 'flask.request')
        self.assertEqual(span.trace_id, 678910)
        self.assertEqual(span.parent_id, 12345)

        # Explicitly enable distributed tracing
        with self.override_config('flask', dict(distributed_tracing_enabled=True)):
            res = self.client.get('/', headers={
                HTTP_HEADER_PARENT_ID: '12345',
                HTTP_HEADER_TRACE_ID: '678910',
            })
            self.assertEqual(res.status_code, 200)
            self.assertEqual(res.data, b'Hello Flask')

        # Assert parent and trace id are properly set on the root span
        span = self.find_span_by_name(self.get_spans(), 'flask.request')
        self.assertEqual(span.trace_id, 678910)
        self.assertEqual(span.parent_id, 12345)

        # With distributed tracing disabled
        with self.override_config('flask', dict(distributed_tracing_enabled=False)):
            res = self.client.get('/', headers={
                HTTP_HEADER_PARENT_ID: '12345',
                HTTP_HEADER_TRACE_ID: '678910',
            })
            self.assertEqual(res.status_code, 200)
            self.assertEqual(res.data, b'Hello Flask')

        # Assert parent and trace id are properly set on the root span
        span = self.find_span_by_name(self.get_spans(), 'flask.request')
        self.assertNotEqual(span.trace_id, 678910)
        self.assertIsNone(span.parent_id)

    def test_request_query_string(self):
        """
        When making a request
            When the request contains a query string
                We create the expected spans
        """
        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        res = self.client.get('/', query_string=dict(hello='flask'))
        self.assertEqual(res.status_code, 200)
        self.assertEqual(res.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 8)

        # Assert the order of the spans created
        self.assertListEqual(
            [
                'flask.request',
                'flask.try_trigger_before_first_request_functions',
                'flask.preprocess_request',
                'flask.dispatch_request',
                'tests.contrib.flask.test_request.index',
                'flask.process_response',
                'flask.do_teardown_request',
                'flask.do_teardown_appcontext',
            ],
            [s.name for s in spans],
        )

        # Assert span services
        for span in spans:
            self.assertEqual(span.service, 'flask')

        # Root request span
        req_span = spans[0]
        self.assertEqual(req_span.service, 'flask')
        self.assertEqual(req_span.name, 'flask.request')
        # Note: contains no query string
        self.assertEqual(req_span.resource, 'GET /')
        self.assertEqual(req_span.span_type, 'web')
        self.assertEqual(req_span.error, 0)
        self.assertIsNone(req_span.parent_id)

        # Request tags
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'index')
        # Note: contains no query string
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/')
        self.assertEqual(req_span.get_tag('http.method'), 'GET')
        # Note: contains no query string
        self.assertEqual(req_span.get_tag(http.URL), 'http://localhost/')
        assert_span_http_status_code(req_span, 200)

        # Handler span
        handler_span = spans[4]
        self.assertEqual(handler_span.service, 'flask')
        self.assertEqual(handler_span.name, 'tests.contrib.flask.test_request.index')
        # Note: contains no query string
        self.assertEqual(handler_span.resource, '/')
        self.assertEqual(req_span.error, 0)

    def test_request_unicode(self):
        """
        When making a request
            When the url contains unicode
                We create the expected spans
        """
        @self.app.route(u'/üŋïĉóđē')
        def unicode():
            return 'üŋïĉóđē', 200

        res = self.client.get(u'/üŋïĉóđē')
        self.assertEqual(res.status_code, 200)
        self.assertEqual(res.data, b'\xc3\xbc\xc5\x8b\xc3\xaf\xc4\x89\xc3\xb3\xc4\x91\xc4\x93')

        spans = self.get_spans()
        self.assertEqual(len(spans), 8)

        # Assert the order of the spans created
        self.assertListEqual(
            [
                'flask.request',
                'flask.try_trigger_before_first_request_functions',
                'flask.preprocess_request',
                'flask.dispatch_request',
                'tests.contrib.flask.test_request.unicode',
                'flask.process_response',
                'flask.do_teardown_request',
                'flask.do_teardown_appcontext',
            ],
            [s.name for s in spans],
        )

        # Assert span services
        for span in spans:
            self.assertEqual(span.service, 'flask')

        # Root request span
        req_span = spans[0]
        self.assertEqual(req_span.service, 'flask')
        self.assertEqual(req_span.name, 'flask.request')
        self.assertEqual(req_span.resource, u'GET /üŋïĉóđē')
        self.assertEqual(req_span.span_type, 'web')
        self.assertEqual(req_span.error, 0)
        self.assertIsNone(req_span.parent_id)

        # Request tags
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'unicode')
        self.assertEqual(req_span.get_tag('flask.url_rule'), u'/üŋïĉóđē')
        self.assertEqual(req_span.get_tag('http.method'), 'GET')
        self.assertEqual(req_span.get_tag(http.URL), u'http://localhost/üŋïĉóđē')
        assert_span_http_status_code(req_span, 200)

        # Handler span
        handler_span = spans[4]
        self.assertEqual(handler_span.service, 'flask')
        self.assertEqual(handler_span.name, 'tests.contrib.flask.test_request.unicode')
        self.assertEqual(handler_span.resource, u'/üŋïĉóđē')
        self.assertEqual(req_span.error, 0)

    def test_request_404(self):
        """
        When making a request
            When the requested endpoint was not found
                We create the expected spans
        """
        res = self.client.get('/not-found')
        self.assertEqual(res.status_code, 404)

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        # Assert the order of the spans created
        self.assertListEqual(
            [
                'flask.request',
                'flask.try_trigger_before_first_request_functions',
                'flask.preprocess_request',
                'flask.dispatch_request',
                'flask.handle_user_exception',
                'flask.handle_http_exception',
                'flask.process_response',
                'flask.do_teardown_request',
                'flask.do_teardown_appcontext',
            ],
            [s.name for s in spans],
        )

        # Assert span services
        for span in spans:
            self.assertEqual(span.service, 'flask')

        # Root request span
        req_span = spans[0]
        self.assertEqual(req_span.service, 'flask')
        self.assertEqual(req_span.name, 'flask.request')
        self.assertEqual(req_span.resource, 'GET 404')
        self.assertEqual(req_span.span_type, 'web')
        self.assertEqual(req_span.error, 0)
        self.assertIsNone(req_span.parent_id)

        # Request tags
        self.assertEqual(req_span.get_tag('http.method'), 'GET')
        self.assertEqual(req_span.get_tag(http.URL), 'http://localhost/not-found')
        assert_span_http_status_code(req_span, 404)

        # Dispatch span
        dispatch_span = spans[3]
        self.assertEqual(dispatch_span.service, 'flask')
        self.assertEqual(dispatch_span.name, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.resource, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.error, 1)
        self.assertTrue(dispatch_span.get_tag('error.msg').startswith('404 Not Found'))
        self.assertTrue(dispatch_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(dispatch_span.get_tag('error.type'), 'werkzeug.exceptions.NotFound')

    def test_request_abort_404(self):
        """
        When making a request
            When the requested endpoint calls `abort(404)`
                We create the expected spans
        """
        @self.app.route('/not-found')
        def not_found():
            abort(404)

        res = self.client.get('/not-found')
        self.assertEqual(res.status_code, 404)

        spans = self.get_spans()
        self.assertEqual(len(spans), 10)

        # Assert the order of the spans created
        self.assertListEqual(
            [
                'flask.request',
                'flask.try_trigger_before_first_request_functions',
                'flask.preprocess_request',
                'flask.dispatch_request',
                'tests.contrib.flask.test_request.not_found',
                'flask.handle_user_exception',
                'flask.handle_http_exception',
                'flask.process_response',
                'flask.do_teardown_request',
                'flask.do_teardown_appcontext',
            ],
            [s.name for s in spans],
        )

        # Assert span services
        for span in spans:
            self.assertEqual(span.service, 'flask')

        # Root request span
        req_span = spans[0]
        self.assertEqual(req_span.service, 'flask')
        self.assertEqual(req_span.name, 'flask.request')
        self.assertEqual(req_span.resource, 'GET /not-found')
        self.assertEqual(req_span.span_type, 'web')
        self.assertEqual(req_span.error, 0)
        self.assertIsNone(req_span.parent_id)

        # Request tags
        self.assertEqual(req_span.get_tag('http.method'), 'GET')
        self.assertEqual(req_span.get_tag(http.URL), 'http://localhost/not-found')
        assert_span_http_status_code(req_span, 404)
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'not_found')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/not-found')

        # Dispatch span
        dispatch_span = spans[3]
        self.assertEqual(dispatch_span.service, 'flask')
        self.assertEqual(dispatch_span.name, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.resource, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.error, 1)
        self.assertTrue(dispatch_span.get_tag('error.msg').startswith('404 Not Found'))
        self.assertTrue(dispatch_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(dispatch_span.get_tag('error.type'), 'werkzeug.exceptions.NotFound')

        # Handler span
        handler_span = spans[4]
        self.assertEqual(handler_span.service, 'flask')
        self.assertEqual(handler_span.name, 'tests.contrib.flask.test_request.not_found')
        self.assertEqual(handler_span.resource, '/not-found')
        self.assertEqual(handler_span.error, 1)
        self.assertTrue(handler_span.get_tag('error.msg').startswith('404 Not Found'))
        self.assertTrue(handler_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(handler_span.get_tag('error.type'), 'werkzeug.exceptions.NotFound')

    def test_request_500(self):
        """
        When making a request
            When the requested endpoint raises an exception
                We create the expected spans
        """
        @self.app.route('/500')
        def fivehundred():
            raise Exception('500 error')

        res = self.client.get('/500')
        self.assertEqual(res.status_code, 500)

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        # Assert the order of the spans created
        self.assertListEqual(
            [
                'flask.request',
                'flask.try_trigger_before_first_request_functions',
                'flask.preprocess_request',
                'flask.dispatch_request',
                'tests.contrib.flask.test_request.fivehundred',
                'flask.handle_user_exception',
                'flask.handle_exception',
                'flask.do_teardown_request',
                'flask.do_teardown_appcontext',
            ],
            [s.name for s in spans],
        )

        # Assert span services
        for span in spans:
            self.assertEqual(span.service, 'flask')

        # Root request span
        req_span = spans[0]
        self.assertEqual(req_span.service, 'flask')
        self.assertEqual(req_span.name, 'flask.request')
        self.assertEqual(req_span.resource, 'GET /500')
        self.assertEqual(req_span.span_type, 'web')
        self.assertEqual(req_span.error, 1)
        self.assertIsNone(req_span.parent_id)

        # Request tags
        self.assertEqual(req_span.get_tag('http.method'), 'GET')
        self.assertEqual(req_span.get_tag(http.URL), 'http://localhost/500')
        assert_span_http_status_code(req_span, 500)
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'fivehundred')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/500')

        # Dispatch span
        dispatch_span = spans[3]
        self.assertEqual(dispatch_span.service, 'flask')
        self.assertEqual(dispatch_span.name, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.resource, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.error, 1)
        self.assertTrue(dispatch_span.get_tag('error.msg').startswith('500 error'))
        self.assertTrue(dispatch_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(dispatch_span.get_tag('error.type'), base_exception_name)

        # Handler span
        handler_span = spans[4]
        self.assertEqual(handler_span.service, 'flask')
        self.assertEqual(handler_span.name, 'tests.contrib.flask.test_request.fivehundred')
        self.assertEqual(handler_span.resource, '/500')
        self.assertEqual(handler_span.error, 1)
        self.assertTrue(handler_span.get_tag('error.msg').startswith('500 error'))
        self.assertTrue(handler_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(handler_span.get_tag('error.type'), base_exception_name)

        # User exception span
        user_ex_span = spans[5]
        self.assertEqual(user_ex_span.service, 'flask')
        self.assertEqual(user_ex_span.name, 'flask.handle_user_exception')
        self.assertEqual(user_ex_span.resource, 'flask.handle_user_exception')
        self.assertEqual(user_ex_span.error, 1)
        self.assertTrue(user_ex_span.get_tag('error.msg').startswith('500 error'))
        self.assertTrue(user_ex_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(user_ex_span.get_tag('error.type'), base_exception_name)

    def test_request_501(self):
        """
        When making a request
            When the requested endpoint calls `abort(501)`
                We create the expected spans
        """
        @self.app.route('/501')
        def fivehundredone():
            abort(501)

        res = self.client.get('/501')
        self.assertEqual(res.status_code, 501)

        spans = self.get_spans()
        self.assertEqual(len(spans), 10)

        # Assert the order of the spans created
        self.assertListEqual(
            [
                'flask.request',
                'flask.try_trigger_before_first_request_functions',
                'flask.preprocess_request',
                'flask.dispatch_request',
                'tests.contrib.flask.test_request.fivehundredone',
                'flask.handle_user_exception',
                'flask.handle_http_exception',
                'flask.process_response',
                'flask.do_teardown_request',
                'flask.do_teardown_appcontext',
            ],
            [s.name for s in spans],
        )

        # Assert span services
        for span in spans:
            self.assertEqual(span.service, 'flask')

        # Root request span
        req_span = spans[0]
        self.assertEqual(req_span.service, 'flask')
        self.assertEqual(req_span.name, 'flask.request')
        self.assertEqual(req_span.resource, 'GET /501')
        self.assertEqual(req_span.span_type, 'web')
        self.assertEqual(req_span.error, 1)
        self.assertIsNone(req_span.parent_id)

        # Request tags
        self.assertEqual(req_span.get_tag('http.method'), 'GET')
        self.assertEqual(req_span.get_tag(http.URL), 'http://localhost/501')
        assert_span_http_status_code(req_span, 501)
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'fivehundredone')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/501')

        # Dispatch span
        dispatch_span = spans[3]
        self.assertEqual(dispatch_span.service, 'flask')
        self.assertEqual(dispatch_span.name, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.resource, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.error, 1)
        self.assertTrue(dispatch_span.get_tag('error.msg').startswith('501 Not Implemented'))
        self.assertTrue(dispatch_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(dispatch_span.get_tag('error.type'), 'werkzeug.exceptions.NotImplemented')

        # Handler span
        handler_span = spans[4]
        self.assertEqual(handler_span.service, 'flask')
        self.assertEqual(handler_span.name, 'tests.contrib.flask.test_request.fivehundredone')
        self.assertEqual(handler_span.resource, '/501')
        self.assertEqual(handler_span.error, 1)
        self.assertTrue(handler_span.get_tag('error.msg').startswith('501 Not Implemented'))
        self.assertTrue(handler_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(handler_span.get_tag('error.type'), 'werkzeug.exceptions.NotImplemented')

        # User exception span
        user_ex_span = spans[5]
        self.assertEqual(user_ex_span.service, 'flask')
        self.assertEqual(user_ex_span.name, 'flask.handle_user_exception')
        self.assertEqual(user_ex_span.resource, 'flask.handle_user_exception')
        self.assertEqual(user_ex_span.error, 0)

    def test_request_error_handler(self):
        """
        When making a request
            When the requested endpoint raises an exception
                We create the expected spans
        """
        @self.app.errorhandler(500)
        def error_handler(e):
            return 'Whoops', 500

        @self.app.route('/500')
        def fivehundred():
            raise Exception('500 error')

        res = self.client.get('/500')
        self.assertEqual(res.status_code, 500)
        self.assertEqual(res.data, b'Whoops')

        spans = self.get_spans()

        if flask_version >= (0, 12, 0):
            self.assertEqual(len(spans), 11)

            # Assert the order of the spans created
            self.assertListEqual(
                [
                    'flask.request',
                    'flask.try_trigger_before_first_request_functions',
                    'flask.preprocess_request',
                    'flask.dispatch_request',
                    'tests.contrib.flask.test_request.fivehundred',
                    'flask.handle_user_exception',
                    'flask.handle_exception',
                    'tests.contrib.flask.test_request.error_handler',
                    'flask.process_response',
                    'flask.do_teardown_request',
                    'flask.do_teardown_appcontext',
                ],
                [s.name for s in spans],
            )
        else:
            self.assertEqual(len(spans), 10)

            # Assert the order of the spans created
            self.assertListEqual(
                [
                    'flask.request',
                    'flask.try_trigger_before_first_request_functions',
                    'flask.preprocess_request',
                    'flask.dispatch_request',
                    'tests.contrib.flask.test_request.fivehundred',
                    'flask.handle_user_exception',
                    'flask.handle_exception',
                    'tests.contrib.flask.test_request.error_handler',
                    'flask.do_teardown_request',
                    'flask.do_teardown_appcontext',
                ],
                [s.name for s in spans],
            )

        # Assert span services
        for span in spans:
            self.assertEqual(span.service, 'flask')

        # Root request span
        req_span = spans[0]
        self.assertEqual(req_span.service, 'flask')
        self.assertEqual(req_span.name, 'flask.request')
        self.assertEqual(req_span.resource, 'GET /500')
        self.assertEqual(req_span.span_type, 'web')
        self.assertEqual(req_span.error, 1)
        self.assertIsNone(req_span.parent_id)

        # Request tags
        self.assertEqual(req_span.get_tag('http.method'), 'GET')
        self.assertEqual(req_span.get_tag(http.URL), 'http://localhost/500')
        assert_span_http_status_code(req_span, 500)
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'fivehundred')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/500')

        # Dispatch span
        dispatch_span = spans[3]
        self.assertEqual(dispatch_span.service, 'flask')
        self.assertEqual(dispatch_span.name, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.resource, 'flask.dispatch_request')
        self.assertEqual(dispatch_span.error, 1)
        self.assertTrue(dispatch_span.get_tag('error.msg').startswith('500 error'))
        self.assertTrue(dispatch_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(dispatch_span.get_tag('error.type'), base_exception_name)

        # Handler span
        handler_span = spans[4]
        self.assertEqual(handler_span.service, 'flask')
        self.assertEqual(handler_span.name, 'tests.contrib.flask.test_request.fivehundred')
        self.assertEqual(handler_span.resource, '/500')
        self.assertEqual(handler_span.error, 1)
        self.assertTrue(handler_span.get_tag('error.msg').startswith('500 error'))
        self.assertTrue(handler_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(handler_span.get_tag('error.type'), base_exception_name)

        # User exception span
        user_ex_span = spans[5]
        self.assertEqual(user_ex_span.service, 'flask')
        self.assertEqual(user_ex_span.name, 'flask.handle_user_exception')
        self.assertEqual(user_ex_span.resource, 'flask.handle_user_exception')
        self.assertEqual(user_ex_span.error, 1)
        self.assertTrue(user_ex_span.get_tag('error.msg').startswith('500 error'))
        self.assertTrue(user_ex_span.get_tag('error.stack').startswith('Traceback'))
        self.assertEqual(user_ex_span.get_tag('error.type'), base_exception_name)
