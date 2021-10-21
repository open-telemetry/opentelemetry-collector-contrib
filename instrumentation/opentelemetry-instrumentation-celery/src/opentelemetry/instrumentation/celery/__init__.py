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
"""
Instrument `celery`_ to trace Celery applications.

.. _celery: https://pypi.org/project/celery/

Usage
-----

* Start broker backend

.. code::

    docker run -p 5672:5672 rabbitmq


* Run instrumented task

.. code:: python

    from opentelemetry.instrumentation.celery import CeleryInstrumentor

    from celery import Celery
    from celery.signals import worker_process_init

    @worker_process_init.connect(weak=False)
    def init_celery_tracing(*args, **kwargs):
        CeleryInstrumentor().instrument()

    app = Celery("tasks", broker="amqp://localhost")

    @app.task
    def add(x, y):
        return x + y

    add.delay(42, 50)

Setting up tracing
------------------

When tracing a celery worker process, tracing and instrumention both must be initialized after the celery worker
process is initialized. This is required for any tracing components that might use threading to work correctly
such as the BatchSpanProcessor. Celery provides a signal called ``worker_process_init`` that can be used to
accomplish this as shown in the example above.

API
---
"""

import logging
from typing import Collection, Iterable

from celery import signals  # pylint: disable=no-name-in-module

from opentelemetry import trace
from opentelemetry.instrumentation.celery import utils
from opentelemetry.instrumentation.celery.package import _instruments
from opentelemetry.instrumentation.celery.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.propagate import extract, inject
from opentelemetry.propagators.textmap import Getter
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace.status import Status, StatusCode

logger = logging.getLogger(__name__)

# Task operations
_TASK_TAG_KEY = "celery.action"
_TASK_APPLY_ASYNC = "apply_async"
_TASK_RUN = "run"

_TASK_RETRY_REASON_KEY = "celery.retry.reason"
_TASK_REVOKED_REASON_KEY = "celery.revoked.reason"
_TASK_REVOKED_TERMINATED_SIGNAL_KEY = "celery.terminated.signal"
_TASK_NAME_KEY = "celery.task_name"


class CeleryGetter(Getter):
    def get(self, carrier, key):
        value = getattr(carrier, key, None)
        if value is None:
            return None
        if isinstance(value, str) or not isinstance(value, Iterable):
            value = (value,)
        return value

    def keys(self, carrier):
        return []


celery_getter = CeleryGetter()


