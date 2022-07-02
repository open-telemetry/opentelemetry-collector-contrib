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
from typing import Callable, Optional

import aiormq
from aio_pika import Exchange
from aio_pika.abc import AbstractMessage

from opentelemetry import propagate, trace
from opentelemetry.instrumentation.aio_pika.span_builder import SpanBuilder
from opentelemetry.trace import Span, Tracer


class PublishDecorator:
    def __init__(self, tracer: Tracer, exchange: Exchange):
        self._tracer = tracer
        self._exchange = exchange

    def _get_publish_span(
        self, message: AbstractMessage, routing_key: str
    ) -> Optional[Span]:
        builder = SpanBuilder(self._tracer)
        builder.set_as_producer()
        builder.set_destination(f"{self._exchange.name},{routing_key}")
        builder.set_channel(self._exchange.channel)
        builder.set_message(message)
        return builder.build()

    def decorate(self, publish: Callable) -> Callable:
        async def decorated_publish(
            message: AbstractMessage, routing_key: str, **kwargs
        ) -> Optional[aiormq.abc.ConfirmationFrameType]:
            span = self._get_publish_span(message, routing_key)
            if not span:
                return await publish(message, routing_key, **kwargs)
            with trace.use_span(span, end_on_exit=True):
                if span.is_recording():
                    propagate.inject(message.properties.headers)
                return_value = await publish(message, routing_key, **kwargs)
            return return_value

        return decorated_publish
