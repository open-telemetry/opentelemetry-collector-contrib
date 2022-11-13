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
import logging

from opentelemetry.instrumentation.botocore.extensions.types import (
    _AttributeMapT,
    _AwsSdkExtension,
    _BotoResultT,
)
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace.span import Span

_SUPPORTED_OPERATIONS = ["SendMessage", "SendMessageBatch", "ReceiveMessage"]

_logger = logging.getLogger(__name__)


class _SqsExtension(_AwsSdkExtension):
    def extract_attributes(self, attributes: _AttributeMapT):
        queue_url = self._call_context.params.get("QueueUrl")
        if queue_url:
            # TODO: update when semantic conventions exist
            attributes["aws.queue_url"] = queue_url
            attributes[SpanAttributes.MESSAGING_SYSTEM] = "aws.sqs"
            attributes[SpanAttributes.MESSAGING_URL] = queue_url
            try:
                attributes[
                    SpanAttributes.MESSAGING_DESTINATION
                ] = queue_url.split("/")[-1]
            except IndexError:
                _logger.error(
                    "Could not extract messaging destination from '%s'",
                    queue_url,
                )

    def on_success(self, span: Span, result: _BotoResultT):
        operation = self._call_context.operation
        if operation in _SUPPORTED_OPERATIONS:
            try:
                if operation == "SendMessage":
                    span.set_attribute(
                        SpanAttributes.MESSAGING_MESSAGE_ID,
                        result.get("MessageId"),
                    )
                elif operation == "SendMessageBatch" and result.get(
                    "Successful"
                ):
                    span.set_attribute(
                        SpanAttributes.MESSAGING_MESSAGE_ID,
                        result["Successful"][0]["MessageId"],
                    )
                elif operation == "ReceiveMessage" and result.get("Messages"):
                    span.set_attribute(
                        SpanAttributes.MESSAGING_MESSAGE_ID,
                        result["Messages"][0]["MessageId"],
                    )
            except (IndexError, KeyError):
                _logger.error("Could not extract the messaging message ID")
