# stdlib
import itertools
import random
import time

from .. import api
from .. import _worker
from ..internal.logger import get_logger
from ..sampler import BasePrioritySampler
from ..settings import config
from ..vendor import monotonic
from ddtrace.vendor.six.moves.queue import Queue, Full, Empty

log = get_logger(__name__)


MAX_TRACES = 1000

DEFAULT_TIMEOUT = 5
LOG_ERR_INTERVAL = 60


class AgentWriter(_worker.PeriodicWorkerThread):

    QUEUE_PROCESSING_INTERVAL = 1

    def __init__(
        self,
        hostname="localhost",
        port=8126,
        uds_path=None,
        https=False,
        shutdown_timeout=DEFAULT_TIMEOUT,
        filters=None,
        sampler=None,
        priority_sampler=None,
        dogstatsd=None,
    ):
        super(AgentWriter, self).__init__(
            interval=self.QUEUE_PROCESSING_INTERVAL, exit_timeout=shutdown_timeout, name=self.__class__.__name__
        )
        self._trace_queue = Q(maxsize=MAX_TRACES)
        self._filters = filters
        self._sampler = sampler
        self._priority_sampler = priority_sampler
        self._last_error_ts = 0
        self.dogstatsd = dogstatsd
        self.api = api.API(
            hostname, port, uds_path=uds_path, https=https, priority_sampling=priority_sampler is not None
        )
        if hasattr(time, "thread_time"):
            self._last_thread_time = time.thread_time()
        self.start()

    def recreate(self):
        """ Create a new instance of :class:`AgentWriter` using the same settings from this instance

        :rtype: :class:`AgentWriter`
        :returns: A new :class:`AgentWriter` instance
        """
        writer = self.__class__(
            hostname=self.api.hostname,
            port=self.api.port,
            uds_path=self.api.uds_path,
            https=self.api.https,
            shutdown_timeout=self.exit_timeout,
            filters=self._filters,
            priority_sampler=self._priority_sampler,
            dogstatsd=self.dogstatsd,
        )
        return writer

    @property
    def _send_stats(self):
        """Determine if we're sending stats or not."""
        return bool(config.health_metrics_enabled and self.dogstatsd)

    def write(self, spans=None, services=None):
        if spans:
            self._trace_queue.put(spans)

    def flush_queue(self):
        try:
            traces = self._trace_queue.get(block=False)
        except Empty:
            return

        if self._send_stats:
            traces_queue_length = len(traces)
            traces_queue_spans = sum(map(len, traces))

        # Before sending the traces, make them go through the
        # filters
        try:
            traces = self._apply_filters(traces)
        except Exception:
            log.error("error while filtering traces", exc_info=True)
            return

        if self._send_stats:
            traces_filtered = len(traces) - traces_queue_length

        # If we have data, let's try to send it.
        traces_responses = self.api.send_traces(traces)
        for response in traces_responses:
            if isinstance(response, Exception) or response.status >= 400:
                self._log_error_status(response)
            elif self._priority_sampler or isinstance(self._sampler, BasePrioritySampler):
                result_traces_json = response.get_json()
                if result_traces_json and "rate_by_service" in result_traces_json:
                    if self._priority_sampler:
                        self._priority_sampler.update_rate_by_service_sample_rates(
                            result_traces_json["rate_by_service"],
                        )
                    if isinstance(self._sampler, BasePrioritySampler):
                        self._sampler.update_rate_by_service_sample_rates(result_traces_json["rate_by_service"],)

        # Dump statistics
        # NOTE: Do not use the buffering of dogstatsd as it's not thread-safe
        # https://github.com/DataDog/datadogpy/issues/439
        if self._send_stats:
            # Statistics about the queue length, size and number of spans
            self.dogstatsd.increment("datadog.tracer.flushes")
            self._histogram_with_total("datadog.tracer.flush.traces", traces_queue_length)
            self._histogram_with_total("datadog.tracer.flush.spans", traces_queue_spans)

            # Statistics about the filtering
            self._histogram_with_total("datadog.tracer.flush.traces_filtered", traces_filtered)

            # Statistics about API
            self._histogram_with_total("datadog.tracer.api.requests", len(traces_responses))

            self._histogram_with_total(
                "datadog.tracer.api.errors", len(list(t for t in traces_responses if isinstance(t, Exception)))
            )
            for status, grouped_responses in itertools.groupby(
                sorted((t for t in traces_responses if not isinstance(t, Exception)), key=lambda r: r.status),
                key=lambda r: r.status,
            ):
                self._histogram_with_total(
                    "datadog.tracer.api.responses", len(list(grouped_responses)), tags=["status:%d" % status]
                )

            # Statistics about the writer thread
            if hasattr(time, "thread_time"):
                new_thread_time = time.thread_time()
                diff = new_thread_time - self._last_thread_time
                self._last_thread_time = new_thread_time
                self.dogstatsd.histogram("datadog.tracer.writer.cpu_time", diff)

    def _histogram_with_total(self, name, value, tags=None):
        """Helper to add metric as a histogram and with a `.total` counter"""
        self.dogstatsd.histogram(name, value, tags=tags)
        self.dogstatsd.increment("%s.total" % (name,), value, tags=tags)

    def run_periodic(self):
        if self._send_stats:
            self.dogstatsd.gauge("datadog.tracer.heartbeat", 1)

        try:
            self.flush_queue()
        finally:
            if not self._send_stats:
                return

            # Statistics about the rate at which spans are inserted in the queue
            dropped, enqueued, enqueued_lengths = self._trace_queue.reset_stats()
            self.dogstatsd.gauge("datadog.tracer.queue.max_length", self._trace_queue.maxsize)
            self.dogstatsd.increment("datadog.tracer.queue.dropped.traces", dropped)
            self.dogstatsd.increment("datadog.tracer.queue.enqueued.traces", enqueued)
            self.dogstatsd.increment("datadog.tracer.queue.enqueued.spans", enqueued_lengths)

    def on_shutdown(self):
        try:
            self.run_periodic()
        finally:
            if not self._send_stats:
                return

            self.dogstatsd.increment("datadog.tracer.shutdown")

    def _log_error_status(self, response):
        log_level = log.debug
        now = monotonic.monotonic()
        if now > self._last_error_ts + LOG_ERR_INTERVAL:
            log_level = log.error
            self._last_error_ts = now
        prefix = "Failed to send traces to Datadog Agent at %s: "
        if isinstance(response, api.Response):
            log_level(
                prefix + "HTTP error status %s, reason %s, message %s",
                self.api,
                response.status,
                response.reason,
                response.msg,
            )
        else:
            log_level(
                prefix + "%s", self.api, response,
            )

    def _apply_filters(self, traces):
        """
        Here we make each trace go through the filters configured in the
        tracer. There is no need for a lock since the traces are owned by the
        AgentWriter at that point.
        """
        if self._filters is not None:
            filtered_traces = []
            for trace in traces:
                for filtr in self._filters:
                    trace = filtr.process_trace(trace)
                    if trace is None:
                        break
                if trace is not None:
                    filtered_traces.append(trace)
            return filtered_traces
        return traces


