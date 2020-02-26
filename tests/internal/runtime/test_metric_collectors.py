from ddtrace.internal.runtime.metric_collectors import (
    RuntimeMetricCollector,
    GCRuntimeMetricCollector,
    PSUtilRuntimeMetricCollector,
)

from ddtrace.internal.runtime.constants import (
    GC_COUNT_GEN0,
    GC_RUNTIME_METRICS,
    PSUTIL_RUNTIME_METRICS,
)
from ...base import BaseTestCase


class TestRuntimeMetricCollector(BaseTestCase):
    def test_failed_module_load_collect(self):
        """Attempts to collect from a collector when it has failed to load its
        module should return no metrics gracefully.
        """
        class A(RuntimeMetricCollector):
            required_modules = ['moduleshouldnotexist']

            def collect_fn(self, keys):
                return {'k': 'v'}

        self.assertIsNotNone(A().collect(), 'collect should return valid metrics')


class TestPSUtilRuntimeMetricCollector(BaseTestCase):
    def test_metrics(self):
        collector = PSUtilRuntimeMetricCollector()
        for (key, value) in collector.collect(PSUTIL_RUNTIME_METRICS):
            self.assertIsNotNone(value)


class TestGCRuntimeMetricCollector(BaseTestCase):
    def test_metrics(self):
        collector = GCRuntimeMetricCollector()
        for (key, value) in collector.collect(GC_RUNTIME_METRICS):
            self.assertIsNotNone(value)

    def test_gen1_changes(self):
        # disable gc
        import gc
        gc.disable()

        # start collector and get current gc counts
        collector = GCRuntimeMetricCollector()
        gc.collect()
        start = gc.get_count()

        # create reference
        a = []
        collected = collector.collect([GC_COUNT_GEN0])
        self.assertGreater(collected[0][1], start[0])

        # delete reference and collect
        del a
        gc.collect()
        collected_after = collector.collect([GC_COUNT_GEN0])
        assert len(collected_after) == 1
        assert collected_after[0][0] == 'runtime.python.gc.count.gen0'
        assert isinstance(collected_after[0][1], int)
