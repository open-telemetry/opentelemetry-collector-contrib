import unittest

import opentelemetry.trace as trace_api


# pylint: disable=C0103
class OpenTelemetryTestCase(unittest.TestCase):
    def assertSameTrace(self, spanA, spanB):
        return self.assertEqual(spanA.context.trace_id, spanB.context.trace_id)

    def assertNotSameTrace(self, spanA, spanB):
        return self.assertNotEqual(
            spanA.context.trace_id, spanB.context.trace_id
        )

    def assertIsChildOf(self, spanA, spanB):
        # spanA is child of spanB
        self.assertIsNotNone(spanA.parent)

        ctxA = spanA.parent
        if isinstance(spanA.parent, trace_api.Span):
            ctxA = spanA.parent.context

        ctxB = spanB
        if isinstance(ctxB, trace_api.Span):
            ctxB = spanB.context

        return self.assertEqual(ctxA.span_id, ctxB.span_id)

    def assertIsNotChildOf(self, spanA, spanB):
        # spanA is NOT child of spanB
        if spanA.parent is None:
            return

        ctxA = spanA.parent
        if isinstance(spanA.parent, trace_api.Span):
            ctxA = spanA.parent.context

        ctxB = spanB
        if isinstance(ctxB, trace_api.Span):
            ctxB = spanB.context

        self.assertNotEqual(ctxA.span_id, ctxB.span_id)

    def assertNamesEqual(self, spans, names):
        self.assertEqual(list(map(lambda x: x.name, spans)), names)
