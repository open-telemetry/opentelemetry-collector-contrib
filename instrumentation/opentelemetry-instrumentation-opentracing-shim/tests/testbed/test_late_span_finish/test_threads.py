from __future__ import print_function

import time
from concurrent.futures import ThreadPoolExecutor

from ..otel_ot_shim_tracer import MockTracer
from ..testcase import OpenTelemetryTestCase


class TestThreads(OpenTelemetryTestCase):
    def setUp(self):
        self.tracer = MockTracer()
        self.executor = ThreadPoolExecutor(max_workers=3)

    def test_main(self):
        # Create a Span and use it as (explicit) parent of a pair of subtasks.
        parent_span = self.tracer.start_span("parent")
        self.submit_subtasks(parent_span)

        # Wait for the threadpool to be done.
        self.executor.shutdown(True)

        # Late-finish the parent Span now.
        parent_span.finish()

        spans = self.tracer.finished_spans()
        self.assertEqual(len(spans), 3)
        self.assertNamesEqual(spans, ["task1", "task2", "parent"])

        for idx in range(2):
            self.assertSameTrace(spans[idx], spans[-1])
            self.assertIsChildOf(spans[idx], spans[-1])
            self.assertTrue(spans[idx].end_time <= spans[-1].end_time)

    # Fire away a few subtasks, passing a parent Span whose lifetime
    # is not tied at all to the children.
    def submit_subtasks(self, parent_span):
        def task(name, interval):
            with self.tracer.scope_manager.activate(parent_span, False):
                with self.tracer.start_active_span(name):
                    time.sleep(interval)

        self.executor.submit(task, "task1", 0.1)
        self.executor.submit(task, "task2", 0.3)
