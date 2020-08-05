from __future__ import print_function

from concurrent.futures import ThreadPoolExecutor

from ..otel_ot_shim_tracer import MockTracer
from ..testcase import OpenTelemetryTestCase


class TestThreads(OpenTelemetryTestCase):
    def setUp(self):
        self.tracer = MockTracer()
        # use max_workers=3 as a general example even if only one would suffice
        self.executor = ThreadPoolExecutor(max_workers=3)

    def test_main(self):
        # Start an isolated task and query for its result -and finish it-
        # in another task/thread
        span = self.tracer.start_span("initial")
        self.submit_another_task(span)

        self.executor.shutdown(True)

        spans = self.tracer.finished_spans()
        self.assertEqual(len(spans), 3)
        self.assertNamesEqual(spans, ["initial", "subtask", "task"])

        # task/subtask are part of the same trace,
        # and subtask is a child of task
        self.assertSameTrace(spans[1], spans[2])
        self.assertIsChildOf(spans[1], spans[2])

        # initial task is not related in any way to those two tasks
        self.assertNotSameTrace(spans[0], spans[1])
        self.assertEqual(spans[0].parent, None)
        self.assertEqual(spans[2].parent, None)

    def task(self, span):
        # Create a new Span for this task
        with self.tracer.start_active_span("task"):

            with self.tracer.scope_manager.activate(span, True):
                # Simulate work strictly related to the initial Span
                pass

            # Use the task span as parent of a new subtask
            with self.tracer.start_active_span("subtask"):
                pass

    def submit_another_task(self, span):
        self.executor.submit(self.task, span)
