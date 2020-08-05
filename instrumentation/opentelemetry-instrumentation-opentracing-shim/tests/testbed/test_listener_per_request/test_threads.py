from __future__ import print_function

from concurrent.futures import ThreadPoolExecutor

from opentracing.ext import tags

from ..otel_ot_shim_tracer import MockTracer
from ..testcase import OpenTelemetryTestCase
from ..utils import get_one_by_tag
from .response_listener import ResponseListener


class Client:
    def __init__(self, tracer):
        self.tracer = tracer
        self.executor = ThreadPoolExecutor(max_workers=3)

    def _task(self, message, listener):
        # pylint: disable=no-self-use
        res = "%s::response" % message
        listener.on_response(res)
        return res

    def send_sync(self, message):
        span = self.tracer.start_span("send")
        span.set_tag(tags.SPAN_KIND, tags.SPAN_KIND_RPC_CLIENT)

        listener = ResponseListener(span)
        return self.executor.submit(self._task, message, listener).result()


class TestThreads(OpenTelemetryTestCase):
    def setUp(self):
        self.tracer = MockTracer()

    def test_main(self):
        client = Client(self.tracer)
        res = client.send_sync("message")
        self.assertEqual(res, "message::response")

        spans = self.tracer.finished_spans()
        self.assertEqual(len(spans), 1)

        span = get_one_by_tag(spans, tags.SPAN_KIND, tags.SPAN_KIND_RPC_CLIENT)
        self.assertIsNotNone(span)
