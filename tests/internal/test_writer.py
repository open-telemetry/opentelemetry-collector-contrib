import time

import pytest

import mock

from ddtrace.span import Span
from ddtrace.api import API
from ddtrace.internal.writer import AgentWriter, Q, Empty
from ..base import BaseTestCase


class RemoveAllFilter:
    def __init__(self):
        self.filtered_traces = 0

    def process_trace(self, trace):
        self.filtered_traces += 1
        return None


class KeepAllFilter:
    def __init__(self):
        self.filtered_traces = 0

    def process_trace(self, trace):
        self.filtered_traces += 1
        return trace


class AddTagFilter:
    def __init__(self, tag_name):
        self.tag_name = tag_name
        self.filtered_traces = 0

    def process_trace(self, trace):
        self.filtered_traces += 1
        for span in trace:
            span.set_tag(self.tag_name, "A value")
        return trace


class DummyAPI(API):
    def __init__(self):
        # Call API.__init__ to setup required properties
        super(DummyAPI, self).__init__(hostname="localhost", port=8126)

        self.traces = []

    def send_traces(self, traces):
        responses = []
        for trace in traces:
            self.traces.append(trace)
            response = mock.Mock()
            response.status = 200
            responses.append(response)
        return responses


class FailingAPI(object):
    @staticmethod
    def send_traces(traces):
        return [Exception("oops")]


