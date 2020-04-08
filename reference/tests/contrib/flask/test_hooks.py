from flask import Blueprint

from ddtrace.ext import http
from . import BaseFlaskTestCase
from ...utils import assert_span_http_status_code


class FlaskHookTestCase(BaseFlaskTestCase):
    def setUp(self):
        super(FlaskHookTestCase, self).setUp()

        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        self.bp = Blueprint(__name__, 'bp')

        @self.bp.route('/bp')
        def bp():
            return 'Hello Blueprint', 200

    def test_before_request(self):
        """
        When Flask before_request hook is registered
            We create the expected spans
        """
        @self.app.before_request
        def before_request():
            pass

        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        # DEV: This will raise an exception if this span doesn't exist
        self.find_span_by_name(spans, 'flask.dispatch_request')

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.before_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.before_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.before_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.preprocess_request')

    def test_before_request_return(self):
        """
        When Flask before_request hook is registered
            When the hook handles the request
                We create the expected spans
        """
        @self.app.before_request
        def before_request():
            return 'Not Allowed', 401

        req = self.client.get('/')
        self.assertEqual(req.status_code, 401)
        self.assertEqual(req.data, b'Not Allowed')

        spans = self.get_spans()
        self.assertEqual(len(spans), 7)

        dispatch = self.find_span_by_name(spans, 'flask.dispatch_request', required=False)
        self.assertIsNone(dispatch)

        root = self.find_span_by_name(spans, 'flask.request')
        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.before_request')
        parent = self.find_span_parent(spans, span)

        # Assert root hook
        # DEV: This is the main thing we need to check with this test
        self.assertEqual(root.service, 'flask')
        self.assertEqual(root.name, 'flask.request')
        self.assertEqual(root.resource, 'GET /')
        self.assertEqual(root.get_tag('flask.endpoint'), 'index')
        self.assertEqual(root.get_tag('flask.url_rule'), '/')
        self.assertEqual(root.get_tag('http.method'), 'GET')
        assert_span_http_status_code(root, 401)
        self.assertEqual(root.get_tag(http.URL), 'http://localhost/')

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.before_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.before_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.preprocess_request')

    def test_before_first_request(self):
        """
        When Flask before_first_request hook is registered
            We create the expected spans
        """
        @self.app.before_first_request
        def before_first_request():
            pass

        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.before_first_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.before_first_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.before_first_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.try_trigger_before_first_request_functions')

        # Make a second request to ensure a span isn't created
        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 8)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.before_first_request', required=False)
        self.assertIsNone(span)

    def test_after_request(self):
        """
        When Flask after_request hook is registered
            We create the expected spans
        """
        @self.app.after_request
        def after_request(response):
            return response

        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.after_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.after_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.after_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.process_response')

    def test_after_request_change_status(self):
        """
        When Flask after_request hook is registered
            We create the expected spans
        """
        @self.app.after_request
        def after_request(response):
            response.status_code = 401
            return response

        req = self.client.get('/')
        self.assertEqual(req.status_code, 401)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        root = self.find_span_by_name(spans, 'flask.request')
        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.after_request')
        parent = self.find_span_parent(spans, span)

        # Assert root span
        assert_span_http_status_code(root, 401)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.after_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.after_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.process_response')

    def test_teardown_request(self):
        """
        When Flask teardown_request hook is registered
            We create the expected spans
        """
        @self.app.teardown_request
        def teardown_request(request):
            pass

        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.teardown_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.teardown_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.teardown_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.do_teardown_request')

    def test_teardown_appcontext(self):
        """
        When Flask teardown_appcontext hook is registered
            We create the expected spans
        """
        @self.app.teardown_appcontext
        def teardown_appcontext(appctx):
            pass

        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.teardown_appcontext')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.teardown_appcontext')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.teardown_appcontext')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.do_teardown_appcontext')

    def test_bp_before_request(self):
        """
        When Blueprint before_request hook is registered
            We create the expected spans
        """
        @self.bp.before_request
        def bp_before_request():
            pass

        self.app.register_blueprint(self.bp)
        req = self.client.get('/bp')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Blueprint')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.bp_before_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.bp_before_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.bp_before_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.preprocess_request')

    def test_bp_before_app_request(self):
        """
        When Blueprint before_app_request hook is registered
            We create the expected spans
        """
        @self.bp.before_app_request
        def bp_before_app_request():
            pass

        self.app.register_blueprint(self.bp)
        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.bp_before_app_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.bp_before_app_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.bp_before_app_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.preprocess_request')

    def test_before_app_first_request(self):
        """
        When Blueprint before_first_request hook is registered
            We create the expected spans
        """
        @self.bp.before_app_first_request
        def bp_before_app_first_request():
            pass

        self.app.register_blueprint(self.bp)
        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.bp_before_app_first_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.bp_before_app_first_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.bp_before_app_first_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.try_trigger_before_first_request_functions')

        # Make a second request to ensure a span isn't created
        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 8)

        span = self.find_span_by_name(
            spans,
            'tests.contrib.flask.test_hooks.bp_before_app_first_request',
            required=False,
        )
        self.assertIsNone(span)

    def test_bp_after_request(self):
        """
        When Blueprint after_request hook is registered
            We create the expected spans
        """
        @self.bp.after_request
        def bp_after_request(response):
            return response

        self.app.register_blueprint(self.bp)
        req = self.client.get('/bp')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Blueprint')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.bp_after_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.bp_after_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.bp_after_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.process_response')

    def test_bp_after_app_request(self):
        """
        When Blueprint after_app_request hook is registered
            We create the expected spans
        """
        @self.bp.after_app_request
        def bp_after_app_request(response):
            return response

        self.app.register_blueprint(self.bp)
        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.bp_after_app_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.bp_after_app_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.bp_after_app_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.process_response')

    def test_bp_teardown_request(self):
        """
        When Blueprint teardown_request hook is registered
            We create the expected spans
        """
        @self.bp.teardown_request
        def bp_teardown_request(request):
            pass

        self.app.register_blueprint(self.bp)
        req = self.client.get('/bp')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Blueprint')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.bp_teardown_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.bp_teardown_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.bp_teardown_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.do_teardown_request')

    def test_bp_teardown_app_request(self):
        """
        When Blueprint teardown_app_request hook is registered
            We create the expected spans
        """
        @self.bp.teardown_app_request
        def bp_teardown_app_request(request):
            pass

        self.app.register_blueprint(self.bp)
        req = self.client.get('/')
        self.assertEqual(req.status_code, 200)
        self.assertEqual(req.data, b'Hello Flask')

        spans = self.get_spans()
        self.assertEqual(len(spans), 9)

        span = self.find_span_by_name(spans, 'tests.contrib.flask.test_hooks.bp_teardown_app_request')
        parent = self.find_span_parent(spans, span)

        # Assert hook span
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'tests.contrib.flask.test_hooks.bp_teardown_app_request')
        self.assertEqual(span.resource, 'tests.contrib.flask.test_hooks.bp_teardown_app_request')
        self.assertEqual(span.meta, dict())

        # Assert correct parent span
        self.assertEqual(parent.name, 'flask.do_teardown_request')
