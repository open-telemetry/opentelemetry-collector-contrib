# -*- coding: utf-8 -*-
import unittest

import flask

from ddtrace.vendor import wrapt
from ddtrace.ext import http
from ddtrace import Pin

from ...test_tracer import get_dummy_tracer
from ...utils import assert_span_http_status_code


class FlaskAutopatchTestCase(unittest.TestCase):
    def setUp(self):
        self.tracer = get_dummy_tracer()
        self.app = flask.Flask(__name__)
        Pin.override(self.app, service='test-flask', tracer=self.tracer)
        self.client = self.app.test_client()

    def test_patched(self):
        """
        When using ddtrace-run
            Then the `flask` module is patched
        """
        # DEV: We have great test coverage in tests.contrib.flask,
        #      we only need basic tests here to assert `ddtrace-run` patched thingsa

        # Assert module is marked as patched
        self.assertTrue(flask._datadog_patch)

        # Assert our instance of flask.app.Flask is patched
        self.assertTrue(isinstance(self.app.add_url_rule, wrapt.ObjectProxy))
        self.assertTrue(isinstance(self.app.wsgi_app, wrapt.ObjectProxy))

        # Assert the base module flask.app.Flask methods are patched
        self.assertTrue(isinstance(flask.app.Flask.add_url_rule, wrapt.ObjectProxy))
        self.assertTrue(isinstance(flask.app.Flask.wsgi_app, wrapt.ObjectProxy))

    def test_request(self):
        """
        When using ddtrace-run
            When making a request to flask app
                We generate the expected spans
        """
        @self.app.route('/')
        def index():
            return 'Hello Flask', 200

        res = self.client.get('/')
        self.assertEqual(res.status_code, 200)
        self.assertEqual(res.data, b'Hello Flask')

        spans = self.tracer.writer.pop()
        self.assertEqual(len(spans), 8)

        self.assertListEqual(
            [
                'flask.request',
                'flask.try_trigger_before_first_request_functions',
                'flask.preprocess_request',
                'flask.dispatch_request',
                'tests.contrib.flask_autopatch.test_flask_autopatch.index',
                'flask.process_response',
                'flask.do_teardown_request',
                'flask.do_teardown_appcontext',
            ],
            [s.name for s in spans],
        )

        # Assert span services
        for span in spans:
            self.assertEqual(span.service, 'test-flask')

        # Root request span
        req_span = spans[0]
        self.assertEqual(req_span.service, 'test-flask')
        self.assertEqual(req_span.name, 'flask.request')
        self.assertEqual(req_span.resource, 'GET /')
        self.assertEqual(req_span.span_type, 'web')
        self.assertEqual(req_span.error, 0)
        self.assertIsNone(req_span.parent_id)

        # Request tags
        self.assertEqual(
            set(['flask.version', 'http.url', 'http.method', 'http.status_code',
                 'flask.endpoint', 'flask.url_rule']),
            set(req_span.meta.keys()),
        )
        self.assertEqual(req_span.get_tag('flask.endpoint'), 'index')
        self.assertEqual(req_span.get_tag('flask.url_rule'), '/')
        self.assertEqual(req_span.get_tag('http.method'), 'GET')
        self.assertEqual(req_span.get_tag(http.URL), 'http://localhost/')
        assert_span_http_status_code(req_span, 200)

        # Handler span
        handler_span = spans[4]
        self.assertEqual(handler_span.service, 'test-flask')
        self.assertEqual(handler_span.name, 'tests.contrib.flask_autopatch.test_flask_autopatch.index')
        self.assertEqual(handler_span.resource, '/')
        self.assertEqual(req_span.error, 0)
