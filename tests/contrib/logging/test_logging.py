import logging

from ddtrace.helpers import get_correlation_ids
from ddtrace.compat import StringIO
from ddtrace.contrib.logging import patch, unpatch
from ddtrace.vendor import wrapt

from ...base import BaseTracerTestCase


logger = logging.getLogger()
logger.level = logging.INFO


def capture_function_log(func, fmt):
    # add stream handler to capture output
    out = StringIO()
    sh = logging.StreamHandler(out)

    try:
        formatter = logging.Formatter(fmt)
        sh.setFormatter(formatter)
        logger.addHandler(sh)
        result = func()
    finally:
        logger.removeHandler(sh)

    return out.getvalue().strip(), result


class LoggingTestCase(BaseTracerTestCase):
    def setUp(self):
        patch()
        super(LoggingTestCase, self).setUp()

    def tearDown(self):
        unpatch()
        super(LoggingTestCase, self).tearDown()

    def test_patch(self):
        """
        Confirm patching was successful
        """
        patch()
        log = logging.getLogger()
        self.assertTrue(isinstance(log.makeRecord, wrapt.BoundFunctionWrapper))

    def test_log_trace(self):
        """
        Check logging patched and formatter including trace info
        """
        @self.tracer.wrap()
        def func():
            logger.info('Hello!')
            return get_correlation_ids(tracer=self.tracer)

        with self.override_config('logging', dict(tracer=self.tracer)):
            # with format string for trace info
            output, result = capture_function_log(
                func,
                fmt='%(message)s - dd.trace_id=%(dd.trace_id)s dd.span_id=%(dd.span_id)s',
            )
            self.assertEqual(
                output,
                'Hello! - dd.trace_id={} dd.span_id={}'.format(*result),
            )

            # without format string
            output, _ = capture_function_log(
                func,
                fmt='%(message)s',
            )
            self.assertEqual(
                output,
                'Hello!',
            )

    def test_log_no_trace(self):
        """
        Check traced funclogging patched and formatter not including trace info
        """
        def func():
            logger.info('Hello!')
            return get_correlation_ids()

        with self.override_config('logging', dict(tracer=self.tracer)):
            # with format string for trace info
            output, _ = capture_function_log(
                func,
                fmt='%(message)s - dd.trace_id=%(dd.trace_id)s dd.span_id=%(dd.span_id)s',
            )
            self.assertEqual(
                output,
                'Hello! - dd.trace_id=0 dd.span_id=0',
            )
