import asyncio

from aiohttp.test_utils import AioHTTPTestCase

from .app.web import setup_app
from ...base import BaseTracerTestCase


class TraceTestCase(BaseTracerTestCase, AioHTTPTestCase):
    """
    Base class that provides a valid ``aiohttp`` application with
    the async tracer.
    """
    def enable_tracing(self):
        pass

    def disable_tracing(self):
        pass

    def tearDown(self):
        # unpatch the aiohttp_jinja2 module
        super(TraceTestCase, self).tearDown()
        self.disable_tracing()

    def get_app(self, loop=None):
        """
        Override the get_app method to return the test application
        """
        # aiohttp 2.0+ stores the loop instance in self.loop; for
        # backward compatibility, we should expect a `loop` argument
        loop = loop or self.loop
        # create the app with the testing loop
        self.app = setup_app(loop)
        asyncio.set_event_loop(loop)
        # trace the app
        self.enable_tracing()
        return self.app
