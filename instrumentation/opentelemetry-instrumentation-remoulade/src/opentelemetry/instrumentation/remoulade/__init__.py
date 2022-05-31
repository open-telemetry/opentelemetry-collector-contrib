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
Usage
-----

* Start broker backend

::

    docker run -p 5672:5672 rabbitmq

* Run instrumented actor

.. code-block:: python

    from remoulade.brokers.rabbitmq import RabbitmqBroker
    import remoulade

    RemouladeInstrumentor().instrument()

    broker = RabbitmqBroker()
    remoulade.set_broker(broker)

    @remoulade.actor
    def multiply(x, y):
        return x * y

    broker.declare_actor(count_words)

    multiply.send(43, 51)

"""
from typing import Collection

from remoulade import Middleware, broker

from opentelemetry import trace
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.remoulade import utils
from opentelemetry.instrumentation.remoulade.package import _instruments
from opentelemetry.instrumentation.remoulade.version import __version__
from opentelemetry.propagate import extract, inject
from opentelemetry.semconv.trace import SpanAttributes

_REMOULADE_MESSAGE_TAG_KEY = "remoulade.action"
_REMOULADE_MESSAGE_SEND = "send"
_REMOULADE_MESSAGE_RUN = "run"

_REMOULADE_MESSAGE_NAME_KEY = "remoulade.actor_name"

_REMOULADE_MESSAGE_RETRY_COUNT_KEY = "remoulade.retry_count"


class _InstrumentationMiddleware(Middleware):
    def __init__(self, _tracer):
        self._tracer = _tracer
        self._span_registry = {}

    def before_process_message(self, _broker, message):
        if "trace_ctx" not in message.options:
            return

        trace_ctx = extract(message.options["trace_ctx"])
        retry_count = message.options.get("retries", 0)
        operation_name = utils.get_operation_name(
            "before_process_message", retry_count
        )
        span_attributes = {_REMOULADE_MESSAGE_RETRY_COUNT_KEY: retry_count}

        span = self._tracer.start_span(
            operation_name,
            kind=trace.SpanKind.CONSUMER,
            context=trace_ctx,
            attributes=span_attributes,
        )

        activation = trace.use_span(span, end_on_exit=True)
        activation.__enter__()  # pylint: disable=E1101

        utils.attach_span(
            self._span_registry, message.message_id, (span, activation)
        )

    def after_process_message(
        self, _broker, message, *, result=None, exception=None
    ):
        span, activation = utils.retrieve_span(
            self._span_registry, message.message_id
        )

        if span is None:
            # no existing span found for message_id
            return

        if span.is_recording():
            span.set_attributes(
                {
                    _REMOULADE_MESSAGE_TAG_KEY: _REMOULADE_MESSAGE_RUN,
                    _REMOULADE_MESSAGE_NAME_KEY: message.actor_name,
                    SpanAttributes.MESSAGING_MESSAGE_ID: message.message_id,
                }
            )

        activation.__exit__(None, None, None)
        utils.detach_span(self._span_registry, message.message_id)

    def before_enqueue(self, _broker, message, delay):
        retry_count = message.options.get("retries", 0)
        operation_name = utils.get_operation_name(
            "before_enqueue", retry_count
        )
        span_attributes = {_REMOULADE_MESSAGE_RETRY_COUNT_KEY: retry_count}

        span = self._tracer.start_span(
            operation_name,
            kind=trace.SpanKind.PRODUCER,
            attributes=span_attributes,
        )

        if span.is_recording():
            span.set_attributes(
                {
                    _REMOULADE_MESSAGE_TAG_KEY: _REMOULADE_MESSAGE_SEND,
                    _REMOULADE_MESSAGE_NAME_KEY: message.actor_name,
                    SpanAttributes.MESSAGING_MESSAGE_ID: message.message_id,
                }
            )

        activation = trace.use_span(span, end_on_exit=True)
        activation.__enter__()  # pylint: disable=E1101

        utils.attach_span(
            self._span_registry,
            message.message_id,
            (span, activation),
            is_publish=True,
        )

        if "trace_ctx" not in message.options:
            message.options["trace_ctx"] = {}
        inject(message.options["trace_ctx"])

    def after_enqueue(self, _broker, message, delay, exception=None):
        _, activation = utils.retrieve_span(
            self._span_registry, message.message_id, is_publish=True
        )

        if activation is None:
            # no existing span found for message_id
            return

        activation.__exit__(None, None, None)
        utils.detach_span(
            self._span_registry, message.message_id, is_publish=True
        )


class RemouladeInstrumentor(BaseInstrumentor):
    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        tracer_provider = kwargs.get("tracer_provider")

        # pylint: disable=attribute-defined-outside-init
        self._tracer = trace.get_tracer(__name__, __version__, tracer_provider)
        instrumentation_middleware = _InstrumentationMiddleware(self._tracer)

        broker.add_extra_default_middleware(instrumentation_middleware)

    def _uninstrument(self, **kwargs):
        broker.remove_extra_default_middleware(_InstrumentationMiddleware)
