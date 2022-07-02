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
from typing import Any, Callable, Collection

import wrapt
from aio_pika import Exchange, Queue
from aio_pika.abc import AbstractIncomingMessage

from opentelemetry import trace
from opentelemetry.instrumentation.aio_pika.callback_decorator import (
    CallbackDecorator,
)
from opentelemetry.instrumentation.aio_pika.package import _instruments
from opentelemetry.instrumentation.aio_pika.publish_decorator import (
    PublishDecorator,
)
from opentelemetry.instrumentation.aio_pika.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.trace import Tracer

_INSTRUMENTATION_MODULE_NAME = "opentelemetry.instrumentation.aio_pika"


class AioPikaInstrumentor(BaseInstrumentor):
    @staticmethod
    def _instrument_queue(tracer: Tracer):
        async def wrapper(wrapped, instance, args, kwargs):
            async def consume(
                callback: Callable[[AbstractIncomingMessage], Any],
                *fargs,
                **fkwargs
            ):
                decorated_callback = CallbackDecorator(
                    tracer, instance
                ).decorate(callback)
                return await wrapped(decorated_callback, *fargs, **fkwargs)

            return await consume(*args, **kwargs)

        wrapt.wrap_function_wrapper(Queue, "consume", wrapper)

    @staticmethod
    def _instrument_exchange(tracer: Tracer):
        async def wrapper(wrapped, instance, args, kwargs):
            decorated_publish = PublishDecorator(tracer, instance).decorate(
                wrapped
            )
            return await decorated_publish(*args, **kwargs)

        wrapt.wrap_function_wrapper(Exchange, "publish", wrapper)

    def _instrument(self, **kwargs):
        tracer_provider = kwargs.get("tracer_provider", None)
        tracer = trace.get_tracer(
            _INSTRUMENTATION_MODULE_NAME, __version__, tracer_provider
        )
        self._instrument_queue(tracer)
        self._instrument_exchange(tracer)

    @staticmethod
    def _uninstrument_queue():
        unwrap(Queue, "consume")

    @staticmethod
    def _uninstrument_exchange():
        unwrap(Exchange, "publish")

    def _uninstrument(self, **kwargs):
        self._uninstrument_queue()
        self._uninstrument_exchange()

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments
