from celery import Celery

from ddtrace import Pin
from ddtrace.compat import PY2
from ddtrace.contrib.celery import patch, unpatch

from ..config import REDIS_CONFIG
from ...base import BaseTracerTestCase


REDIS_URL = 'redis://127.0.0.1:{port}'.format(port=REDIS_CONFIG['port'])
BROKER_URL = '{redis}/{db}'.format(redis=REDIS_URL, db=0)
BACKEND_URL = '{redis}/{db}'.format(redis=REDIS_URL, db=1)


class CeleryBaseTestCase(BaseTracerTestCase):
    """Test case that handles a full fledged Celery application with a
    custom tracer. It patches the new Celery application.
    """

    def setUp(self):
        super(CeleryBaseTestCase, self).setUp()

        # instrument Celery and create an app with Broker and Result backends
        patch()
        self.pin = Pin(service='celery-unittest', tracer=self.tracer)
        self.app = Celery('celery.test_app', broker=BROKER_URL, backend=BACKEND_URL)
        # override pins to use our Dummy Tracer
        Pin.override(self.app, tracer=self.tracer)

    def tearDown(self):
        # remove instrumentation from Celery
        unpatch()
        self.app = None

        super(CeleryBaseTestCase, self).tearDown()

    def assert_items_equal(self, a, b):
        if PY2:
            return self.assertItemsEqual(a, b)
        return self.assertCountEqual(a, b)