class AgentWriterTests(BaseTestCase):
    N_TRACES = 11

    def create_worker(self, filters=None, api_class=DummyAPI, enable_stats=False):
        with self.override_global_config(dict(health_metrics_enabled=enable_stats)):
            self.dogstatsd = mock.Mock()
            worker = AgentWriter(dogstatsd=self.dogstatsd, filters=filters)
            worker._STATS_EVERY_INTERVAL = 1
            self.api = api_class()
            worker.api = self.api
            for i in range(self.N_TRACES):
                worker.write(
                    [Span(tracer=None, name="name", trace_id=i, span_id=j, parent_id=j - 1 or None) for j in range(7)]
                )
            worker.stop()
            worker.join()
            return worker

    def test_send_stats(self):
        dogstatsd = mock.Mock()
        worker = AgentWriter(dogstatsd=dogstatsd)
        assert worker._send_stats is False
        with self.override_global_config(dict(health_metrics_enabled=True)):
            assert worker._send_stats is True

        worker = AgentWriter(dogstatsd=None)
        assert worker._send_stats is False
        with self.override_global_config(dict(health_metrics_enabled=True)):
            assert worker._send_stats is False

    def test_filters_keep_all(self):
        filtr = KeepAllFilter()
        self.create_worker([filtr])
        self.assertEqual(len(self.api.traces), self.N_TRACES)
        self.assertEqual(filtr.filtered_traces, self.N_TRACES)

    def test_filters_remove_all(self):
        filtr = RemoveAllFilter()
        self.create_worker([filtr])
        self.assertEqual(len(self.api.traces), 0)
        self.assertEqual(filtr.filtered_traces, self.N_TRACES)

    def test_filters_add_tag(self):
        tag_name = "Tag"
        filtr = AddTagFilter(tag_name)
        self.create_worker([filtr])
        self.assertEqual(len(self.api.traces), self.N_TRACES)
        self.assertEqual(filtr.filtered_traces, self.N_TRACES)
        for trace in self.api.traces:
            for span in trace:
                self.assertIsNotNone(span.get_tag(tag_name))

    def test_filters_short_circuit(self):
        filtr = KeepAllFilter()
        filters = [RemoveAllFilter(), filtr]
        self.create_worker(filters)
        self.assertEqual(len(self.api.traces), 0)
        self.assertEqual(filtr.filtered_traces, 0)

    def test_no_dogstats(self):
        worker = self.create_worker()
        assert worker._send_stats is False
        assert [] == self.dogstatsd.gauge.mock_calls

    def test_dogstatsd(self):
        self.create_worker(enable_stats=True)
        assert [
            mock.call("datadog.tracer.heartbeat", 1),
            mock.call("datadog.tracer.queue.max_length", 1000),
        ] == self.dogstatsd.gauge.mock_calls

        assert [
            mock.call("datadog.tracer.flushes"),
            mock.call("datadog.tracer.flush.traces.total", 11, tags=None),
            mock.call("datadog.tracer.flush.spans.total", 77, tags=None),
            mock.call("datadog.tracer.flush.traces_filtered.total", 0, tags=None),
            mock.call("datadog.tracer.api.requests.total", 11, tags=None),
            mock.call("datadog.tracer.api.errors.total", 0, tags=None),
            mock.call("datadog.tracer.api.responses.total", 11, tags=["status:200"]),
            mock.call("datadog.tracer.queue.dropped.traces", 0),
            mock.call("datadog.tracer.queue.enqueued.traces", 11),
            mock.call("datadog.tracer.queue.enqueued.spans", 77),
            mock.call("datadog.tracer.shutdown"),
        ] == self.dogstatsd.increment.mock_calls

        histogram_calls = [
            mock.call("datadog.tracer.flush.traces", 11, tags=None),
            mock.call("datadog.tracer.flush.spans", 77, tags=None),
            mock.call("datadog.tracer.flush.traces_filtered", 0, tags=None),
            mock.call("datadog.tracer.api.requests", 11, tags=None),
            mock.call("datadog.tracer.api.errors", 0, tags=None),
            mock.call("datadog.tracer.api.responses", 11, tags=["status:200"]),
        ]
        if hasattr(time, "thread_time"):
            histogram_calls.append(mock.call("datadog.tracer.writer.cpu_time", mock.ANY))

        assert histogram_calls == self.dogstatsd.histogram.mock_calls

    def test_dogstatsd_failing_api(self):
        self.create_worker(api_class=FailingAPI, enable_stats=True)
        assert [
            mock.call("datadog.tracer.heartbeat", 1),
            mock.call("datadog.tracer.queue.max_length", 1000),
        ] == self.dogstatsd.gauge.mock_calls

        assert [
            mock.call("datadog.tracer.flushes"),
            mock.call("datadog.tracer.flush.traces.total", 11, tags=None),
            mock.call("datadog.tracer.flush.spans.total", 77, tags=None),
            mock.call("datadog.tracer.flush.traces_filtered.total", 0, tags=None),
            mock.call("datadog.tracer.api.requests.total", 1, tags=None),
            mock.call("datadog.tracer.api.errors.total", 1, tags=None),
            mock.call("datadog.tracer.queue.dropped.traces", 0),
            mock.call("datadog.tracer.queue.enqueued.traces", 11),
            mock.call("datadog.tracer.queue.enqueued.spans", 77),
            mock.call("datadog.tracer.shutdown"),
        ] == self.dogstatsd.increment.mock_calls

        histogram_calls = [
            mock.call("datadog.tracer.flush.traces", 11, tags=None),
            mock.call("datadog.tracer.flush.spans", 77, tags=None),
            mock.call("datadog.tracer.flush.traces_filtered", 0, tags=None),
            mock.call("datadog.tracer.api.requests", 1, tags=None),
            mock.call("datadog.tracer.api.errors", 1, tags=None),
        ]
        if hasattr(time, "thread_time"):
            histogram_calls.append(mock.call("datadog.tracer.writer.cpu_time", mock.ANY))

        assert histogram_calls == self.dogstatsd.histogram.mock_calls


def test_queue_full():
    q = Q(maxsize=3)
    q.put([1])
    q.put(2)
    q.put([3])
    q.put([4, 4])
    assert list(q.queue) == [[1], 2, [4, 4]] or list(q.queue) == [[1], [4, 4], [3]] or list(q.queue) == [[4, 4], 2, [3]]
    assert q.dropped == 1
    assert q.accepted == 4
    assert q.accepted_lengths == 5
    dropped, accepted, accepted_lengths = q.reset_stats()
    assert dropped == 1
    assert accepted == 4
    assert accepted_lengths == 5


def test_queue_get():
    q = Q(maxsize=3)
    q.put(1)
    q.put(2)
    assert list(q.get()) == [1, 2]
    with pytest.raises(Empty):
        q.get(block=False)
