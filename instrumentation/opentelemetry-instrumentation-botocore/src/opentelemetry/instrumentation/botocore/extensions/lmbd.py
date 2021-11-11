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

import abc
import inspect
import json
import re
from typing import Dict

from opentelemetry.instrumentation.botocore.extensions.types import (
    _AttributeMapT,
    _AwsSdkCallContext,
    _AwsSdkExtension,
)
from opentelemetry.propagate import inject
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace.span import Span


class _LambdaOperation(abc.ABC):
    @classmethod
    @abc.abstractmethod
    def operation_name(cls):
        pass

    @classmethod
    def prepare_attributes(
        cls, call_context: _AwsSdkCallContext, attributes: _AttributeMapT
    ):
        pass

    @classmethod
    def before_service_call(cls, call_context: _AwsSdkCallContext, span: Span):
        pass


class _OpInvoke(_LambdaOperation):
    # https://docs.aws.amazon.com/lambda/latest/dg/API_Invoke.html#API_Invoke_RequestParameters
    ARN_LAMBDA_PATTERN = re.compile(
        "(?:arn:(?:aws[a-zA-Z-]*)?:lambda:)?"
        "(?:[a-z]{2}(?:-gov)?-[a-z]+-\\d{1}:)?(?:\\d{12}:)?"
        "(?:function:)?([a-zA-Z0-9-_\\.]+)(?::(?:\\$LATEST|[a-zA-Z0-9-_]+))?"
    )

    @classmethod
    def operation_name(cls):
        return "Invoke"

    @classmethod
    def extract_attributes(
        cls, call_context: _AwsSdkCallContext, attributes: _AttributeMapT
    ):
        attributes[SpanAttributes.FAAS_INVOKED_PROVIDER] = "aws"
        attributes[
            SpanAttributes.FAAS_INVOKED_NAME
        ] = cls._parse_function_name(call_context)
        attributes[SpanAttributes.FAAS_INVOKED_REGION] = call_context.region

    @classmethod
    def _parse_function_name(cls, call_context: _AwsSdkCallContext):
        function_name_or_arn = call_context.params.get("FunctionName")
        matches = cls.ARN_LAMBDA_PATTERN.match(function_name_or_arn)
        function_name = matches.group(1)
        return function_name_or_arn if function_name is None else function_name

    @classmethod
    def before_service_call(cls, call_context: _AwsSdkCallContext, span: Span):
        cls._inject_current_span(call_context)

    @classmethod
    def _inject_current_span(cls, call_context: _AwsSdkCallContext):
        payload_str = call_context.params.get("Payload")
        if payload_str is None:
            return

        # TODO: reconsider propagation via payload as it manipulates input of the called lambda function
        try:
            payload = json.loads(payload_str)
            headers = payload.get("headers", {})
            inject(headers)
            payload["headers"] = headers
            call_context.params["Payload"] = json.dumps(payload)
        except ValueError:
            pass


################################################################################
# Lambda extension
################################################################################

_OPERATION_MAPPING = {
    op.operation_name(): op
    for op in globals().values()
    if inspect.isclass(op)
    and issubclass(op, _LambdaOperation)
    and not inspect.isabstract(op)
}  # type: Dict[str, _LambdaOperation]


class _LambdaExtension(_AwsSdkExtension):
    def __init__(self, call_context: _AwsSdkCallContext):
        super().__init__(call_context)
        self._op = _OPERATION_MAPPING.get(call_context.operation)

    def extract_attributes(self, attributes: _AttributeMapT):
        if self._op is None:
            return

        self._op.extract_attributes(self._call_context, attributes)

    def before_service_call(self, span: Span):
        if self._op is None:
            return

        self._op.before_service_call(self._call_context, span)
