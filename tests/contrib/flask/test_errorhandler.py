import flask

from . import BaseFlaskTestCase
from ...utils import assert_span_http_status_code


class FlaskErrorhandlerTestCase(BaseFlaskTestCase):
    def test_default_404_handler(self):
        """
        When making a 404 request
            And no user defined error handler is defined
                We create the expected spans
        """
        # Make our 404 request
        res = self.client.get('/unknown')
        self.assertEqual(res.status_code, 404)

        spans = self.get_spans()

        req_span = self.find_span_by_name(spans, 'flask.request')
        dispatch_span = self.find_span_by_name(spans, 'flask.dispatch_request')
        user_ex_span = self.find_span_by_name(spans, 'flask.handle_user_exception')
        http_ex_span = self.find_span_by_name(spans, 'flask.handle_http_exception')

        # flask.request span
        self.assertEqual(req_span.error, 0)
        assert_span_http_status_code(req_span, 404)
        self.assertIsNone(req_span.get_tag('flask.endpoint'))
        self.assertIsNone(req_span.get_tag('flask.url_rule'))

        # flask.dispatch_request span
        self.assertEqual(dispatch_span.error, 1)
        error_msg = dispatch_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('404 Not Found'))
        error_stack = dispatch_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = dispatch_span.get_tag('error.type')
        self.assertEqual(error_type, 'werkzeug.exceptions.NotFound')

        # flask.handle_user_exception span
        self.assertEqual(user_ex_span.meta, dict())
        self.assertEqual(user_ex_span.error, 0)

        # flask.handle_http_exception span
        self.assertEqual(http_ex_span.meta, dict())
        self.assertEqual(http_ex_span.error, 0)

    def test_abort_500(self):
        """
        When making a 500 request
            And no user defined error handler is defined
                We create the expected spans
        """
        @self.app.route('/500')
        def endpoint_500():
            flask.abort(500)

        # Make our 500 request
        res = self.client.get('/500')
        self.assertEqual(res.status_code, 500)

        spans = self.get_spans()

        req_span = self.find_span_by_name(spans, 'flask.request')
        dispatch_span = self.find_span_by_name(spans, 'flask.dispatch_request')
        endpoint_span = self.find_span_by_name(spans, 'tests.contrib.flask.test_errorhandler.endpoint_500')
        user_ex_span = self.find_span_by_name(spans, 'flask.handle_user_exception')
        http_ex_span = self.find_span_by_name(spans, 'flask.handle_http_exception')

        # flask.request span
        self.assertEqual(req_span.error, 1)

        assert_span_http_status_code(req_span, 500)
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'endpoint_500')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/500')

        # flask.dispatch_request span
        self.assertEqual(dispatch_span.error, 1)
        error_msg = dispatch_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('500 Internal Server Error'))
        error_stack = dispatch_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = dispatch_span.get_tag('error.type')
        self.assertEqual(error_type, 'werkzeug.exceptions.InternalServerError')

        # tests.contrib.flask.test_errorhandler.endpoint_500 span
        self.assertEqual(endpoint_span.error, 1)
        error_msg = endpoint_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('500 Internal Server Error'))
        error_stack = endpoint_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = endpoint_span.get_tag('error.type')
        self.assertEqual(error_type, 'werkzeug.exceptions.InternalServerError')

        # flask.handle_user_exception span
        self.assertEqual(user_ex_span.meta, dict())
        self.assertEqual(user_ex_span.error, 0)

        # flask.handle_http_exception span
        self.assertEqual(http_ex_span.meta, dict())
        self.assertEqual(http_ex_span.error, 0)

    def test_abort_500_custom_handler(self):
        """
        When making a 500 request
            And a user defined error handler is defined
                We create the expected spans
        """
        @self.app.errorhandler(500)
        def handle_500(e):
            return 'whoops', 200

        @self.app.route('/500')
        def endpoint_500():
            flask.abort(500)

        # Make our 500 request
        res = self.client.get('/500')
        self.assertEqual(res.status_code, 200)
        self.assertEqual(res.data, b'whoops')

        spans = self.get_spans()

        req_span = self.find_span_by_name(spans, 'flask.request')
        dispatch_span = self.find_span_by_name(spans, 'flask.dispatch_request')
        endpoint_span = self.find_span_by_name(spans, 'tests.contrib.flask.test_errorhandler.endpoint_500')
        handler_span = self.find_span_by_name(spans, 'tests.contrib.flask.test_errorhandler.handle_500')
        user_ex_span = self.find_span_by_name(spans, 'flask.handle_user_exception')
        http_ex_span = self.find_span_by_name(spans, 'flask.handle_http_exception')

        # flask.request span
        self.assertEqual(req_span.error, 0)
        assert_span_http_status_code(req_span, 200)
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'endpoint_500')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/500')

        # flask.dispatch_request span
        self.assertEqual(dispatch_span.error, 1)
        error_msg = dispatch_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('500 Internal Server Error'))
        error_stack = dispatch_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = dispatch_span.get_tag('error.type')
        self.assertEqual(error_type, 'werkzeug.exceptions.InternalServerError')

        # tests.contrib.flask.test_errorhandler.endpoint_500 span
        self.assertEqual(endpoint_span.error, 1)
        error_msg = endpoint_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('500 Internal Server Error'))
        error_stack = endpoint_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = endpoint_span.get_tag('error.type')
        self.assertEqual(error_type, 'werkzeug.exceptions.InternalServerError')

        # tests.contrib.flask.test_errorhandler.handle_500 span
        self.assertEqual(handler_span.error, 0)
        self.assertIsNone(handler_span.get_tag('error.msg'))
        self.assertIsNone(handler_span.get_tag('error.stack'))
        self.assertIsNone(handler_span.get_tag('error.type'))

        # flask.handle_user_exception span
        self.assertEqual(user_ex_span.meta, dict())
        self.assertEqual(user_ex_span.error, 0)

        # flask.handle_http_exception span
        self.assertEqual(http_ex_span.meta, dict())
        self.assertEqual(http_ex_span.error, 0)

    def test_raise_user_exception(self):
        """
        When raising a custom user exception
            And no user defined error handler is defined
                We create the expected spans
        """
        class FlaskTestException(Exception):
            pass

        @self.app.route('/error')
        def endpoint_error():
            raise FlaskTestException('custom error message')

        # Make our 500 request
        res = self.client.get('/error')
        self.assertEqual(res.status_code, 500)

        spans = self.get_spans()

        req_span = self.find_span_by_name(spans, 'flask.request')
        dispatch_span = self.find_span_by_name(spans, 'flask.dispatch_request')
        endpoint_span = self.find_span_by_name(spans, 'tests.contrib.flask.test_errorhandler.endpoint_error')
        user_ex_span = self.find_span_by_name(spans, 'flask.handle_user_exception')
        http_ex_span = self.find_span_by_name(spans, 'flask.handle_http_exception', required=False)

        # flask.request span
        self.assertEqual(req_span.error, 1)
        assert_span_http_status_code(req_span, 500)
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'endpoint_error')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/error')

        # flask.dispatch_request span
        self.assertEqual(dispatch_span.error, 1)
        error_msg = dispatch_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('custom error message'))
        error_stack = dispatch_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = dispatch_span.get_tag('error.type')
        self.assertEqual(error_type, 'tests.contrib.flask.test_errorhandler.FlaskTestException')

        # tests.contrib.flask.test_errorhandler.endpoint_500 span
        self.assertEqual(endpoint_span.error, 1)
        error_msg = endpoint_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('custom error message'))
        error_stack = endpoint_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = endpoint_span.get_tag('error.type')
        self.assertEqual(error_type, 'tests.contrib.flask.test_errorhandler.FlaskTestException')

        # flask.handle_user_exception span
        self.assertEqual(user_ex_span.error, 1)
        error_msg = user_ex_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('custom error message'))
        error_stack = user_ex_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = user_ex_span.get_tag('error.type')
        self.assertEqual(error_type, 'tests.contrib.flask.test_errorhandler.FlaskTestException')

        # flask.handle_http_exception span
        self.assertIsNone(http_ex_span)

    def test_raise_user_exception_handler(self):
        """
        When raising a custom user exception
            And a user defined error handler is defined
                We create the expected spans
        """
        class FlaskTestException(Exception):
            pass

        @self.app.errorhandler(FlaskTestException)
        def handle_error(e):
            return 'whoops', 200

        @self.app.route('/error')
        def endpoint_error():
            raise FlaskTestException('custom error message')

        # Make our 500 request
        res = self.client.get('/error')
        self.assertEqual(res.status_code, 200)
        self.assertEqual(res.data, b'whoops')

        spans = self.get_spans()

        req_span = self.find_span_by_name(spans, 'flask.request')
        dispatch_span = self.find_span_by_name(spans, 'flask.dispatch_request')
        endpoint_span = self.find_span_by_name(spans, 'tests.contrib.flask.test_errorhandler.endpoint_error')
        handler_span = self.find_span_by_name(spans, 'tests.contrib.flask.test_errorhandler.handle_error')
        user_ex_span = self.find_span_by_name(spans, 'flask.handle_user_exception')
        http_ex_span = self.find_span_by_name(spans, 'flask.handle_http_exception', required=False)

        # flask.request span
        self.assertEqual(req_span.error, 0)
        assert_span_http_status_code(req_span, 200)
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'endpoint_error')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/error')

        # flask.dispatch_request span
        self.assertEqual(dispatch_span.error, 1)
        error_msg = dispatch_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('custom error message'))
        error_stack = dispatch_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = dispatch_span.get_tag('error.type')
        self.assertEqual(error_type, 'tests.contrib.flask.test_errorhandler.FlaskTestException')

        # tests.contrib.flask.test_errorhandler.endpoint_500 span
        self.assertEqual(endpoint_span.error, 1)
        error_msg = endpoint_span.get_tag('error.msg')
        self.assertTrue(error_msg.startswith('custom error message'))
        error_stack = endpoint_span.get_tag('error.stack')
        self.assertTrue(error_stack.startswith('Traceback (most recent call last):'))
        error_type = endpoint_span.get_tag('error.type')
        self.assertEqual(error_type, 'tests.contrib.flask.test_errorhandler.FlaskTestException')

        # tests.contrib.flask.test_errorhandler.handle_error span
        self.assertEqual(handler_span.error, 0)

        # flask.handle_user_exception span
        self.assertEqual(user_ex_span.error, 0)
        self.assertEqual(user_ex_span.meta, dict())

        # flask.handle_http_exception span
        self.assertIsNone(http_ex_span)