class Q(Queue):
    """
    Q is a threadsafe queue that let's you pop everything at once and
    will randomly overwrite elements when it's over the max size.

    This queue also exposes some statistics about its length, the number of items dropped, etc.
    """

    def __init__(self, maxsize=0):
        # Cannot use super() here because Queue in Python2 is old style class
        Queue.__init__(self, maxsize)
        # Number of item dropped (queue full)
        self.dropped = 0
        # Number of items accepted
        self.accepted = 0
        # Cumulative length of accepted items
        self.accepted_lengths = 0

    def put(self, item):
        try:
            # Cannot use super() here because Queue in Python2 is old style class
            Queue.put(self, item, block=False)
        except Full:
            # If the queue is full, replace a random item. We need to make sure
            # the queue is not emptied was emptied in the meantime, so we lock
            # check qsize value.
            with self.mutex:
                qsize = self._qsize()
                if qsize != 0:
                    idx = random.randrange(0, qsize)
                    self.queue[idx] = item
                    log.warning("Writer queue is full has more than %d traces, some traces will be lost", self.maxsize)
                    self.dropped += 1
                    self._update_stats(item)
                    return
            # The queue has been emptied, simply retry putting item
            return self.put(item)
        else:
            with self.mutex:
                self._update_stats(item)

    def _update_stats(self, item):
        # self.mutex needs to be locked to make sure we don't lose data when resetting
        self.accepted += 1
        if hasattr(item, "__len__"):
            item_length = len(item)
        else:
            item_length = 1
        self.accepted_lengths += item_length

    def reset_stats(self):
        """Reset the stats to 0.

        :return: The current value of dropped, accepted and accepted_lengths.
        """
        with self.mutex:
            dropped, accepted, accepted_lengths = (self.dropped, self.accepted, self.accepted_lengths)
            self.dropped, self.accepted, self.accepted_lengths = 0, 0, 0
        return dropped, accepted, accepted_lengths

    def _get(self):
        things = self.queue
        self._init(self.maxsize)
        return things
