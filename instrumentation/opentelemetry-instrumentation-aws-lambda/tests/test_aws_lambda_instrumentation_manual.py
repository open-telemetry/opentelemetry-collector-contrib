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
import os
from importlib import import_module
from unittest import mock

from opentelemetry.environment_variables import OTEL_PROPAGATORS
from opentelemetry.instrumentation.aws_lambda import (
    _HANDLER,
    _X_AMZN_TRACE_ID,
    OTEL_INSTRUMENTATION_AWS_LAMBDA_FLUSH_TIMEOUT,
    AwsLambdaInstrumentor,
)
from opentelemetry.propagate import get_global_textmap
from opentelemetry.propagators.aws.aws_xray_propagator import (
    TRACE_ID_FIRST_PART_LENGTH,
    TRACE_ID_VERSION,
)
from opentelemetry.semconv.resource import ResourceAttributes
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import SpanKind
from opentelemetry.trace.propagation.tracecontext import (
    TraceContextTextMapPropagator,
)


class MockLambdaContext:
    def __init__(self, aws_request_id, invoked_function_arn):
        self.invoked_function_arn = invoked_function_arn
        self.aws_request_id = aws_request_id


MOCK_LAMBDA_CONTEXT = MockLambdaContext(
    aws_request_id="mock_aws_request_id",
    invoked_function_arn="arn://mock-lambda-function-arn",
)

MOCK_XRAY_TRACE_ID = 0x5FB7331105E8BB83207FA31D4D9CDB4C
MOCK_XRAY_TRACE_ID_STR = f"{MOCK_XRAY_TRACE_ID:x}"
MOCK_XRAY_PARENT_SPAN_ID = 0x3328B8445A6DBAD2
MOCK_XRAY_TRACE_CONTEXT_COMMON = f"Root={TRACE_ID_VERSION}-{MOCK_XRAY_TRACE_ID_STR[:TRACE_ID_FIRST_PART_LENGTH]}-{MOCK_XRAY_TRACE_ID_STR[TRACE_ID_FIRST_PART_LENGTH:]};Parent={MOCK_XRAY_PARENT_SPAN_ID:x}"
MOCK_XRAY_TRACE_CONTEXT_SAMPLED = f"{MOCK_XRAY_TRACE_CONTEXT_COMMON};Sampled=1"
MOCK_XRAY_TRACE_CONTEXT_NOT_SAMPLED = (
    f"{MOCK_XRAY_TRACE_CONTEXT_COMMON};Sampled=0"
)

# See more:
# https://www.w3.org/TR/trace-context/#examples-of-http-traceparent-headers

MOCK_W3C_TRACE_ID = 0x5CE0E9A56015FEC5AADFA328AE398115
MOCK_W3C_PARENT_SPAN_ID = 0xAB54A98CEB1F0AD2
MOCK_W3C_TRACE_CONTEXT_SAMPLED = (
    f"00-{MOCK_W3C_TRACE_ID:x}-{MOCK_W3C_PARENT_SPAN_ID:x}-01"
)

MOCK_W3C_TRACE_STATE_KEY = "vendor_specific_key"
MOCK_W3C_TRACE_STATE_VALUE = "test_value"


def mock_execute_lambda(event=None):
    """Mocks the AWS Lambda execution.

    NOTE: We don't use `moto`'s `mock_lambda` because we are not instrumenting
    calls to AWS Lambda using the AWS SDK. Instead, we are instrumenting AWS
    Lambda itself.

    See more:
    https://docs.aws.amazon.com/lambda/latest/dg/runtimes-modify.html#runtime-wrapper

    Args:
        event: The Lambda event which may or may not be used by instrumentation.
    """

    module_name, handler_name = os.environ[_HANDLER].rsplit(".", 1)
    handler_module = import_module(module_name.replace("/", "."))
    getattr(handler_module, handler_name)(event, MOCK_LAMBDA_CONTEXT)


