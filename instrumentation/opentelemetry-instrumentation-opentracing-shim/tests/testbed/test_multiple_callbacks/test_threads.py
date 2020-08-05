from __future__ import print_function

import random
import time
from concurrent.futures import ThreadPoolExecutor

from ..otel_ot_shim_tracer import MockTracer
from ..testcase import OpenTelemetryTestCase
from ..utils import RefCount, get_logger

random.seed()
logger = get_logger(__name__)


class TestThreads(OpenTelemetryTestCase):
    def setUp(self):
        self.tracer = MockTracer()
        self.executor = ThreadPoolExecutor(max_workers=3)

    def test_main(self):
        try:
            scope = self.tracer.start_active_span(
                "parent", finish_on_close=False
            )
            scope.span.ref_count = RefCount(1)
            self.submit_callbacks(scope.span)
        finally:
            scope.close()
            if scope.span.ref_count.decr() == 0:
                scope.span.finish()

        self.executor.shutdown(True)

        spans = self.tracer.finished_spans()
        self.assertEqual(len(spans), 4)
        self.assertNamesEqual(spans, ["task", "task", "task", "parent"])

        for idx in range(3):
            self.assertSameTrace(spans[idx], spans[-1])
            self.assertIsChildOf(spans[idx], spans[-1])

    def task(self, interval, parent_span):
        logger.info("Starting task")

        try:
            scope = self.tracer.scope_manager.activate(parent_span, False)
            with self.tracer.start_active_span("task"):
                time.sleep(interval)
        finally:
            scope.close()
            if parent_span.ref_count.decr() == 0:
                parent_span.finish()

    def submit_callbacks(self, parent_span):
        for _ in range(3):
            parent_span.ref_count.incr()
            self.executor.submit(
                self.task, 0.1 + random.randint(200, 500) * 0.001, parent_span
            )
