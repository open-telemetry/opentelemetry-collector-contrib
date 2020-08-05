from __future__ import print_function

import asyncio

import opentracing
from opentracing.ext import tags

from ..otel_ot_shim_tracer import MockTracer
from ..testcase import OpenTelemetryTestCase
from ..utils import get_logger, get_one_by_tag, stop_loop_when

logger = get_logger(__name__)


class Server:
    def __init__(self, *args, **kwargs):
        tracer = kwargs.pop("tracer")
        queue = kwargs.pop("queue")
        super(Server, self).__init__(*args, **kwargs)

        self.tracer = tracer
        self.queue = queue

    async def run(self):
        value = await self.queue.get()
        self.process(value)

    def process(self, message):
        logger.info("Processing message in server")

        ctx = self.tracer.extract(opentracing.Format.TEXT_MAP, message)
        with self.tracer.start_active_span("receive", child_of=ctx) as scope:
            scope.span.set_tag(tags.SPAN_KIND, tags.SPAN_KIND_RPC_SERVER)


class Client:
    def __init__(self, tracer, queue):
        self.tracer = tracer
        self.queue = queue

    async def send(self):
        with self.tracer.start_active_span("send") as scope:
            scope.span.set_tag(tags.SPAN_KIND, tags.SPAN_KIND_RPC_CLIENT)

            message = {}
            self.tracer.inject(
                scope.span.context, opentracing.Format.TEXT_MAP, message
            )
            await self.queue.put(message)

        logger.info("Sent message from client")


class TestAsyncio(OpenTelemetryTestCase):
    def setUp(self):
        self.tracer = MockTracer()
        self.queue = asyncio.Queue()
        self.loop = asyncio.get_event_loop()
        self.server = Server(tracer=self.tracer, queue=self.queue)

    def test(self):
        client = Client(self.tracer, self.queue)
        self.loop.create_task(self.server.run())
        self.loop.create_task(client.send())

        stop_loop_when(
            self.loop,
            lambda: len(self.tracer.finished_spans()) >= 2,
            timeout=5.0,
        )
        self.loop.run_forever()

        spans = self.tracer.finished_spans()
        self.assertIsNotNone(
            get_one_by_tag(spans, tags.SPAN_KIND, tags.SPAN_KIND_RPC_SERVER)
        )
        self.assertIsNotNone(
            get_one_by_tag(spans, tags.SPAN_KIND, tags.SPAN_KIND_RPC_CLIENT)
        )
