import time
import concurrent

from ddtrace.contrib.futures import patch, unpatch

from tests.opentracer.utils import init_tracer
from ...base import BaseTracerTestCase


class PropagationTestCase(BaseTracerTestCase):
    """Ensures the Context Propagation works between threads
    when the ``futures`` library is used, or when the
    ``concurrent`` module is available (Python 3 only)
    """
    def setUp(self):
        super(PropagationTestCase, self).setUp()

        # instrument ``concurrent``
        patch()

    def tearDown(self):
        # remove instrumentation
        unpatch()

        super(PropagationTestCase, self).tearDown()

    def test_propagation(self):
        # it must propagate the tracing context if available

        def fn():
            # an active context must be available
            # DEV: With `ContextManager` `.active()` will never be `None`
            self.assertIsNotNone(self.tracer.context_provider.active())
            with self.tracer.trace('executor.thread'):
                return 42

        with self.override_global_tracer():
            with self.tracer.trace('main.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    future = executor.submit(fn)
                    result = future.result()
                    # assert the right result
                    self.assertEqual(result, 42)

        # the trace must be completed
        self.assert_structure(
            dict(name='main.thread'),
            (
                dict(name='executor.thread'),
            ),
        )

    def test_propagation_with_params(self):
        # instrumentation must proxy arguments if available

        def fn(value, key=None):
            # an active context must be available
            # DEV: With `ThreadLocalContext` `.active()` will never be `None`
            self.assertIsNotNone(self.tracer.context_provider.active())
            with self.tracer.trace('executor.thread'):
                return value, key

        with self.override_global_tracer():
            with self.tracer.trace('main.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    future = executor.submit(fn, 42, 'CheeseShop')
                    value, key = future.result()
                    # assert the right result
                    self.assertEqual(value, 42)
                    self.assertEqual(key, 'CheeseShop')

        # the trace must be completed
        self.assert_structure(
            dict(name='main.thread'),
            (
                dict(name='executor.thread'),
            ),
        )

    def test_disabled_instrumentation(self):
        # it must not propagate if the module is disabled
        unpatch()

        def fn():
            # an active context must be available
            # DEV: With `ThreadLocalContext` `.active()` will never be `None`
            self.assertIsNotNone(self.tracer.context_provider.active())
            with self.tracer.trace('executor.thread'):
                return 42

        with self.override_global_tracer():
            with self.tracer.trace('main.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    future = executor.submit(fn)
                    result = future.result()
                    # assert the right result
                    self.assertEqual(result, 42)

        # we provide two different traces
        self.assert_span_count(2)

        # Retrieve the root spans (no parents)
        # DEV: Results are sorted based on root span start time
        traces = self.get_root_spans()
        self.assertEqual(len(traces), 2)

        traces[0].assert_structure(dict(name='main.thread'))
        traces[1].assert_structure(dict(name='executor.thread'))

    def test_double_instrumentation(self):
        # double instrumentation must not happen
        patch()

        def fn():
            with self.tracer.trace('executor.thread'):
                return 42

        with self.override_global_tracer():
            with self.tracer.trace('main.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    future = executor.submit(fn)
                    result = future.result()
                    # assert the right result
                    self.assertEqual(result, 42)

        # the trace must be completed
        self.assert_structure(
            dict(name='main.thread'),
            (
                dict(name='executor.thread'),
            ),
        )

    def test_no_parent_span(self):
        def fn():
            with self.tracer.trace('executor.thread'):
                return 42

        with self.override_global_tracer():
            with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                future = executor.submit(fn)
                result = future.result()
                # assert the right result
                self.assertEqual(result, 42)

        # the trace must be completed
        self.assert_structure(dict(name='executor.thread'))

    def test_multiple_futures(self):
        def fn():
            with self.tracer.trace('executor.thread'):
                return 42

        with self.override_global_tracer():
            with self.tracer.trace('main.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
                    futures = [executor.submit(fn) for _ in range(4)]
                    for future in futures:
                        result = future.result()
                        # assert the right result
                        self.assertEqual(result, 42)

        # the trace must be completed
        self.assert_structure(
            dict(name='main.thread'),
            (
                dict(name='executor.thread'),
                dict(name='executor.thread'),
                dict(name='executor.thread'),
                dict(name='executor.thread'),
            ),
        )

    def test_multiple_futures_no_parent(self):
        def fn():
            with self.tracer.trace('executor.thread'):
                return 42

        with self.override_global_tracer():
            with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
                futures = [executor.submit(fn) for _ in range(4)]
                for future in futures:
                    result = future.result()
                    # assert the right result
                    self.assertEqual(result, 42)

        # the trace must be completed
        self.assert_span_count(4)
        traces = self.get_root_spans()
        self.assertEqual(len(traces), 4)
        for trace in traces:
            trace.assert_structure(dict(name='executor.thread'))

    def test_nested_futures(self):
        def fn2():
            with self.tracer.trace('nested.thread'):
                return 42

        def fn():
            with self.tracer.trace('executor.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    future = executor.submit(fn2)
                    result = future.result()
                    self.assertEqual(result, 42)
                    return result

        with self.override_global_tracer():
            with self.tracer.trace('main.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    future = executor.submit(fn)
                    result = future.result()
                    # assert the right result
                    self.assertEqual(result, 42)

        # the trace must be completed
        self.assert_span_count(3)
        self.assert_structure(
            dict(name='main.thread'),
            (
                (
                    dict(name='executor.thread'),
                    (
                        dict(name='nested.thread'),
                    ),
                ),
            ),
        )

    def test_multiple_nested_futures(self):
        def fn2():
            with self.tracer.trace('nested.thread'):
                return 42

        def fn():
            with self.tracer.trace('executor.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    futures = [executor.submit(fn2) for _ in range(4)]
                    for future in futures:
                        result = future.result()
                        self.assertEqual(result, 42)
                        return result

        with self.override_global_tracer():
            with self.tracer.trace('main.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    futures = [executor.submit(fn) for _ in range(4)]
                    for future in futures:
                        result = future.result()
                        # assert the right result
                        self.assertEqual(result, 42)

        # the trace must be completed
        self.assert_structure(
            dict(name='main.thread'),
            (
                (
                    dict(name='executor.thread'),
                    (
                        dict(name='nested.thread'),
                    ) * 4,
                ),
            ) * 4,
        )

    def test_multiple_nested_futures_no_parent(self):
        def fn2():
            with self.tracer.trace('nested.thread'):
                return 42

        def fn():
            with self.tracer.trace('executor.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    futures = [executor.submit(fn2) for _ in range(4)]
                    for future in futures:
                        result = future.result()
                        self.assertEqual(result, 42)
                        return result

        with self.override_global_tracer():
            with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                futures = [executor.submit(fn) for _ in range(4)]
                for future in futures:
                    result = future.result()
                    # assert the right result
                    self.assertEqual(result, 42)

        # the trace must be completed
        traces = self.get_root_spans()
        self.assertEqual(len(traces), 4)

        for trace in traces:
            trace.assert_structure(
                dict(name='executor.thread'),
                (
                    dict(name='nested.thread'),
                ) * 4,
            )

    def test_send_trace_when_finished(self):
        # it must send the trace only when all threads are finished

        def fn():
            with self.tracer.trace('executor.thread'):
                # wait before returning
                time.sleep(0.05)
                return 42

        with self.override_global_tracer():
            with self.tracer.trace('main.thread'):
                # don't wait for the execution
                executor = concurrent.futures.ThreadPoolExecutor(max_workers=2)
                future = executor.submit(fn)
                time.sleep(0.01)

        # assert main thread span is fniished first
        self.assert_span_count(1)
        self.assert_structure(dict(name='main.thread'))

        # then wait for the second thread and send the trace
        result = future.result()
        self.assertEqual(result, 42)

        self.assert_span_count(2)
        self.assert_structure(
            dict(name='main.thread'),
            (
                dict(name='executor.thread'),
            ),
        )

    def test_propagation_ot(self):
        """OpenTracing version of test_propagation."""
        # it must propagate the tracing context if available
        ot_tracer = init_tracer('my_svc', self.tracer)

        def fn():
            # an active context must be available
            self.assertTrue(self.tracer.context_provider.active() is not None)
            with self.tracer.trace('executor.thread'):
                return 42

        with self.override_global_tracer():
            with ot_tracer.start_active_span('main.thread'):
                with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
                    future = executor.submit(fn)
                    result = future.result()
                    # assert the right result
                    self.assertEqual(result, 42)

        # the trace must be completed
        self.assert_structure(
            dict(name='main.thread'),
            (
                dict(name='executor.thread'),
            ),
        )
