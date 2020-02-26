import mock

import flask

from ddtrace import Pin
from ddtrace.contrib.flask import unpatch
from ddtrace.contrib.flask.patch import flask_version

from . import BaseFlaskTestCase


class FlaskSignalsTestCase(BaseFlaskTestCase):
    def get_signal(self, signal_name):
        # v0.9 missed importing `appcontext_tearing_down` in `flask/__init__.py`
        #  https://github.com/pallets/flask/blob/0.9/flask/__init__.py#L35-L37
        #  https://github.com/pallets/flask/blob/0.9/flask/signals.py#L52
        # DEV: Version 0.9 doesn't have a patch version
        if flask_version <= (0, 9) and signal_name == 'appcontext_tearing_down':
            return getattr(flask.signals, signal_name)
        return getattr(flask, signal_name)

    def signal_function(self, name):
        def signal(*args, **kwargs):
            pass

        func = mock.Mock(signal, name=name)
        func.__module__ = 'tests.contrib.flask'
        func.__name__ = name
        return func

    def call_signal(self, signal_name, *args, **kwargs):
        """Context manager helper used for generating a mock signal function and registering with flask"""
        func = self.signal_function(signal_name)

        signal = self.get_signal(signal_name)
        signal.connect(func, self.app)

        try:
            signal.send(*args, **kwargs)
            return func
        finally:
            # DEV: There is a bug in `blinker.Signal.disconnect` :(
            signal.receivers.clear()

    def signals(self):
        """Helper to get the signals for the current Flask version being tested"""
        signals = [
            'request_started',
            'request_finished',
            'request_tearing_down',

            'template_rendered',

            'got_request_exception',
            'appcontext_tearing_down',
        ]
        # This signal was added in 0.11.0
        if flask_version >= (0, 11):
            signals.append('before_render_template')

        # These were added in 0.10
        if flask_version >= (0, 10):
            signals.append('appcontext_pushed')
            signals.append('appcontext_popped')
            signals.append('message_flashed')

        return signals

    def test_patched(self):
        """
        When the signals are patched
            Their ``receivers_for`` method is wrapped as a ``wrapt.ObjectProxy``
        """
        # DEV: We call `patch()` in `setUp`
        for signal_name in self.signals():
            signal = self.get_signal(signal_name)
            receivers_for = getattr(signal, 'receivers_for')
            self.assert_is_wrapped(receivers_for)

    def test_unpatch(self):
        """
        When the signals are unpatched
            Their ``receivers_for`` method is not a ``wrapt.ObjectProxy``
        """
        unpatch()

        for signal_name in self.signals():
            signal = self.get_signal(signal_name)
            receivers_for = getattr(signal, 'receivers_for')
            self.assert_is_not_wrapped(receivers_for)

    def test_signals(self):
        """
        When a signal is connected
            We create a span whenever that signal is sent
        """
        for signal_name in self.signals():
            func = self.call_signal(signal_name, self.app)

            # Ensure our handler was called
            func.assert_called_once_with(self.app)

            # Assert number of spans created
            spans = self.get_spans()
            self.assertEqual(len(spans), 1)

            # Assert the span that was created
            span = spans[0]
            self.assertEqual(span.service, 'flask')
            self.assertEqual(span.name, 'tests.contrib.flask.{}'.format(signal_name))
            self.assertEqual(span.resource, 'tests.contrib.flask.{}'.format(signal_name))
            self.assertEqual(set(span.meta.keys()), set(['flask.signal']))
            self.assertEqual(span.meta['flask.signal'], signal_name)

    def test_signals_multiple(self):
        """
        When a signal is connected
            When multiple functions are registered for that signal
                We create a span whenever that signal is sent
        """
        # Our signal handlers
        request_started_a = self.signal_function('request_started_a')
        request_started_b = self.signal_function('request_started_b')

        flask.request_started.connect(request_started_a, self.app)
        flask.request_started.connect(request_started_b, self.app)

        try:
            flask.request_started.send(self.app)
        finally:
            # DEV: There is a bug in `blinker.Signal.disconnect` :(
            flask.request_started.receivers.clear()

        # Ensure our handlers were called only once
        request_started_a.assert_called_once_with(self.app)
        request_started_b.assert_called_once_with(self.app)

        # Assert number of spans created
        spans = self.get_spans()
        self.assertEqual(len(spans), 2)

        # Assert the span that was created
        span_a = spans[0]
        self.assertEqual(span_a.service, 'flask')
        self.assertEqual(span_a.name, 'tests.contrib.flask.request_started_a')
        self.assertEqual(span_a.resource, 'tests.contrib.flask.request_started_a')
        self.assertEqual(set(span_a.meta.keys()), set(['flask.signal']))
        self.assertEqual(span_a.meta['flask.signal'], 'request_started')

        # Assert the span that was created
        span_b = spans[1]
        self.assertEqual(span_b.service, 'flask')
        self.assertEqual(span_b.name, 'tests.contrib.flask.request_started_b')
        self.assertEqual(span_b.resource, 'tests.contrib.flask.request_started_b')
        self.assertEqual(set(span_b.meta.keys()), set(['flask.signal']))
        self.assertEqual(span_b.meta['flask.signal'], 'request_started')

    def test_signals_pin_disabled(self):
        """
        When a signal is connected
            When the app pin is disabled
                We do not create any spans when the signal is sent
        """
        # Disable the pin on the app
        pin = Pin.get_from(self.app)
        pin.tracer.enabled = False

        for signal_name in self.signals():
            func = self.call_signal(signal_name, self.app)

            # Ensure our function was called by the signal
            func.assert_called_once_with(self.app)

            # Assert number of spans created
            spans = self.get_spans()
            self.assertEqual(len(spans), 0)
