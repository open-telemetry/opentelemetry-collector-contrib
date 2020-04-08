from ddtrace import Pin
from ddtrace.contrib.flask import patch, unpatch
import flask
from ddtrace.vendor import wrapt

from ...base import BaseTracerTestCase


class BaseFlaskTestCase(BaseTracerTestCase):
    def setUp(self):
        super(BaseFlaskTestCase, self).setUp()

        patch()

        self.app = flask.Flask(__name__, template_folder="test_templates/")
        self.client = self.app.test_client()
        Pin.override(self.app, tracer=self.tracer)

    def tearDown(self):
        # Remove any remaining spans
        self.tracer.writer.pop()

        # Unpatch Flask
        unpatch()

    def get_spans(self):
        return self.tracer.writer.pop()

    def assert_is_wrapped(self, obj):
        self.assertTrue(isinstance(obj, wrapt.ObjectProxy), "{} is not wrapped".format(obj))

    def assert_is_not_wrapped(self, obj):
        self.assertFalse(isinstance(obj, wrapt.ObjectProxy), "{} is wrapped".format(obj))

    def find_span_by_name(self, spans, name, required=True):
        """Helper to find the first span with a given name from a list"""
        span = next((s for s in spans if s.name == name), None)
        if required:
            self.assertIsNotNone(span, "could not find span with name {}".format(name))
        return span

    def find_span_parent(self, spans, span, required=True):
        """Helper to search for a span's parent in a given list of spans"""
        parent = next((s for s in spans if s.span_id == span.parent_id), None)
        if required:
            self.assertIsNotNone(parent, "could not find parent span {}".format(span))
        return parent
