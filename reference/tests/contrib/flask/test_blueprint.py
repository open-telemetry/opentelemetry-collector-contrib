import flask

from ddtrace import Pin
from ddtrace.contrib.flask import unpatch

from . import BaseFlaskTestCase


class FlaskBlueprintTestCase(BaseFlaskTestCase):
    def test_patch(self):
        """
        When we patch Flask
            Then ``flask.Blueprint.register`` is patched
            Then ``flask.Blueprint.add_url_rule`` is patched
        """
        # DEV: We call `patch` in `setUp`
        self.assert_is_wrapped(flask.Blueprint.register)
        self.assert_is_wrapped(flask.Blueprint.add_url_rule)

    def test_unpatch(self):
        """
        When we unpatch Flask
            Then ``flask.Blueprint.register`` is not patched
            Then ``flask.Blueprint.add_url_rule`` is not patched
        """
        unpatch()
        self.assert_is_not_wrapped(flask.Blueprint.register)
        self.assert_is_not_wrapped(flask.Blueprint.add_url_rule)

    def test_blueprint_register(self):
        """
        When we register a ``flask.Blueprint`` to a ``flask.Flask``
            When no ``Pin`` is attached to the ``Blueprint``
                We attach the pin from the ``flask.Flask`` app
            When a ``Pin`` is manually added to the ``Blueprint``
                We do not use the ``flask.Flask`` app ``Pin``
        """
        bp = flask.Blueprint('pinned', __name__)
        Pin(service='flask-bp', tracer=self.tracer).onto(bp)

        # DEV: This is more common than calling ``flask.Blueprint.register`` directly
        self.app.register_blueprint(bp)
        pin = Pin.get_from(bp)
        self.assertEqual(pin.service, 'flask-bp')

        bp = flask.Blueprint('not-pinned', __name__)
        self.app.register_blueprint(bp)
        pin = Pin.get_from(bp)
        self.assertEqual(pin.service, 'flask')

    def test_blueprint_add_url_rule(self):
        """
        When we call ``flask.Blueprint.add_url_rule``
            When the ``Blueprint`` has a ``Pin`` attached
                We clone the Blueprint's ``Pin`` to the view
            When the ``Blueprint`` does not have a ``Pin`` attached
                We do not attach a ``Pin`` to the func
        """
        # When the Blueprint has a Pin attached
        bp = flask.Blueprint('pinned', __name__)
        Pin(service='flask-bp', tracer=self.tracer).onto(bp)

        @bp.route('/')
        def test_view():
            pass

        # Assert the view func has a `Pin` attached with the Blueprint's service name
        pin = Pin.get_from(test_view)
        self.assertIsNotNone(pin)
        self.assertEqual(pin.service, 'flask-bp')

        # When the Blueprint does not have a Pin attached
        bp = flask.Blueprint('not-pinned', __name__)

        @bp.route('/')
        def test_view():
            pass

        # Assert the view does not have a `Pin` attached
        pin = Pin.get_from(test_view)
        self.assertIsNone(pin)

    def test_blueprint_request(self):
        """
        When making a request to a Blueprint's endpoint
            We create the expected spans
        """
        bp = flask.Blueprint('bp', __name__)

        @bp.route('/')
        def test():
            return 'test'

        self.app.register_blueprint(bp)

        # Request the endpoint
        self.client.get('/')

        # Only extract the span we care about
        # DEV: Making a request creates a bunch of lifecycle spans,
        #   ignore them, we test them elsewhere
        span = self.find_span_by_name(self.get_spans(), 'bp.test')
        self.assertEqual(span.service, 'flask')
        self.assertEqual(span.name, 'bp.test')
        self.assertEqual(span.resource, '/')
        self.assertEqual(span.meta, dict())

    def test_blueprint_request_pin_override(self):
        """
        When making a request to a Blueprint's endpoint
            When we attach a ``Pin`` to the Blueprint
                We create the expected spans
        """
        bp = flask.Blueprint('bp', __name__)
        Pin.override(bp, service='flask-bp', tracer=self.tracer)

        @bp.route('/')
        def test():
            return 'test'

        self.app.register_blueprint(bp)

        # Request the endpoint
        self.client.get('/')

        # Only extract the span we care about
        # DEV: Making a request creates a bunch of lifecycle spans,
        #   ignore them, we test them elsewhere
        span = self.find_span_by_name(self.get_spans(), 'bp.test')
        self.assertEqual(span.service, 'flask-bp')
        self.assertEqual(span.name, 'bp.test')
        self.assertEqual(span.resource, '/')
        self.assertEqual(span.meta, dict())

    def test_blueprint_request_pin_disabled(self):
        """
        When making a request to a Blueprint's endpoint
            When the app's ``Pin`` is disabled
                We do not create any spans
        """
        pin = Pin.get_from(self.app)
        pin.tracer.enabled = False

        bp = flask.Blueprint('bp', __name__)

        @bp.route('/')
        def test():
            return 'test'

        self.app.register_blueprint(bp)

        # Request the endpoint
        self.client.get('/')

        self.assertEqual(len(self.get_spans()), 0)
