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

from pika.channel import Channel
from pika.spec import Basic, BasicProperties

from opentelemetry.instrumentation.pika import utils
from opentelemetry.semconv.trace import (
    MessagingOperationValues,
    SpanAttributes,
)
from opentelemetry.trace import Span, SpanKind, Tracer


class TestUtils(TestCase):
    @staticmethod
    @mock.patch("opentelemetry.context.get_value")
    @mock.patch("opentelemetry.instrumentation.pika.utils._generate_span_name")
    @mock.patch("opentelemetry.instrumentation.pika.utils._enrich_span")
    def test_get_span(
        enrich_span: mock.MagicMock,
        generate_span_name: mock.MagicMock,
        get_value: mock.MagicMock,
    ) -> None:
        tracer = mock.MagicMock(spec=Tracer)
        channel = mock.MagicMock()
        properties = mock.MagicMock()
        task_name = "test.test"
        destination = "myqueue"
        span_kind = mock.MagicMock(spec=SpanKind)
        get_value.return_value = None
        _ = utils._get_span(
            tracer, channel, properties, task_name, destination, span_kind
        )
        generate_span_name.assert_called_once()
        tracer.start_span.assert_called_once_with(
            name=generate_span_name.return_value, kind=span_kind
        )
        enrich_span.assert_called_once()

    @mock.patch("opentelemetry.context.get_value")
    @mock.patch("opentelemetry.instrumentation.pika.utils._generate_span_name")
    @mock.patch("opentelemetry.instrumentation.pika.utils._enrich_span")
    def test_get_span_suppressed(
        self,
        enrich_span: mock.MagicMock,
        generate_span_name: mock.MagicMock,
        get_value: mock.MagicMock,
    ) -> None:
        tracer = mock.MagicMock(spec=Tracer)
        channel = mock.MagicMock()
        properties = mock.MagicMock()
        task_name = "test.test"
        span_kind = mock.MagicMock(spec=SpanKind)
        get_value.return_value = True
        ctx = mock.MagicMock()
        span = utils._get_span(
            tracer, channel, properties, task_name, span_kind, ctx
        )
        self.assertEqual(span, None)
        generate_span_name.assert_not_called()
        enrich_span.assert_not_called()

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

    @mock.patch("opentelemetry.instrumentation.pika.utils._get_span")
    @mock.patch("opentelemetry.propagate.extract")
    @mock.patch("opentelemetry.trace.use_span")
    def test_decorate_callback(
        self,
        use_span: mock.MagicMock,
        extract: mock.MagicMock,
        get_span: mock.MagicMock,
    ) -> None:
        callback = mock.MagicMock()
        mock_task_name = "mock_task_name"
        tracer = mock.MagicMock()
        channel = mock.MagicMock(spec=Channel)
        method = mock.MagicMock(spec=Basic.Deliver)
        method.exchange = "test_exchange"
        properties = mock.MagicMock()
        mock_body = b"mock_body"
        decorated_callback = utils._decorate_callback(
            callback, tracer, mock_task_name
        )
        retval = decorated_callback(channel, method, properties, mock_body)
        extract.assert_called_once_with(
            properties.headers, getter=utils._pika_getter
        )
        get_span.assert_called_once_with(
            tracer,
            channel,
            properties,
            destination=method.exchange,
            span_kind=SpanKind.CONSUMER,
            task_name=mock_task_name,
            operation=MessagingOperationValues.RECEIVE,
        )
        use_span.assert_called_once_with(
            get_span.return_value, end_on_exit=True
        )
        callback.assert_called_once_with(
            channel, method, properties, mock_body
        )
        self.assertEqual(retval, callback.return_value)

    @mock.patch("opentelemetry.instrumentation.pika.utils._get_span")
    @mock.patch("opentelemetry.propagate.extract")
    @mock.patch("opentelemetry.trace.use_span")
    def test_decorate_callback_with_hook(
        self,
        use_span: mock.MagicMock,
        extract: mock.MagicMock,
        get_span: mock.MagicMock,
    ) -> None:
        callback = mock.MagicMock()
        mock_task_name = "mock_task_name"
        tracer = mock.MagicMock()
        channel = mock.MagicMock(spec=Channel)
        method = mock.MagicMock(spec=Basic.Deliver)
        method.exchange = "test_exchange"
        properties = mock.MagicMock()
        mock_body = b"mock_body"
        consume_hook = mock.MagicMock()

        decorated_callback = utils._decorate_callback(
            callback, tracer, mock_task_name, consume_hook
        )
        retval = decorated_callback(channel, method, properties, mock_body)
        extract.assert_called_once_with(
            properties.headers, getter=utils._pika_getter
        )
        get_span.assert_called_once_with(
            tracer,
            channel,
            properties,
            destination=method.exchange,
            span_kind=SpanKind.CONSUMER,
            task_name=mock_task_name,
            operation=MessagingOperationValues.RECEIVE,
        )
        use_span.assert_called_once_with(
            get_span.return_value, end_on_exit=True
        )
        consume_hook.assert_called_once_with(
            get_span.return_value, mock_body, properties
        )
        callback.assert_called_once_with(
            channel, method, properties, mock_body
        )
        self.assertEqual(retval, callback.return_value)

    @mock.patch("opentelemetry.instrumentation.pika.utils._get_span")
    @mock.patch("opentelemetry.propagate.inject")
    @mock.patch("opentelemetry.trace.use_span")
    def test_decorate_basic_publish(
        self,
        use_span: mock.MagicMock,
        inject: mock.MagicMock,
        get_span: mock.MagicMock,
    ) -> None:
        callback = mock.MagicMock()
        tracer = mock.MagicMock()
        channel = mock.MagicMock(spec=Channel)
        exchange_name = "test-exchange"
        routing_key = "test-routing-key"
        properties = mock.MagicMock()
        mock_body = b"mock_body"
        decorated_basic_publish = utils._decorate_basic_publish(
            callback, channel, tracer
        )
        retval = decorated_basic_publish(
            exchange_name, routing_key, mock_body, properties
        )
        get_span.assert_called_once_with(
            tracer,
            channel,
            properties,
            destination=exchange_name,
            span_kind=SpanKind.PRODUCER,
            task_name="(temporary)",
            operation=None,
        )
        use_span.assert_called_once_with(
            get_span.return_value, end_on_exit=True
        )
        get_span.return_value.is_recording.assert_called_once()
        inject.assert_called_once_with(properties.headers)
        callback.assert_called_once_with(
            exchange_name, routing_key, mock_body, properties, False
        )
        self.assertEqual(retval, callback.return_value)

    @mock.patch("opentelemetry.instrumentation.pika.utils._get_span")
    @mock.patch("opentelemetry.propagate.inject")
    @mock.patch("opentelemetry.trace.use_span")
    @mock.patch("pika.spec.BasicProperties.__new__")
    def test_decorate_basic_publish_no_properties(
        self,
        basic_properties: mock.MagicMock,
        use_span: mock.MagicMock,
        inject: mock.MagicMock,
        get_span: mock.MagicMock,
    ) -> None:
        callback = mock.MagicMock()
        tracer = mock.MagicMock()
        channel = mock.MagicMock(spec=Channel)
        method = mock.MagicMock(spec=Basic.Deliver)
        mock_body = b"mock_body"
        decorated_basic_publish = utils._decorate_basic_publish(
            callback, channel, tracer
        )
        retval = decorated_basic_publish(channel, method, body=mock_body)
        basic_properties.assert_called_once_with(BasicProperties, headers={})
        use_span.assert_called_once_with(
            get_span.return_value, end_on_exit=True
        )
        get_span.return_value.is_recording.assert_called_once()
        inject.assert_called_once_with(basic_properties.return_value.headers)
        self.assertEqual(retval, callback.return_value)

    @staticmethod
    @mock.patch("opentelemetry.instrumentation.pika.utils._get_span")
    def test_decorate_basic_publish_published_message_to_queue(
        get_span: mock.MagicMock,
    ) -> None:
        callback = mock.MagicMock()
        tracer = mock.MagicMock()
        channel = mock.MagicMock(spec=Channel)
        exchange_name = ""
        routing_key = "test-routing-key"
        properties = mock.MagicMock()
        mock_body = b"mock_body"

        decorated_basic_publish = utils._decorate_basic_publish(
            callback, channel, tracer
        )
        decorated_basic_publish(
            exchange_name, routing_key, mock_body, properties
        )

        get_span.assert_called_once_with(
            tracer,
            channel,
            properties,
            destination=routing_key,
            span_kind=SpanKind.PRODUCER,
            task_name="(temporary)",
            operation=None,
        )

    @mock.patch("opentelemetry.instrumentation.pika.utils._get_span")
    @mock.patch("opentelemetry.propagate.inject")
    @mock.patch("opentelemetry.trace.use_span")
    def test_decorate_basic_publish_with_hook(
        self,
        use_span: mock.MagicMock,
        inject: mock.MagicMock,
        get_span: mock.MagicMock,
    ) -> None:
        callback = mock.MagicMock()
        tracer = mock.MagicMock()
        channel = mock.MagicMock(spec=Channel)
        exchange_name = "test-exchange"
        routing_key = "test-routing-key"
        properties = mock.MagicMock()
        mock_body = b"mock_body"
        publish_hook = mock.MagicMock()

        decorated_basic_publish = utils._decorate_basic_publish(
            callback, channel, tracer, publish_hook
        )
        retval = decorated_basic_publish(
            exchange_name, routing_key, mock_body, properties
        )
        get_span.assert_called_once_with(
            tracer,
            channel,
            properties,
            destination=exchange_name,
            span_kind=SpanKind.PRODUCER,
            task_name="(temporary)",
            operation=None,
        )
        use_span.assert_called_once_with(
            get_span.return_value, end_on_exit=True
        )
        get_span.return_value.is_recording.assert_called_once()
        inject.assert_called_once_with(properties.headers)
        publish_hook.assert_called_once_with(
            get_span.return_value, mock_body, properties
        )
        callback.assert_called_once_with(
            exchange_name, routing_key, mock_body, properties, False
        )
        self.assertEqual(retval, callback.return_value)
