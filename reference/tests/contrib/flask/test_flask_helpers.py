import flask

from ddtrace import Pin
from ddtrace.contrib.flask import unpatch
from ddtrace.compat import StringIO

from . import BaseFlaskTestCase


class FlaskHelpersTestCase(BaseFlaskTestCase):
    def test_patch(self):
        """
        When we patch Flask
            Then ``flask.jsonify`` is patched
            Then ``flask.send_file`` is patched
        """
        # DEV: We call `patch` in `setUp`
        self.assert_is_wrapped(flask.jsonify)
        self.assert_is_wrapped(flask.send_file)

    def test_unpatch(self):
        """
        When we unpatch Flask
            Then ``flask.jsonify`` is unpatched
            Then ``flask.send_file`` is unpatched
        """
        unpatch()
        self.assert_is_not_wrapped(flask.jsonify)
        self.assert_is_not_wrapped(flask.send_file)

    def test_jsonify(self):
        """
        When we call a patched ``flask.jsonify``
            We create a span as expected
        """
        # DEV: `jsonify` requires a active app and request contexts
        with self.app.app_context():
            with self.app.test_request_context('/'):
                response = flask.jsonify(dict(key='value'))
                self.assertTrue(isinstance(response, flask.Response))
                self.assertEqual(response.status_code, 200)

        # 1 span for `jsonify`
        # 1 span for tearing down the app context we created
        # 1 span for tearing down the request context we created
        spans = self.get_spans()
        self.assertEqual(len(spans), 3)

        self.assertIsNone(spans[0].service)
        self.assertEqual(spans[0].name, 'flask.jsonify')
        self.assertEqual(spans[0].resource, 'flask.jsonify')
        assert spans[0].meta == dict()

        self.assertEqual(spans[1].name, 'flask.do_teardown_request')
        self.assertEqual(spans[2].name, 'flask.do_teardown_appcontext')

    def test_jsonify_pin_disabled(self):
        """
        When we call a patched ``flask.jsonify``
            When the ``flask.Flask`` ``Pin`` is disabled
                We do not create a span
        """
        # Disable the pin on the app
        pin = Pin.get_from(self.app)
        pin.tracer.enabled = False

        # DEV: `jsonify` requires a active app and request contexts
        with self.app.app_context():
            with self.app.test_request_context('/'):
                response = flask.jsonify(dict(key='value'))
                self.assertTrue(isinstance(response, flask.Response))
                self.assertEqual(response.status_code, 200)

        self.assertEqual(len(self.get_spans()), 0)

    def test_send_file(self):
        """
        When calling a patched ``flask.send_file``
            We create the expected spans
        """
        fp = StringIO('static file')

        with self.app.app_context():
            with self.app.test_request_context('/'):
                # DEV: Flask >= (0, 12, 0) tries to infer mimetype, so set explicitly
                response = flask.send_file(fp, mimetype='text/plain')
                self.assertTrue(isinstance(response, flask.Response))
                self.assertEqual(response.status_code, 200)

        # 1 for `send_file`
        # 1 for tearing down the request context we created
        # 1 for tearing down the app context we created
        spans = self.get_spans()
        self.assertEqual(len(spans), 3)

        self.assertEqual(spans[0].service, 'flask')
        self.assertEqual(spans[0].name, 'flask.send_file')
        self.assertEqual(spans[0].resource, 'flask.send_file')
        assert spans[0].meta == dict()

        self.assertEqual(spans[1].name, 'flask.do_teardown_request')
        self.assertEqual(spans[2].name, 'flask.do_teardown_appcontext')

    def test_send_file_pin_disabled(self):
        """
        When calling a patched ``flask.send_file``
            When the app's ``Pin`` has been disabled
                We do not create any spans
        """
        pin = Pin.get_from(self.app)
        pin.tracer.enabled = False

        fp = StringIO('static file')
        with self.app.app_context():
            with self.app.test_request_context('/'):
                # DEV: Flask >= (0, 12, 0) tries to infer mimetype, so set explicitly
                response = flask.send_file(fp, mimetype='text/plain')
                self.assertTrue(isinstance(response, flask.Response))
                self.assertEqual(response.status_code, 200)

        self.assertEqual(len(self.get_spans()), 0)
