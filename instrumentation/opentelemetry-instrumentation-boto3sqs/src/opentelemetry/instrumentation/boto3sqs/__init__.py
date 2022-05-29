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
Instrument `boto3sqs`_ to trace SQS applications.

.. _boto3sqs: https://boto3.amazonaws.com/v1/documentation/api/latest/reference/services/sqs.html


Usage
-----

.. code:: python

    import boto3
    from opentelemetry.instrumentation.boto3sqs import Boto3SQSInstrumentor


    Boto3SQSInstrumentor().instrument()
"""
import logging
from typing import Any, Collection, Dict, Generator, List, Optional

import boto3
import botocore.client
from wrapt import wrap_function_wrapper

from opentelemetry import context, propagate, trace
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import (
    _SUPPRESS_INSTRUMENTATION_KEY,
    unwrap,
)
from opentelemetry.propagators.textmap import CarrierT, Getter, Setter
from opentelemetry.semconv.trace import (
    MessagingDestinationKindValues,
    MessagingOperationValues,
    SpanAttributes,
)
from opentelemetry.trace import Link, Span, SpanKind, Tracer, TracerProvider

from .package import _instruments
from .version import __version__

_logger = logging.getLogger(__name__)
# We use this prefix so we can request all instrumentation MessageAttributeNames with a wildcard, without harming
# existing filters
_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER: str = "otel."
_OTEL_IDENTIFIER_LENGTH = len(_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER)


class Boto3SQSGetter(Getter[CarrierT]):
    def get(self, carrier: CarrierT, key: str) -> Optional[List[str]]:
        value = carrier.get(f"{_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER}{key}", {})
        if not value:
            return None
        return [value.get("StringValue")]

    def keys(self, carrier: CarrierT) -> List[str]:
        return [
            key[_OTEL_IDENTIFIER_LENGTH:]
            if key.startswith(_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER)
            else key
            for key in carrier.keys()
        ]


class Boto3SQSSetter(Setter[CarrierT]):
    def set(self, carrier: CarrierT, key: str, value: str) -> None:
        # This is a limitation defined by AWS for SQS MessageAttributes size
        if len(carrier.items()) < 10:
            carrier[f"{_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER}{key}"] = {
                "StringValue": value,
                "DataType": "String",
            }
        else:
            _logger.warning(
                "Boto3 SQS instrumentation: cannot set context propagation on SQS/SNS message due to maximum amount of "
                "MessageAttributes"
            )


boto3sqs_getter = Boto3SQSGetter()
boto3sqs_setter = Boto3SQSSetter()


# pylint: disable=attribute-defined-outside-init
class Boto3SQSInstrumentor(BaseInstrumentor):
    received_messages_spans: Dict[str, Span] = {}
    current_span_related_to_token: Span = None
    current_context_token = None

    class ContextableList(list):
        """
        Since the classic way to process SQS messages is using a `for` loop, without a well defined scope like a
        callback - we are doing something similar to the instrumentaiton of Kafka-python and instrumenting the
        `__iter__` functions and the `__getitem__` functions to set the span context of the addressed message. Since
        the return value from an `SQS.ReceiveMessage` returns a builtin list, we cannot wrap it and change all of the
        calls for `list.__iter__` and `list.__getitem__` - therefore we use ContextableList. It is bound to the
        received_messages_spans dict
        """

        def __getitem__(self, key: int) -> Any:
            retval = super(
                Boto3SQSInstrumentor.ContextableList, self
            ).__getitem__(key)
            if not isinstance(retval, dict):
                return retval
            receipt_handle = retval.get("ReceiptHandle")
            if not receipt_handle:
                return retval
            started_span = Boto3SQSInstrumentor.received_messages_spans.get(
                receipt_handle
            )
            if started_span is None:
                return retval
            if Boto3SQSInstrumentor.current_context_token:
                context.detach(Boto3SQSInstrumentor.current_context_token)
            Boto3SQSInstrumentor.current_context_token = context.attach(
                trace.set_span_in_context(started_span)
            )
            Boto3SQSInstrumentor.current_span_related_to_token = started_span
            return retval

        def __iter__(self) -> Generator:
            index = 0
            while index < len(self):
                yield self[index]
                index = index + 1

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    @staticmethod
    def _enrich_span(
        span: Span,
        queue_name: str,
        conversation_id: Optional[str] = None,
        operation: Optional[MessagingOperationValues] = None,
        message_id: Optional[str] = None,
    ) -> None:
        if not span.is_recording():
            return
        span.set_attribute(SpanAttributes.MESSAGING_SYSTEM, "aws.sqs")
        span.set_attribute(SpanAttributes.MESSAGING_DESTINATION, queue_name)
        span.set_attribute(
            SpanAttributes.MESSAGING_DESTINATION_KIND,
            MessagingDestinationKindValues.QUEUE.value,
        )
        if operation:
            span.set_attribute(
                SpanAttributes.MESSAGING_OPERATION, operation.value
            )
        else:
            span.set_attribute(SpanAttributes.MESSAGING_TEMP_DESTINATION, True)
        if conversation_id:
            span.set_attribute(
                SpanAttributes.MESSAGING_CONVERSATION_ID, conversation_id
            )
        if message_id:
            span.set_attribute(SpanAttributes.MESSAGING_MESSAGE_ID, message_id)

    @staticmethod
    def _safe_end_processing_span(receipt_handle: str) -> None:
        started_span: Span = Boto3SQSInstrumentor.received_messages_spans.pop(
            receipt_handle, None
        )
        if started_span:
            if (
                Boto3SQSInstrumentor.current_span_related_to_token
                == started_span
            ):
                context.detach(Boto3SQSInstrumentor.current_context_token)
                Boto3SQSInstrumentor.current_context_token = None
            started_span.end()

    @staticmethod
    def _extract_queue_name_from_url(queue_url: str) -> str:
        # A Queue name cannot have the `/` char, therefore we can return the part after the last /
        return queue_url.split("/")[-1]

    def _create_processing_span(
        self, queue_name: str, receipt_handle: str, message: Dict[str, Any]
    ) -> None:
        message_attributes = message.get("MessageAttributes", {})
        links = []
        ctx = propagate.extract(message_attributes, getter=boto3sqs_getter)
        if ctx:
            for item in ctx.values():
                if hasattr(item, "get_span_context"):
                    links.append(Link(context=item.get_span_context()))
        span = self._tracer.start_span(
            name=f"{queue_name} process", links=links, kind=SpanKind.CONSUMER
        )
        with trace.use_span(span):
            message_id = message.get("MessageId")
            Boto3SQSInstrumentor.received_messages_spans[receipt_handle] = span
            Boto3SQSInstrumentor._enrich_span(
                span,
                queue_name,
                message_id=message_id,
                operation=MessagingOperationValues.PROCESS,
            )

    def _wrap_send_message(self) -> None:
        def send_wrapper(wrapped, instance, args, kwargs):
            if context.get_value(_SUPPRESS_INSTRUMENTATION_KEY):
                return wrapped(*args, **kwargs)
            queue_url = kwargs.get("QueueUrl")
            # The method expect QueueUrl and Entries params, so if they are None, we call wrapped to receive the
            # original exception
            queue_name = Boto3SQSInstrumentor._extract_queue_name_from_url(
                queue_url
            )
            with self._tracer.start_as_current_span(
                name=f"{queue_name} send",
                kind=SpanKind.PRODUCER,
                end_on_exit=True,
            ) as span:
                Boto3SQSInstrumentor._enrich_span(span, queue_name)
                attributes = kwargs.pop("MessageAttributes", {})
                propagate.inject(attributes, setter=boto3sqs_setter)
                retval = wrapped(*args, MessageAttributes=attributes, **kwargs)
                message_id = retval.get("MessageId")
                if message_id:
                    if span.is_recording():
                        span.set_attribute(
                            SpanAttributes.MESSAGING_MESSAGE_ID, message_id
                        )
                return retval

        wrap_function_wrapper(self._sqs_class, "send_message", send_wrapper)

    def _wrap_send_message_batch(self) -> None:
        def send_batch_wrapper(wrapped, instance, args, kwargs):
            queue_url = kwargs.get("QueueUrl")
            entries = kwargs.get("Entries")
            # The method expect QueueUrl and Entries params, so if they are None, we call wrapped to receive the
            # original exception
            if (
                context.get_value(_SUPPRESS_INSTRUMENTATION_KEY)
                or not queue_url
                or not entries
            ):
                return wrapped(*args, **kwargs)
            queue_name = Boto3SQSInstrumentor._extract_queue_name_from_url(
                queue_url
            )
            ids_to_spans: Dict[str, Span] = {}
            for entry in entries:
                entry_id = entry["Id"]
                span = self._tracer.start_span(
                    name=f"{queue_name} send",
                    kind=SpanKind.PRODUCER,
                )
                ids_to_spans[entry_id] = span
                Boto3SQSInstrumentor._enrich_span(
                    span, queue_name, conversation_id=entry_id
                )
                with trace.use_span(span):
                    if "MessageAttributes" not in entry:
                        entry["MessageAttributes"] = {}
                    propagate.inject(
                        entry["MessageAttributes"], setter=boto3sqs_setter
                    )
            retval = wrapped(*args, **kwargs)
            for successful_messages in retval["Successful"]:
                message_identifier = successful_messages["Id"]
                message_span = ids_to_spans.get(message_identifier)
                if message_span:
                    if message_span.is_recording():
                        message_span.set_attribute(
                            SpanAttributes.MESSAGING_MESSAGE_ID,
                            successful_messages.get("MessageId"),
                        )
            for span in ids_to_spans.values():
                span.end()
            return retval

        wrap_function_wrapper(
            self._sqs_class, "send_message_batch", send_batch_wrapper
        )

    def _wrap_receive_message(self) -> None:
        def receive_message_wrapper(wrapped, instance, args, kwargs):
            queue_url = kwargs.get("QueueUrl")
            message_attribute_names = kwargs.pop("MessageAttributeNames", [])
            message_attribute_names.append(
                f"{_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER}*"
            )
            queue_name = Boto3SQSInstrumentor._extract_queue_name_from_url(
                queue_url
            )
            with self._tracer.start_as_current_span(
                name=f"{queue_name} receive",
                end_on_exit=True,
                kind=SpanKind.CONSUMER,
            ) as span:
                Boto3SQSInstrumentor._enrich_span(
                    span,
                    queue_name,
                    operation=MessagingOperationValues.RECEIVE,
                )
                retval = wrapped(
                    *args,
                    MessageAttributeNames=message_attribute_names,
                    **kwargs,
                )
                messages = retval.get("Messages", [])
                if not messages:
                    return retval
                for message in messages:
                    receipt_handle = message.get("ReceiptHandle")
                    if not receipt_handle:
                        continue
                    Boto3SQSInstrumentor._safe_end_processing_span(
                        receipt_handle
                    )
                    self._create_processing_span(
                        queue_name, receipt_handle, message
                    )
                retval["Messages"] = Boto3SQSInstrumentor.ContextableList(
                    messages
                )
            return retval

        wrap_function_wrapper(
            self._sqs_class, "receive_message", receive_message_wrapper
        )

    def _wrap_delete_message(self) -> None:
        def delete_message_wrapper(wrapped, instance, args, kwargs):
            receipt_handle = kwargs.get("ReceiptHandle")
            if receipt_handle:
                Boto3SQSInstrumentor._safe_end_processing_span(receipt_handle)
            return wrapped(*args, **kwargs)

        wrap_function_wrapper(
            self._sqs_class, "delete_message", delete_message_wrapper
        )

    def _wrap_delete_message_batch(self) -> None:
        def delete_message_wrapper_batch(wrapped, instance, args, kwargs):
            entries = kwargs.get("Entries")
            for entry in entries:
                receipt_handle = entry.get("ReceiptHandle")
                if receipt_handle:
                    Boto3SQSInstrumentor._safe_end_processing_span(
                        receipt_handle
                    )
                return wrapped(*args, **kwargs)

        wrap_function_wrapper(
            self._sqs_class,
            "delete_message_batch",
            delete_message_wrapper_batch,
        )

    def _wrap_client_creation(self) -> None:
        """
        Since botocore creates classes on the fly using schemas, the SQS class is not necesraily created upon the call
        of `instrument()`. Therefore we need to wrap the creation of the boto3 client, which triggers the creation of
        the SQS client.
        """

        def client_wrapper(wrapped, instance, args, kwargs):
            retval = wrapped(*args, **kwargs)
            if not self._did_decorate:
                self._decorate_sqs()
            return retval

        wrap_function_wrapper(boto3, "client", client_wrapper)

    def _decorate_sqs(self) -> None:
        """
        Since botocore creates classes on the fly using schemas, we try to find the class that inherits from the base
        class and is SQS to wrap.
        """
        # We define SQS client as the only client that implements send_message_batch
        sqs_class = [
            cls
            for cls in botocore.client.BaseClient.__subclasses__()
            if hasattr(cls, "send_message_batch")
        ]
        if sqs_class:
            self._sqs_class = sqs_class[0]
            self._did_decorate = True
            self._wrap_send_message()
            self._wrap_send_message_batch()
            self._wrap_receive_message()
            self._wrap_delete_message()
            self._wrap_delete_message_batch()

    def _un_decorate_sqs(self) -> None:
        if self._did_decorate:
            unwrap(self._sqs_class, "send_message")
            unwrap(self._sqs_class, "send_message_batch")
            unwrap(self._sqs_class, "receive_message")
            unwrap(self._sqs_class, "delete_message")
            unwrap(self._sqs_class, "delete_message_batch")
            self._did_decorate = False

    def _instrument(self, **kwargs: Dict[str, Any]) -> None:
        self._did_decorate: bool = False
        self._tracer_provider: Optional[TracerProvider] = kwargs.get(
            "tracer_provider"
        )
        self._tracer: Tracer = trace.get_tracer(
            __name__, __version__, self._tracer_provider
        )
        self._wrap_client_creation()
        self._decorate_sqs()

    def _uninstrument(self, **kwargs: Dict[str, Any]) -> None:
        unwrap(boto3, "client")
        self._un_decorate_sqs()
