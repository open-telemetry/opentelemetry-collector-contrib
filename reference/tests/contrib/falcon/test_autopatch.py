from falcon import testing

import ddtrace

from ...base import BaseTracerTestCase
from .app import get_app
from .test_suite import FalconTestCase


class AutoPatchTestCase(BaseTracerTestCase, testing.TestCase, FalconTestCase):

    # Added because falcon 1.3 and 1.4 test clients (falcon.testing.client.TestClient) expect this property to be
    # defined. It would be initialized in the constructor, but we call it here like in 'TestClient.__init__(self, None)'
    # because falcon 1.0.x does not have such module and would fail. Once we stop supporting falcon 1.0.x then we can
    # use the cleaner __init__ invocation
    _default_headers = None

    def setUp(self):
        super(AutoPatchTestCase, self).setUp()

        self._service = 'my-falcon'

        # Since most integrations do `from ddtrace import tracer` we cannot update do `ddtrace.tracer = self.tracer`
        self.original_writer = ddtrace.tracer.writer
        ddtrace.tracer.writer = self.tracer.writer
        self.tracer = ddtrace.tracer

        # build a test app without adding a tracer middleware;
        # reconfigure the global tracer since the autopatch mode
        # uses it
        self.api = get_app(tracer=None)

    def tearDown(self):
        super(AutoPatchTestCase, self).tearDown()

        ddtrace.tracer.writer = self.original_writer
