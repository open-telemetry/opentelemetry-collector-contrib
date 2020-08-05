from __future__ import absolute_import, print_function

import asyncio

from ..otel_ot_shim_tracer import MockTracer
from ..testcase import OpenTelemetryTestCase


class TestAsyncio(OpenTelemetryTestCase):
    def setUp(self):
        self.tracer = MockTracer()
        self.loop = asyncio.get_event_loop()

    def test_main(self):
        res = self.loop.run_until_complete(self.parent_task("message"))
        self.assertEqual(res, "message::response")

        spans = self.tracer.finished_spans()
        self.assertEqual(len(spans), 2)
        self.assertNamesEqual(spans, ["child", "parent"])
        self.assertIsChildOf(spans[0], spans[1])

    async def parent_task(self, message):  # noqa
        with self.tracer.start_active_span("parent"):
            res = await self.child_task(message)

        return res

    async def child_task(self, message):
        # No need to pass/activate the parent Span, as it stays in the context.
        with self.tracer.start_active_span("child"):
            return "%s::response" % message
