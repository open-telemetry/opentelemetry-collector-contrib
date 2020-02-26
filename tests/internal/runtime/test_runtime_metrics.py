import time

import mock

from ddtrace.internal.runtime.runtime_metrics import (
    RuntimeTags,
    RuntimeMetrics,
)
from ddtrace.internal.runtime.constants import (
    DEFAULT_RUNTIME_METRICS,
    GC_COUNT_GEN0,
    SERVICE,
    ENV
)

from ...base import (
    BaseTestCase,
    BaseTracerTestCase,
)


class TestRuntimeTags(BaseTracerTestCase):
    def test_all_tags(self):
        with self.override_global_tracer():
            with self.trace('test', service='test'):
                tags = set([k for (k, v) in RuntimeTags()])
                assert SERVICE in tags
                # no env set by default
                assert ENV not in tags

    def test_one_tag(self):
        with self.override_global_tracer():
            with self.trace('test', service='test'):
                tags = [k for (k, v) in RuntimeTags(enabled=[SERVICE])]
                self.assertEqual(tags, [SERVICE])

    def test_env_tag(self):
        def filter_only_env_tags(tags):
            return [
                (k, v)
                for (k, v) in RuntimeTags()
                if k == 'env'
            ]

        with self.override_global_tracer():
            # first without env tag set in tracer
            with self.trace('first-test', service='test'):
                tags = filter_only_env_tags(RuntimeTags())
                assert tags == []

            # then with an env tag set
            self.tracer.set_tags({'env': 'tests.dog'})
            with self.trace('second-test', service='test'):
                tags = filter_only_env_tags(RuntimeTags())
                assert tags == [('env', 'tests.dog')]

            # check whether updating env works
            self.tracer.set_tags({'env': 'staging.dog'})
            with self.trace('third-test', service='test'):
                tags = filter_only_env_tags(RuntimeTags())
                assert tags == [('env', 'staging.dog')]


class TestRuntimeMetrics(BaseTestCase):
    def test_all_metrics(self):
        metrics = set([k for (k, v) in RuntimeMetrics()])
        self.assertSetEqual(metrics, DEFAULT_RUNTIME_METRICS)

    def test_one_metric(self):
        metrics = [k for (k, v) in RuntimeMetrics(enabled=[GC_COUNT_GEN0])]
        self.assertEqual(metrics, [GC_COUNT_GEN0])


class TestRuntimeWorker(BaseTracerTestCase):
    def test_tracer_metrics(self):
        # Mock socket.socket to hijack the dogstatsd socket
        with mock.patch('socket.socket'):
            # configure tracer for runtime metrics
            self.tracer._RUNTIME_METRICS_INTERVAL = 1. / 4
            self.tracer.configure(collect_metrics=True)
            self.tracer.set_tags({'env': 'tests.dog'})

            with self.override_global_tracer(self.tracer):
                root = self.start_span('parent', service='parent')
                context = root.context
                self.start_span('child', service='child', child_of=context)

            time.sleep(self.tracer._RUNTIME_METRICS_INTERVAL * 2)

            # Get the socket before it disappears
            statsd_socket = self.tracer._dogstatsd_client.socket
            # now stop collection
            self.tracer.configure(collect_metrics=False)

        received = [
            s.args[0].decode('utf-8') for s in statsd_socket.send.mock_calls
        ]

        # we expect more than one flush since it is also called on shutdown
        assert len(received) > 1

        # expect all metrics in default set are received
        # DEV: dogstatsd gauges in form "{metric_name}:{metric_value}|g#t{tag_name}:{tag_value},..."
        self.assertSetEqual(
            set([gauge.split(':')[0]
                 for packet in received
                 for gauge in packet.split('\n')]),
            DEFAULT_RUNTIME_METRICS
        )

        # check to last set of metrics returned to confirm tags were set
        for gauge in received[-len(DEFAULT_RUNTIME_METRICS):]:
            self.assertRegexpMatches(gauge, 'service:parent')
            self.assertRegexpMatches(gauge, 'service:child')
            self.assertRegexpMatches(gauge, 'env:tests.dog')
            self.assertRegexpMatches(gauge, 'lang_interpreter:')
            self.assertRegexpMatches(gauge, 'lang_version:')
            self.assertRegexpMatches(gauge, 'lang:')
            self.assertRegexpMatches(gauge, 'tracer_version:')
