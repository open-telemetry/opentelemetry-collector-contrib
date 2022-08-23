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

# pylint: disable=no-name-in-module

from contextlib import contextmanager
from typing import Any, Dict
from unittest import TestCase, mock

import boto3
from botocore.awsrequest import AWSResponse
from wrapt import BoundFunctionWrapper, FunctionWrapper

from opentelemetry.instrumentation.boto3sqs import (
    Boto3SQSGetter,
    Boto3SQSInstrumentor,
    Boto3SQSSetter,
)
from opentelemetry.semconv.trace import (
    MessagingDestinationKindValues,
    MessagingOperationValues,
    SpanAttributes,
)
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import SpanKind
from opentelemetry.trace.span import Span, format_span_id, format_trace_id


def _make_sqs_client():
    return boto3.client(
        "sqs",
        region_name="us-east-1",
        aws_access_key_id="dummy",
        aws_secret_access_key="dummy",
    )


class TestBoto3SQSInstrumentor(TestCase):
    def _assert_instrumented(self, client):
        self.assertIsInstance(boto3.client, FunctionWrapper)
        self.assertIsInstance(client.send_message, BoundFunctionWrapper)
        self.assertIsInstance(client.send_message_batch, BoundFunctionWrapper)
        self.assertIsInstance(client.receive_message, BoundFunctionWrapper)
        self.assertIsInstance(client.delete_message, BoundFunctionWrapper)
        self.assertIsInstance(
            client.delete_message_batch, BoundFunctionWrapper
        )

    @staticmethod
    @contextmanager
    def _active_instrumentor():
        Boto3SQSInstrumentor().instrument()
        try:
            yield
        finally:
            Boto3SQSInstrumentor().uninstrument()

    def test_instrument_api_before_client_init(self) -> None:
        with self._active_instrumentor():
            client = _make_sqs_client()
            self._assert_instrumented(client)

    def test_instrument_api_after_client_init(self) -> None:
        client = _make_sqs_client()
        with self._active_instrumentor():
            self._assert_instrumented(client)

    def test_instrument_multiple_clients(self):
        with self._active_instrumentor():
            self._assert_instrumented(_make_sqs_client())
            self._assert_instrumented(_make_sqs_client())


class TestBoto3SQSGetter(TestCase):
    def setUp(self) -> None:
        self.getter = Boto3SQSGetter()

    def test_get_none(self) -> None:
        carrier = {}
        value = self.getter.get(carrier, "test")
        self.assertIsNone(value)

    def test_get_value(self) -> None:
        key = "test"
        value = "value"
        carrier = {key: {"StringValue": value, "DataType": "String"}}
        val = self.getter.get(carrier, key)
        self.assertEqual(val, [value])

    def test_keys(self):
        carrier = {
            "test1": {"StringValue": "value1", "DataType": "String"},
            "test2": {"StringValue": "value2", "DataType": "String"},
        }
        keys = self.getter.keys(carrier)
        self.assertEqual(keys, list(carrier.keys()))

    def test_keys_empty(self):
        keys = self.getter.keys({})
        self.assertEqual(keys, [])


class TestBoto3SQSSetter(TestCase):
    def setUp(self) -> None:
        self.setter = Boto3SQSSetter()

    def test_simple(self):
        original_key = "SomeHeader"
        original_value = {"NumberValue": 1, "DataType": "Number"}
        carrier = {original_key: original_value.copy()}
        key = "test"
        value = "value"
        self.setter.set(carrier, key, value)
        # Ensure the original value is not harmed
        for dict_key, dict_val in carrier[original_key].items():
            self.assertEqual(original_value[dict_key], dict_val)
        # Ensure the new key is added well
        self.assertEqual(carrier[key]["StringValue"], value)


