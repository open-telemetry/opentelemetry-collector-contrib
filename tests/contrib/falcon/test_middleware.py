from falcon import testing

from .app import get_app
from .test_suite import FalconTestCase
from ...base import BaseTracerTestCase


class MiddlewareTestCase(BaseTracerTestCase, testing.TestCase, FalconTestCase):
    """Executes tests using the manual instrumentation so a middleware
    is explicitly added.
    """
    def setUp(self):
        super(MiddlewareTestCase, self).setUp()

        # build a test app with a dummy tracer
        self._service = 'falcon'
        self.api = get_app(tracer=self.tracer)
