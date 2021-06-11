# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import collections
import logging
import threading
import typing

from opentelemetry.context import Context, attach, detach, set_value
from opentelemetry.instrumentation.utils import _SUPPRESS_INSTRUMENTATION_KEY
from opentelemetry.sdk.trace import Span, SpanProcessor
from opentelemetry.sdk.trace.export import SpanExporter
from opentelemetry.trace import INVALID_TRACE_ID
from opentelemetry.util._time import _time_ns

logger = logging.getLogger(__name__)


class DatadogExportSpanProcessor(SpanProcessor):
    """Datadog exporter span processor

    DatadogExportSpanProcessor is an implementation of `SpanProcessor` that
    batches all opened spans into a list per trace. When all spans for a trace
    are ended, the trace is queues up for export. This is required for exporting
    to the Datadog Agent which expects to received list of spans for each trace.
    """

    _FLUSH_TOKEN = INVALID_TRACE_ID

    def __init__(
        self,
        span_exporter: SpanExporter,
        schedule_delay_millis: float = 5000,
        max_trace_size: int = 4096,
    ):
        if max_trace_size <= 0:
            raise ValueError("max_queue_size must be a positive integer.")

        if schedule_delay_millis <= 0:
            raise ValueError("schedule_delay_millis must be positive.")

        self.span_exporter = span_exporter

        # queue trace_ids for traces with recently ended spans for worker thread to check
        # for exporting
        self.check_traces_queue = (
            collections.deque()
        )  # type: typing.Deque[int]

        self.traces_lock = threading.Lock()
        # dictionary of trace_ids to a list of spans where the first span is the
        # first opened span for the trace
        self.traces = collections.defaultdict(list)
        # counter to keep track of the number of spans and ended spans for a
        # trace_id
        self.traces_spans_count = collections.Counter()
        self.traces_spans_ended_count = collections.Counter()

        self.worker_thread = threading.Thread(target=self.worker, daemon=True)

        # threading conditions used for flushing and shutdown
        self.condition = threading.Condition(threading.Lock())
        self.flush_condition = threading.Condition(threading.Lock())

        # flag to indicate that there is a flush operation on progress
        self._flushing = False

        self.max_trace_size = max_trace_size
        self._spans_dropped = False
        self.schedule_delay_millis = schedule_delay_millis
        self.done = False
        self.worker_thread.start()

    def on_start(
        self, span: Span, parent_context: typing.Optional[Context] = None
    ) -> None:
        ctx = span.get_span_context()
        trace_id = ctx.trace_id

        with self.traces_lock:
            # check upper bound on number of spans for trace before adding new
            # span
            if self.traces_spans_count[trace_id] == self.max_trace_size:
                logger.warning("Max spans for trace, spans will be dropped.")
                self._spans_dropped = True
                return

            # add span to end of list for a trace and update the counter
            self.traces[trace_id].append(span)
            self.traces_spans_count[trace_id] += 1

    def on_end(self, span: Span) -> None:
        if self.done:
            logger.warning("Already shutdown, dropping span.")
            return

        ctx = span.get_span_context()
        trace_id = ctx.trace_id

        with self.traces_lock:
            self.traces_spans_ended_count[trace_id] += 1
            if self.is_trace_exportable(trace_id):
                self.check_traces_queue.appendleft(trace_id)

    def worker(self):
        timeout = self.schedule_delay_millis / 1e3
        while not self.done:
            if not self._flushing:
                with self.condition:
                    self.condition.wait(timeout)
                    if not self.check_traces_queue:
                        # spurious notification, let's wait again, reset timeout
                        timeout = self.schedule_delay_millis / 1e3
                        continue
                    if self.done:
                        # missing spans will be sent when calling flush
                        break

            # substract the duration of this export call to the next timeout
            start = _time_ns()
            self.export()
            end = _time_ns()
            duration = (end - start) / 1e9
            timeout = self.schedule_delay_millis / 1e3 - duration

        # be sure that all spans are sent
        self._drain_queue()

    def is_trace_exportable(self, trace_id):
        return (
            self.traces_spans_count[trace_id]
            - self.traces_spans_ended_count[trace_id]
            <= 0
        )

    def export(self) -> None:
        """Exports traces with finished spans."""
        notify_flush = False
        export_trace_ids = []

        while self.check_traces_queue:
            trace_id = self.check_traces_queue.pop()
            if trace_id is self._FLUSH_TOKEN:
                notify_flush = True
            else:
                with self.traces_lock:
                    # check whether trace is exportable again in case that new
                    # spans were started since we last concluded trace was
                    # exportable
                    if self.is_trace_exportable(trace_id):
                        export_trace_ids.append(trace_id)
                        del self.traces_spans_count[trace_id]
                        del self.traces_spans_ended_count[trace_id]

        if len(export_trace_ids) > 0:
            token = attach(set_value(_SUPPRESS_INSTRUMENTATION_KEY, True))

            for trace_id in export_trace_ids:
                with self.traces_lock:
                    try:
                        # Ignore type b/c the Optional[None]+slicing is too "clever"
                        # for mypy
                        self.span_exporter.export(self.traces[trace_id])  # type: ignore
                    # pylint: disable=broad-except
                    except Exception:
                        logger.exception(
                            "Exception while exporting Span batch."
                        )
                    finally:
                        del self.traces[trace_id]

            detach(token)

        if notify_flush:
            with self.flush_condition:
                self.flush_condition.notify()

    def _drain_queue(self):
        """Export all elements until queue is empty.

        Can only be called from the worker thread context because it invokes
        `export` that is not thread safe.
        """
        while self.check_traces_queue:
            self.export()

    def force_flush(self, timeout_millis: int = 30000) -> bool:
        if self.done:
            logger.warning("Already shutdown, ignoring call to force_flush().")
            return True

        self._flushing = True
        self.check_traces_queue.appendleft(self._FLUSH_TOKEN)

        # wake up worker thread
        with self.condition:
            self.condition.notify_all()

        # wait for token to be processed
        with self.flush_condition:
            ret = self.flush_condition.wait(timeout_millis / 1e3)

        self._flushing = False

        if not ret:
            logger.warning("Timeout was exceeded in force_flush().")
        return ret

    def shutdown(self) -> None:
        # signal the worker thread to finish and then wait for it
        self.done = True
        with self.condition:
            self.condition.notify_all()
        self.worker_thread.join()
        self.span_exporter.shutdown()