class TestBoto3SQSInstrumentation(TestBase):
    def setUp(self):
        super().setUp()
        self._reset_instrumentor()
        Boto3SQSInstrumentor().instrument()

        self._client = _make_sqs_client()
        self._queue_name = "MyQueue"
        self._queue_url = f"https://sqs.us-east-1.amazonaws.com/123456789012/{self._queue_name}"

    def tearDown(self):
        super().tearDown()
        Boto3SQSInstrumentor().uninstrument()
        self._reset_instrumentor()

    @staticmethod
    def _reset_instrumentor():
        Boto3SQSInstrumentor.received_messages_spans.clear()
        Boto3SQSInstrumentor.current_span_related_to_token = None
        Boto3SQSInstrumentor.current_context_token = None

    @staticmethod
    def _make_aws_response_func(response):
        def _response_func(*args, **kwargs):
            return AWSResponse("http://127.0.0.1", 200, {}, "{}"), response

        return _response_func

    @contextmanager
    def _mocked_endpoint(self, response):
        response_func = self._make_aws_response_func(response)
        with mock.patch(
            "botocore.endpoint.Endpoint.make_request", new=response_func
        ):
            yield

    def _assert_injected_span(self, msg_attrs: Dict[str, Any], span: Span):
        trace_parent = msg_attrs["traceparent"]["StringValue"]
        ctx = span.get_span_context()
        self.assertEqual(
            self._to_trace_parent(ctx.trace_id, ctx.span_id),
            trace_parent.lower(),
        )

    def _default_span_attrs(self):
        return {
            SpanAttributes.MESSAGING_SYSTEM: "aws.sqs",
            SpanAttributes.MESSAGING_DESTINATION: self._queue_name,
            SpanAttributes.MESSAGING_DESTINATION_KIND: MessagingDestinationKindValues.QUEUE.value,
            SpanAttributes.MESSAGING_URL: self._queue_url,
        }

    @staticmethod
    def _to_trace_parent(trace_id: int, span_id: int) -> str:
        return f"00-{format_trace_id(trace_id)}-{format_span_id(span_id)}-01".lower()

    def _get_only_span(self):
        spans = self.get_finished_spans()
        self.assertEqual(1, len(spans))
        return spans[0]

    @staticmethod
    def _make_message(message_id: str, body: str, receipt: str):
        return {
            "MessageId": message_id,
            "ReceiptHandle": receipt,
            "MD5OfBody": "777",
            "Body": body,
            "Attributes": {},
            "MD5OfMessageAttributes": "111",
            "MessageAttributes": {},
        }

    def _add_trace_parent(
        self, message: Dict[str, Any], trace_id: int, span_id: int
    ):
        message["MessageAttributes"]["traceparent"] = {
            "StringValue": self._to_trace_parent(trace_id, span_id),
            "DataType": "String",
        }

    def test_send_message(self):
        message_id = "123456789"
        mock_response = {
            "MD5OfMessageBody": "1234",
            "MD5OfMessageAttributes": "5678",
            "MD5OfMessageSystemAttributes": "9012",
            "MessageId": message_id,
            "SequenceNumber": "0",
        }

        message_attrs = {}

        with self._mocked_endpoint(mock_response):
            self._client.send_message(
                QueueUrl=self._queue_url,
                MessageBody="hello msg",
                MessageAttributes=message_attrs,
            )

        span = self._get_only_span()
        self.assertEqual(f"{self._queue_name} send", span.name)
        self.assertEqual(SpanKind.PRODUCER, span.kind)
        self.assertEqual(
            {
                SpanAttributes.MESSAGING_MESSAGE_ID: message_id,
                **self._default_span_attrs(),
            },
            span.attributes,
        )
        self._assert_injected_span(message_attrs, span)

    def test_receive_message(self):
        msg_def = {
            "1": {"receipt": "01", "trace_id": 10, "span_id": 1},
            "2": {"receipt": "02", "trace_id": 20, "span_id": 2},
        }

        mock_response = {"Messages": []}
        for msg_id, attrs in msg_def.items():
            message = self._make_message(
                msg_id, f"hello {msg_id}", attrs["receipt"]
            )
            self._add_trace_parent(
                message, attrs["trace_id"], attrs["span_id"]
            )
            mock_response["Messages"].append(message)

        message_attr_names = []

        with self._mocked_endpoint(mock_response):
            response = self._client.receive_message(
                QueueUrl=self._queue_url,
                MessageAttributeNames=message_attr_names,
            )

        self.assertIn("traceparent", message_attr_names)

        # receive span
        span = self._get_only_span()
        self.assertEqual(f"{self._queue_name} receive", span.name)
        self.assertEqual(SpanKind.CONSUMER, span.kind)
        self.assertEqual(
            {
                SpanAttributes.MESSAGING_OPERATION: MessagingOperationValues.RECEIVE.value,
                **self._default_span_attrs(),
            },
            span.attributes,
        )

        self.memory_exporter.clear()

        # processing spans
        self.assertEqual(2, len(response["Messages"]))
        for msg in response["Messages"]:
            msg_id = msg["MessageId"]
            attrs = msg_def[msg_id]
            with self._mocked_endpoint(None):
                self._client.delete_message(
                    QueueUrl=self._queue_url, ReceiptHandle=attrs["receipt"]
                )

            span = self._get_only_span()
            self.assertEqual(f"{self._queue_name} process", span.name)

            # processing span attributes
            self.assertEqual(
                {
                    SpanAttributes.MESSAGING_MESSAGE_ID: msg_id,
                    SpanAttributes.MESSAGING_OPERATION: MessagingOperationValues.PROCESS.value,
                    **self._default_span_attrs(),
                },
                span.attributes,
            )

            # processing span links
            self.assertEqual(1, len(span.links))
            link = span.links[0]
            self.assertEqual(attrs["trace_id"], link.context.trace_id)
            self.assertEqual(attrs["span_id"], link.context.span_id)

            self.memory_exporter.clear()