class TestAwsLambdaInstrumentor(TestBase):
    """AWS Lambda Instrumentation Testsuite"""

    def setUp(self):
        super().setUp()
        self.common_env_patch = mock.patch.dict(
            "os.environ",
            {_HANDLER: "mocks.lambda_function.handler"},
        )
        self.common_env_patch.start()

        # NOTE: Whether AwsLambdaInstrumentor().instrument() is run is decided
        # by each test case. It depends on if the test is for auto or manual
        # instrumentation.

    def tearDown(self):
        super().tearDown()
        self.common_env_patch.stop()
        AwsLambdaInstrumentor().uninstrument()

    def test_active_tracing(self):
        test_env_patch = mock.patch.dict(
            "os.environ",
            {
                **os.environ,
                # Using Active tracing
                _X_AMZN_TRACE_ID: MOCK_XRAY_TRACE_CONTEXT_SAMPLED,
            },
        )
        test_env_patch.start()

        AwsLambdaInstrumentor().instrument()

        mock_execute_lambda()

        spans = self.memory_exporter.get_finished_spans()

        assert spans

        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.name, os.environ[_HANDLER])
        self.assertEqual(span.get_span_context().trace_id, MOCK_XRAY_TRACE_ID)
        self.assertEqual(span.kind, SpanKind.SERVER)
        self.assertSpanHasAttributes(
            span,
            {
                ResourceAttributes.FAAS_ID: MOCK_LAMBDA_CONTEXT.invoked_function_arn,
                SpanAttributes.FAAS_EXECUTION: MOCK_LAMBDA_CONTEXT.aws_request_id,
            },
        )

        parent_context = span.parent
        self.assertEqual(
            parent_context.trace_id, span.get_span_context().trace_id
        )
        self.assertEqual(parent_context.span_id, MOCK_XRAY_PARENT_SPAN_ID)
        self.assertTrue(parent_context.is_remote)

        test_env_patch.stop()

    def test_parent_context_from_lambda_event(self):
        test_env_patch = mock.patch.dict(
            "os.environ",
            {
                **os.environ,
                # NOT Active Tracing
                _X_AMZN_TRACE_ID: MOCK_XRAY_TRACE_CONTEXT_NOT_SAMPLED,
                # NOT using the X-Ray Propagator
                OTEL_PROPAGATORS: "tracecontext",
            },
        )
        test_env_patch.start()

        AwsLambdaInstrumentor().instrument()

        mock_execute_lambda(
            {
                "headers": {
                    TraceContextTextMapPropagator._TRACEPARENT_HEADER_NAME: MOCK_W3C_TRACE_CONTEXT_SAMPLED,
                    TraceContextTextMapPropagator._TRACESTATE_HEADER_NAME: f"{MOCK_W3C_TRACE_STATE_KEY}={MOCK_W3C_TRACE_STATE_VALUE},foo=1,bar=2",
                }
            }
        )

        spans = self.memory_exporter.get_finished_spans()

        assert spans

        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.get_span_context().trace_id, MOCK_W3C_TRACE_ID)

        parent_context = span.parent
        self.assertEqual(
            parent_context.trace_id, span.get_span_context().trace_id
        )
        self.assertEqual(parent_context.span_id, MOCK_W3C_PARENT_SPAN_ID)
        self.assertEqual(len(parent_context.trace_state), 3)
        self.assertEqual(
            parent_context.trace_state.get(MOCK_W3C_TRACE_STATE_KEY),
            MOCK_W3C_TRACE_STATE_VALUE,
        )
        self.assertTrue(parent_context.is_remote)

        test_env_patch.stop()

    def test_using_custom_extractor(self):
        def custom_event_context_extractor(lambda_event):
            return get_global_textmap().extract(lambda_event["foo"]["headers"])

        test_env_patch = mock.patch.dict(
            "os.environ",
            {
                **os.environ,
                # NOT Active Tracing
                _X_AMZN_TRACE_ID: MOCK_XRAY_TRACE_CONTEXT_NOT_SAMPLED,
                # NOT using the X-Ray Propagator
                OTEL_PROPAGATORS: "tracecontext",
            },
        )
        test_env_patch.start()

        AwsLambdaInstrumentor().instrument(
            event_context_extractor=custom_event_context_extractor,
        )

        mock_execute_lambda(
            {
                "foo": {
                    "headers": {
                        TraceContextTextMapPropagator._TRACEPARENT_HEADER_NAME: MOCK_W3C_TRACE_CONTEXT_SAMPLED,
                        TraceContextTextMapPropagator._TRACESTATE_HEADER_NAME: f"{MOCK_W3C_TRACE_STATE_KEY}={MOCK_W3C_TRACE_STATE_VALUE},foo=1,bar=2",
                    }
                }
            }
        )

        spans = self.memory_exporter.get_finished_spans()

        assert spans

        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.get_span_context().trace_id, MOCK_W3C_TRACE_ID)

        parent_context = span.parent
        self.assertEqual(
            parent_context.trace_id, span.get_span_context().trace_id
        )
        self.assertEqual(parent_context.span_id, MOCK_W3C_PARENT_SPAN_ID)
        self.assertEqual(len(parent_context.trace_state), 3)
        self.assertEqual(
            parent_context.trace_state.get(MOCK_W3C_TRACE_STATE_KEY),
            MOCK_W3C_TRACE_STATE_VALUE,
        )
        self.assertTrue(parent_context.is_remote)

        test_env_patch.stop()

    def test_lambda_no_error_with_invalid_flush_timeout(self):

        test_env_patch = mock.patch.dict(
            "os.environ",
            {
                **os.environ,
                # NOT Active Tracing
                _X_AMZN_TRACE_ID: MOCK_XRAY_TRACE_CONTEXT_NOT_SAMPLED,
                # NOT using the X-Ray Propagator
                OTEL_PROPAGATORS: "tracecontext",
                OTEL_INSTRUMENTATION_AWS_LAMBDA_FLUSH_TIMEOUT: "invalid-timeout-string",
            },
        )
        test_env_patch.start()

        AwsLambdaInstrumentor().instrument()

        mock_execute_lambda()

        spans = self.memory_exporter.get_finished_spans()

        assert spans

        self.assertEqual(len(spans), 1)

        test_env_patch.stop()

    def test_lambda_handles_multiple_consumers(self):
        test_env_patch = mock.patch.dict(
            "os.environ",
            {
                **os.environ,
                # NOT Active Tracing
                _X_AMZN_TRACE_ID: MOCK_XRAY_TRACE_CONTEXT_NOT_SAMPLED,
                # NOT using the X-Ray Propagator
                OTEL_PROPAGATORS: "tracecontext",
            },
        )
        test_env_patch.start()

        AwsLambdaInstrumentor().instrument()

        mock_execute_lambda({"Records": [{"eventSource": "aws:sqs"}]})
        mock_execute_lambda({"Records": [{"eventSource": "aws:s3"}]})
        mock_execute_lambda({"Records": [{"eventSource": "aws:sns"}]})
        mock_execute_lambda({"Records": [{"eventSource": "aws:dynamodb"}]})

        spans = self.memory_exporter.get_finished_spans()

        assert spans

        test_env_patch.stop()