class CeleryInstrumentor(BaseInstrumentor):
    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        tracer_provider = kwargs.get("tracer_provider")

        # pylint: disable=attribute-defined-outside-init
        self._tracer = trace.get_tracer(__name__, __version__, tracer_provider)

        signals.task_prerun.connect(self._trace_prerun, weak=False)
        signals.task_postrun.connect(self._trace_postrun, weak=False)
        signals.before_task_publish.connect(
            self._trace_before_publish, weak=False
        )
        signals.after_task_publish.connect(
            self._trace_after_publish, weak=False
        )
        signals.task_failure.connect(self._trace_failure, weak=False)
        signals.task_retry.connect(self._trace_retry, weak=False)

    def _uninstrument(self, **kwargs):
        signals.task_prerun.disconnect(self._trace_prerun)
        signals.task_postrun.disconnect(self._trace_postrun)
        signals.before_task_publish.disconnect(self._trace_before_publish)
        signals.after_task_publish.disconnect(self._trace_after_publish)
        signals.task_failure.disconnect(self._trace_failure)
        signals.task_retry.disconnect(self._trace_retry)

    def _trace_prerun(self, *args, **kwargs):
        task = utils.retrieve_task(kwargs)
        task_id = utils.retrieve_task_id(kwargs)

        if task is None or task_id is None:
            return

        request = task.request
        tracectx = extract(request, getter=celery_getter) or None

        logger.debug("prerun signal start task_id=%s", task_id)

        operation_name = f"{_TASK_RUN}/{task.name}"
        span = self._tracer.start_span(
            operation_name, context=tracectx, kind=trace.SpanKind.CONSUMER
        )

        activation = trace.use_span(span, end_on_exit=True)
        activation.__enter__()  # pylint: disable=E1101
        utils.attach_span(task, task_id, (span, activation))

    @staticmethod
    def _trace_postrun(*args, **kwargs):
        task = utils.retrieve_task(kwargs)
        task_id = utils.retrieve_task_id(kwargs)

        if task is None or task_id is None:
            return

        logger.debug("postrun signal task_id=%s", task_id)

        # retrieve and finish the Span
        span, activation = utils.retrieve_span(task, task_id)
        if span is None:
            logger.warning("no existing span found for task_id=%s", task_id)
            return

        # request context tags
        if span.is_recording():
            span.set_attribute(_TASK_TAG_KEY, _TASK_RUN)
            utils.set_attributes_from_context(span, kwargs)
            utils.set_attributes_from_context(span, task.request)
            span.set_attribute(_TASK_NAME_KEY, task.name)

        activation.__exit__(None, None, None)
        utils.detach_span(task, task_id)

    def _trace_before_publish(self, *args, **kwargs):
        task = utils.retrieve_task_from_sender(kwargs)
        task_id = utils.retrieve_task_id_from_message(kwargs)

        if task is None or task_id is None:
            return

        operation_name = f"{_TASK_APPLY_ASYNC}/{task.name}"
        span = self._tracer.start_span(
            operation_name, kind=trace.SpanKind.PRODUCER
        )

        # apply some attributes here because most of the data is not available
        if span.is_recording():
            span.set_attribute(_TASK_TAG_KEY, _TASK_APPLY_ASYNC)
            span.set_attribute(SpanAttributes.MESSAGING_MESSAGE_ID, task_id)
            span.set_attribute(_TASK_NAME_KEY, task.name)
            utils.set_attributes_from_context(span, kwargs)

        activation = trace.use_span(span, end_on_exit=True)
        activation.__enter__()  # pylint: disable=E1101

        utils.attach_span(task, task_id, (span, activation), is_publish=True)

        headers = kwargs.get("headers")
        if headers:
            inject(headers)

    @staticmethod
    def _trace_after_publish(*args, **kwargs):
        task = utils.retrieve_task_from_sender(kwargs)
        task_id = utils.retrieve_task_id_from_message(kwargs)

        if task is None or task_id is None:
            return

        # retrieve and finish the Span
        _, activation = utils.retrieve_span(task, task_id, is_publish=True)
        if activation is None:
            logger.warning("no existing span found for task_id=%s", task_id)
            return

        activation.__exit__(None, None, None)  # pylint: disable=E1101
        utils.detach_span(task, task_id, is_publish=True)

    @staticmethod
    def _trace_failure(*args, **kwargs):
        task = utils.retrieve_task_from_sender(kwargs)
        task_id = utils.retrieve_task_id(kwargs)

        if task is None or task_id is None:
            return

        # retrieve and pass exception info to activation
        span, _ = utils.retrieve_span(task, task_id)
        if span is None or not span.is_recording():
            return

        status_kwargs = {"status_code": StatusCode.ERROR}

        ex = kwargs.get("einfo")

        if (
            hasattr(task, "throws")
            and ex is not None
            and isinstance(ex.exception, task.throws)
        ):
            return

        if ex is not None:
            status_kwargs["description"] = str(ex)
        span.set_status(Status(**status_kwargs))

    @staticmethod
    def _trace_retry(*args, **kwargs):
        task = utils.retrieve_task_from_sender(kwargs)
        task_id = utils.retrieve_task_id_from_request(kwargs)
        reason = utils.retrieve_reason(kwargs)

        if task is None or task_id is None or reason is None:
            return

        span, _ = utils.retrieve_span(task, task_id)
        if span is None or not span.is_recording():
            return

        # Add retry reason metadata to span
        # Use `str(reason)` instead of `reason.message` in case we get
        # something that isn't an `Exception`
        span.set_attribute(_TASK_RETRY_REASON_KEY, str(reason))
