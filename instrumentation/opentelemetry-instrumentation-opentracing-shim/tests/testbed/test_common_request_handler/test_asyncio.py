from __future__ import print_function

import asyncio

from opentracing.ext import tags

from ..otel_ot_shim_tracer import MockTracer
from ..testcase import OpenTelemetryTestCase
from ..utils import get_logger, get_one_by_operation_name, stop_loop_when
from .request_handler import RequestHandler

logger = get_logger(__name__)


class Client:
    def __init__(self, request_handler, loop):
        self.request_handler = request_handler
        self.loop = loop

    async def send_task(self, message):
        request_context = {}

        async def before_handler():
            self.request_handler.before_request(message, request_context)

        async def after_handler():
            self.request_handler.after_request(message, request_context)

        await before_handler()
        await after_handler()

        return "%s::response" % message

    def send(self, message):
        return self.send_task(message)

    def send_sync(self, message):
        return self.loop.run_until_complete(self.send_task(message))


class TestAsyncio(OpenTelemetryTestCase):
    """
    There is only one instance of 'RequestHandler' per 'Client'. Methods of
    'RequestHandler' are executed in different Tasks, and no Span propagation
    among them is done automatically.
    Therefore we cannot use current active span and activate span.
    So one issue here is setting correct parent span.
    """

    def setUp(self):
        self.tracer = MockTracer()
        self.loop = asyncio.get_event_loop()
        self.client = Client(RequestHandler(self.tracer), self.loop)

    def test_two_callbacks(self):
        res_future1 = self.loop.create_task(self.client.send("message1"))
        res_future2 = self.loop.create_task(self.client.send("message2"))

        stop_loop_when(
            self.loop,
            lambda: len(self.tracer.finished_spans()) >= 2,
            timeout=5.0,
        )
        self.loop.run_forever()

        self.assertEqual("message1::response", res_future1.result())
        self.assertEqual("message2::response", res_future2.result())

        spans = self.tracer.finished_spans()
        self.assertEqual(len(spans), 2)

        for span in spans:
            self.assertEqual(
                span.attributes.get(tags.SPAN_KIND, None),
                tags.SPAN_KIND_RPC_CLIENT,
            )

        self.assertNotSameTrace(spans[0], spans[1])
        self.assertIsNone(spans[0].parent)
        self.assertIsNone(spans[1].parent)

    def test_parent_not_picked(self):
        """Active parent should not be picked up by child."""

        async def do_task():
            with self.tracer.start_active_span("parent"):
                response = await self.client.send_task("no_parent")
                self.assertEqual("no_parent::response", response)

        self.loop.run_until_complete(do_task())

        spans = self.tracer.finished_spans()
        self.assertEqual(len(spans), 2)

        child_span = get_one_by_operation_name(spans, "send")
        self.assertIsNotNone(child_span)

        parent_span = get_one_by_operation_name(spans, "parent")
        self.assertIsNotNone(parent_span)

        # Here check that there is no parent-child relation.
        self.assertIsNotChildOf(child_span, parent_span)

    def test_good_solution_to_set_parent(self):
        """Asyncio and contextvars are integrated, in this case it is not needed
        to activate current span by hand.
        """

        async def do_task():
            with self.tracer.start_active_span("parent"):
                # Set ignore_active_span to False indicating that the
                # framework will do it for us.
                req_handler = RequestHandler(
                    self.tracer, ignore_active_span=False,
                )
                client = Client(req_handler, self.loop)
                response = await client.send_task("correct_parent")

                self.assertEqual("correct_parent::response", response)

            # Send second request, now there is no active parent,
            # but it will be set, ups
            response = await client.send_task("wrong_parent")
            self.assertEqual("wrong_parent::response", response)

        self.loop.run_until_complete(do_task())

        spans = self.tracer.finished_spans()
        self.assertEqual(len(spans), 3)

        spans = sorted(spans, key=lambda x: x.start_time)
        parent_span = get_one_by_operation_name(spans, "parent")
        self.assertIsNotNone(parent_span)

        self.assertIsChildOf(spans[1], parent_span)
        self.assertIsNotChildOf(spans[2], parent_span)
