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
from unittest import TestCase, mock

from opentelemetry.instrumentation.pika import utils
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import Span, Tracer


class TestUtils(TestCase):
    @staticmethod
    @mock.patch("opentelemetry.context.get_value")
    @mock.patch("opentelemetry.instrumentation.pika.utils._generate_span_name")
    @mock.patch("opentelemetry.instrumentation.pika.utils._enrich_span")
    @mock.patch("opentelemetry.propagate.extract")
    def test_get_span(
        extract: mock.MagicMock,
        enrich_span: mock.MagicMock,
        generate_span_name: mock.MagicMock,
        get_value: mock.MagicMock,
    ) -> None:
        tracer = mock.MagicMock(spec=Tracer)
        channel = mock.MagicMock()
        properties = mock.MagicMock()
        task_name = "test.test"
        get_value.return_value = None
        _ = utils._get_span(tracer, channel, properties, task_name)
        extract.assert_called_once()
        generate_span_name.assert_called_once()
        tracer.start_span.assert_called_once_with(
            context=extract.return_value, name=generate_span_name.return_value
        )
        enrich_span.assert_called_once()

    @mock.patch("opentelemetry.context.get_value")
    @mock.patch("opentelemetry.instrumentation.pika.utils._generate_span_name")
    @mock.patch("opentelemetry.instrumentation.pika.utils._enrich_span")
    @mock.patch("opentelemetry.propagate.extract")
    def test_get_span_suppressed(
        self,
        extract: mock.MagicMock,
        enrich_span: mock.MagicMock,
        generate_span_name: mock.MagicMock,
        get_value: mock.MagicMock,
    ) -> None:
        tracer = mock.MagicMock(spec=Tracer)
        channel = mock.MagicMock()
        properties = mock.MagicMock()
        task_name = "test.test"
        get_value.return_value = True
        span = utils._get_span(tracer, channel, properties, task_name)
        self.assertEqual(span, None)
        extract.assert_called_once()
        generate_span_name.assert_not_called()

    def test_generate_span_name_no_operation(self) -> None:
        task_name = "test.test"
        operation = None
        span_name = utils._generate_span_name(task_name, operation)
        self.assertEqual(span_name, f"{task_name} send")

    def test_generate_span_name_with_operation(self) -> None:
        task_name = "test.test"
        operation = mock.MagicMock()
        operation.value = "process"
        span_name = utils._generate_span_name(task_name, operation)
        self.assertEqual(span_name, f"{task_name} {operation.value}")

    @staticmethod
    def test_enrich_span_basic_values() -> None:
        channel = mock.MagicMock()
        properties = mock.MagicMock()
        task_destination = "test.test"
        span = mock.MagicMock(spec=Span)
        utils._enrich_span(span, channel, properties, task_destination)
        span.set_attribute.assert_has_calls(
            any_order=True,
            calls=[
                mock.call(SpanAttributes.MESSAGING_SYSTEM, "rabbitmq"),
                mock.call(SpanAttributes.MESSAGING_TEMP_DESTINATION, True),
                mock.call(
                    SpanAttributes.MESSAGING_DESTINATION, task_destination
                ),
                mock.call(
                    SpanAttributes.MESSAGING_MESSAGE_ID, properties.message_id
                ),
                mock.call(
                    SpanAttributes.MESSAGING_CONVERSATION_ID,
                    properties.correlation_id,
                ),
                mock.call(
                    SpanAttributes.NET_PEER_NAME,
                    channel.connection.params.host,
                ),
                mock.call(
                    SpanAttributes.NET_PEER_PORT,
                    channel.connection.params.port,
                ),
            ],
        )

    @staticmethod
    def test_enrich_span_with_operation() -> None:
        channel = mock.MagicMock()
        properties = mock.MagicMock()
        task_destination = "test.test"
        operation = mock.MagicMock()
        span = mock.MagicMock(spec=Span)
        utils._enrich_span(
            span, channel, properties, task_destination, operation
        )
        span.set_attribute.assert_has_calls(
            any_order=True,
            calls=[
                mock.call(SpanAttributes.MESSAGING_OPERATION, operation.value)
            ],
        )

    @staticmethod
    def test_enrich_span_without_operation() -> None:
        channel = mock.MagicMock()
        properties = mock.MagicMock()
        task_destination = "test.test"
        span = mock.MagicMock(spec=Span)
        utils._enrich_span(span, channel, properties, task_destination)
        span.set_attribute.assert_has_calls(
            any_order=True,
            calls=[mock.call(SpanAttributes.MESSAGING_TEMP_DESTINATION, True)],
        )

    @staticmethod
    def test_enrich_span_unique_connection() -> None:
        channel = mock.MagicMock()
        properties = mock.MagicMock()
        task_destination = "test.test"
        span = mock.MagicMock(spec=Span)
        # We do this to create the behaviour of hasattr(channel.connection, "params") == False
        del channel.connection.params
        utils._enrich_span(span, channel, properties, task_destination)
        span.set_attribute.assert_has_calls(
            any_order=True,
            calls=[
                mock.call(
                    SpanAttributes.NET_PEER_NAME,
                    channel.connection._impl.params.host,
                ),
                mock.call(
                    SpanAttributes.NET_PEER_PORT,
                    channel.connection._impl.params.port,
                ),
            ],
        )
