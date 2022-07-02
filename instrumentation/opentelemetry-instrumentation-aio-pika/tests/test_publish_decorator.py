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
import asyncio
from typing import Type
from unittest import TestCase, mock

from aio_pika import Exchange, RobustExchange

from opentelemetry.instrumentation.aio_pika.publish_decorator import (
    PublishDecorator,
)
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import SpanKind, get_tracer

from .consts import (
    CHANNEL,
    CONNECTION,
    CORRELATION_ID,
    EXCHANGE_NAME,
    MESSAGE,
    MESSAGE_ID,
    MESSAGING_SYSTEM,
    ROUTING_KEY,
    SERVER_HOST,
    SERVER_PORT,
)


class TestInstrumentedExchange(TestCase):
    EXPECTED_ATTRIBUTES = {
        SpanAttributes.MESSAGING_SYSTEM: MESSAGING_SYSTEM,
        SpanAttributes.MESSAGING_DESTINATION: f"{EXCHANGE_NAME},{ROUTING_KEY}",
        SpanAttributes.NET_PEER_NAME: SERVER_HOST,
        SpanAttributes.NET_PEER_PORT: SERVER_PORT,
        SpanAttributes.MESSAGING_MESSAGE_ID: MESSAGE_ID,
        SpanAttributes.MESSAGING_CONVERSATION_ID: CORRELATION_ID,
        SpanAttributes.MESSAGING_TEMP_DESTINATION: True,
    }

    def setUp(self):
        self.tracer = get_tracer(__name__)
        self.loop = asyncio.new_event_loop()
        asyncio.set_event_loop(self.loop)

    def test_get_publish_span(self):
        exchange = Exchange(CONNECTION, CHANNEL, EXCHANGE_NAME)
        tracer = mock.MagicMock()
        PublishDecorator(tracer, exchange)._get_publish_span(
            MESSAGE, ROUTING_KEY
        )
        tracer.start_span.assert_called_once_with(
            f"{EXCHANGE_NAME},{ROUTING_KEY} send",
            kind=SpanKind.PRODUCER,
            attributes=self.EXPECTED_ATTRIBUTES,
        )

    def _test_publish(self, exchange_type: Type[Exchange]):
        exchange = exchange_type(CONNECTION, CHANNEL, EXCHANGE_NAME)
        with mock.patch.object(
            PublishDecorator, "_get_publish_span"
        ) as mock_get_publish_span:
            with mock.patch.object(
                Exchange, "publish", return_value=asyncio.sleep(0)
            ) as mock_publish:
                decorated_publish = PublishDecorator(
                    self.tracer, exchange
                ).decorate(mock_publish)
                self.loop.run_until_complete(
                    decorated_publish(MESSAGE, ROUTING_KEY)
                )
        mock_publish.assert_called_once()
        mock_get_publish_span.assert_called_once()

    def test_publish(self):
        self._test_publish(Exchange)

    def test_robust_publish(self):
        self._test_publish(RobustExchange)
