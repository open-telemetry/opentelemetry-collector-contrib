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
import json
from unittest.mock import Mock, patch

import botocore.session
from botocore.exceptions import ParamValidationError
from moto import (  # pylint: disable=import-error
    mock_ec2,
    mock_kinesis,
    mock_kms,
    mock_s3,
    mock_sqs,
    mock_sts,
    mock_xray,
)

from opentelemetry import trace as trace_api
from opentelemetry.context import (
    _SUPPRESS_HTTP_INSTRUMENTATION_KEY,
    attach,
    detach,
    set_value,
)
from opentelemetry.instrumentation.botocore import BotocoreInstrumentor
from opentelemetry.instrumentation.utils import _SUPPRESS_INSTRUMENTATION_KEY
from opentelemetry.propagate import get_global_textmap, set_global_textmap
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.mock_textmap import MockTextMapPropagator
from opentelemetry.test.test_base import TestBase

_REQUEST_ID_REGEX_MATCH = r"[A-Z0-9]{52}"


# pylint:disable=too-many-public-methods
class TestBotocoreInstrumentor(TestBase):
    """Botocore integration testsuite"""

    def setUp(self):
        super().setUp()
        BotocoreInstrumentor().instrument()

        self.session = botocore.session.get_session()
        self.session.set_credentials(
            access_key="access-key", secret_key="secret-key"
        )
        self.region = "us-west-2"

    def tearDown(self):
        super().tearDown()
        BotocoreInstrumentor().uninstrument()

    def _make_client(self, service: str):
        return self.session.create_client(service, region_name=self.region)

    def _default_span_attributes(self, service: str, operation: str):
        return {
            SpanAttributes.RPC_SYSTEM: "aws-api",
            SpanAttributes.RPC_SERVICE: service,
            SpanAttributes.RPC_METHOD: operation,
            "aws.region": self.region,
            "retry_attempts": 0,
            SpanAttributes.HTTP_STATUS_CODE: 200,
        }

    def assert_only_span(self):
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(1, len(spans))
        return spans[0]

    def assert_span(
        self,
        service: str,
        operation: str,
        request_id=None,
        attributes=None,
    ):
        span = self.assert_only_span()
        expected = self._default_span_attributes(service, operation)
        if attributes:
            expected.update(attributes)

        span_attributes_request_id = "aws.request_id"
        if request_id is _REQUEST_ID_REGEX_MATCH:
            actual_request_id = span.attributes[span_attributes_request_id]
            self.assertRegex(actual_request_id, _REQUEST_ID_REGEX_MATCH)
            expected[span_attributes_request_id] = actual_request_id
        elif request_id is not None:
            expected[span_attributes_request_id] = request_id

        self.assertSpanHasAttributes(span, expected)
        self.assertEqual(f"{service}.{operation}", span.name)
        return span

    @mock_ec2
    def test_traced_client(self):
        ec2 = self._make_client("ec2")

        ec2.describe_instances()

        request_id = "fdcdcab1-ae5c-489e-9c33-4637c5dda355"
        self.assert_span("EC2", "DescribeInstances", request_id=request_id)

    @mock_ec2
    def test_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            ec2 = self._make_client("ec2")
            ec2.describe_instances()
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    @mock_s3
    def test_exception(self):
        s3 = self._make_client("s3")

        with self.assertRaises(ParamValidationError):
            s3.list_objects(bucket="mybucket")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(1, len(spans))
        span = spans[0]

        expected = self._default_span_attributes("S3", "ListObjects")
        expected.pop(SpanAttributes.HTTP_STATUS_CODE)
        expected.pop("retry_attempts")
        self.assertEqual(expected, span.attributes)
        self.assertIs(span.status.status_code, trace_api.StatusCode.ERROR)

        self.assertEqual(1, len(span.events))
        event = span.events[0]
        self.assertIn(SpanAttributes.EXCEPTION_STACKTRACE, event.attributes)
        self.assertIn(SpanAttributes.EXCEPTION_TYPE, event.attributes)
        self.assertIn(SpanAttributes.EXCEPTION_MESSAGE, event.attributes)

    @mock_s3
    def test_s3_client(self):
        s3 = self._make_client("s3")

        s3.list_buckets()
        self.assert_span("S3", "ListBuckets")

    @mock_s3
    def test_s3_put(self):
        s3 = self._make_client("s3")

        location = {"LocationConstraint": "us-west-2"}
        s3.create_bucket(Bucket="mybucket", CreateBucketConfiguration=location)
        self.assert_span(
            "S3", "CreateBucket", request_id=_REQUEST_ID_REGEX_MATCH
        )
        self.memory_exporter.clear()

        s3.put_object(Key="foo", Bucket="mybucket", Body=b"bar")
        self.assert_span("S3", "PutObject", request_id=_REQUEST_ID_REGEX_MATCH)
        self.memory_exporter.clear()

        s3.get_object(Bucket="mybucket", Key="foo")
        self.assert_span("S3", "GetObject", request_id=_REQUEST_ID_REGEX_MATCH)

    @mock_sqs
    def test_sqs_client(self):
        sqs = self._make_client("sqs")

        sqs.list_queues()

        self.assert_span(
            "SQS", "ListQueues", request_id=_REQUEST_ID_REGEX_MATCH
        )

    @mock_sqs
    def test_sqs_send_message(self):
        sqs = self._make_client("sqs")
        test_queue_name = "test_queue_name"

        response = sqs.create_queue(QueueName=test_queue_name)
        self.assert_span(
            "SQS", "CreateQueue", request_id=_REQUEST_ID_REGEX_MATCH
        )
        self.memory_exporter.clear()

        queue_url = response["QueueUrl"]
        sqs.send_message(QueueUrl=queue_url, MessageBody="Test SQS MESSAGE!")

        self.assert_span(
            "SQS",
            "SendMessage",
            request_id=_REQUEST_ID_REGEX_MATCH,
            attributes={"aws.queue_url": queue_url},
        )

    @mock_kinesis
    def test_kinesis_client(self):
        kinesis = self._make_client("kinesis")

        kinesis.list_streams()
        self.assert_span("Kinesis", "ListStreams")

    @mock_kinesis
    def test_unpatch(self):
        kinesis = self._make_client("kinesis")

        BotocoreInstrumentor().uninstrument()

        kinesis.list_streams()
        self.assertEqual(0, len(self.memory_exporter.get_finished_spans()))

    @mock_ec2
    def test_uninstrument_does_not_inject_headers(self):
        headers = {}
        previous_propagator = get_global_textmap()
        try:
            set_global_textmap(MockTextMapPropagator())

            def intercept_headers(**kwargs):
                headers.update(kwargs["request"].headers)

            ec2 = self._make_client("ec2")

            BotocoreInstrumentor().uninstrument()

            ec2.meta.events.register_first(
                "before-send.ec2.DescribeInstances", intercept_headers
            )
            with self.tracer_provider.get_tracer("test").start_span("parent"):
                ec2.describe_instances()

            self.assertNotIn(MockTextMapPropagator.TRACE_ID_KEY, headers)
            self.assertNotIn(MockTextMapPropagator.SPAN_ID_KEY, headers)
        finally:
            set_global_textmap(previous_propagator)

    @mock_sqs
    def test_double_patch(self):
        sqs = self._make_client("sqs")

        BotocoreInstrumentor().instrument()
        BotocoreInstrumentor().instrument()

        sqs.list_queues()
        self.assert_span(
            "SQS", "ListQueues", request_id=_REQUEST_ID_REGEX_MATCH
        )

    @mock_kms
    def test_kms_client(self):
        kms = self._make_client("kms")

        kms.list_keys(Limit=21)

        span = self.assert_only_span()
        # check for exact attribute set to make sure not to leak any kms secrets
        self.assertEqual(
            self._default_span_attributes("KMS", "ListKeys"), span.attributes
        )

    @mock_sts
    def test_sts_client(self):
        sts = self._make_client("sts")

        sts.get_caller_identity()

        span = self.assert_only_span()
        expected = self._default_span_attributes("STS", "GetCallerIdentity")
        expected["aws.request_id"] = "c6104cbe-af31-11e0-8154-cbc7ccf896c7"
        # check for exact attribute set to make sure not to leak any sts secrets
        self.assertEqual(expected, span.attributes)

    @mock_ec2
    def test_propagator_injects_into_request(self):
        headers = {}
        previous_propagator = get_global_textmap()

        def check_headers(**kwargs):
            nonlocal headers
            headers = kwargs["request"].headers

        try:
            set_global_textmap(MockTextMapPropagator())

            ec2 = self._make_client("ec2")
            ec2.meta.events.register_first(
                "before-send.ec2.DescribeInstances", check_headers
            )
            ec2.describe_instances()

            request_id = "fdcdcab1-ae5c-489e-9c33-4637c5dda355"
            span = self.assert_span(
                "EC2", "DescribeInstances", request_id=request_id
            )

            self.assertIn(MockTextMapPropagator.TRACE_ID_KEY, headers)
            self.assertEqual(
                str(span.get_span_context().trace_id),
                headers[MockTextMapPropagator.TRACE_ID_KEY],
            )
            self.assertIn(MockTextMapPropagator.SPAN_ID_KEY, headers)
            self.assertEqual(
                str(span.get_span_context().span_id),
                headers[MockTextMapPropagator.SPAN_ID_KEY],
            )

        finally:
            set_global_textmap(previous_propagator)

    @mock_xray
    def test_suppress_instrumentation_xray_client(self):
        xray_client = self._make_client("xray")
        token = attach(set_value(_SUPPRESS_INSTRUMENTATION_KEY, True))
        try:
            xray_client.put_trace_segments(TraceSegmentDocuments=["str1"])
            xray_client.put_trace_segments(TraceSegmentDocuments=["str2"])
        finally:
            detach(token)
        self.assertEqual(0, len(self.get_finished_spans()))

    @mock_xray
    def test_suppress_http_instrumentation_xray_client(self):
        xray_client = self._make_client("xray")
        token = attach(set_value(_SUPPRESS_HTTP_INSTRUMENTATION_KEY, True))
        try:
            xray_client.put_trace_segments(TraceSegmentDocuments=["str1"])
            xray_client.put_trace_segments(TraceSegmentDocuments=["str2"])
        finally:
            detach(token)
        self.assertEqual(2, len(self.get_finished_spans()))

    @mock_s3
    def test_request_hook(self):
        request_hook_service_attribute_name = "request_hook.service_name"
        request_hook_operation_attribute_name = "request_hook.operation_name"
        request_hook_api_params_attribute_name = "request_hook.api_params"

        def request_hook(span, service_name, operation_name, api_params):
            hook_attributes = {
                request_hook_service_attribute_name: service_name,
                request_hook_operation_attribute_name: operation_name,
                request_hook_api_params_attribute_name: json.dumps(api_params),
            }

            span.set_attributes(hook_attributes)

        BotocoreInstrumentor().uninstrument()
        BotocoreInstrumentor().instrument(request_hook=request_hook)

        s3 = self._make_client("s3")

        params = {
            "Bucket": "mybucket",
            "CreateBucketConfiguration": {"LocationConstraint": "us-west-2"},
        }
        s3.create_bucket(**params)
        self.assert_span(
            "S3",
            "CreateBucket",
            attributes={
                request_hook_service_attribute_name: "s3",
                request_hook_operation_attribute_name: "CreateBucket",
                request_hook_api_params_attribute_name: json.dumps(params),
            },
        )

    @mock_s3
    def test_response_hook(self):
        response_hook_service_attribute_name = "request_hook.service_name"
        response_hook_operation_attribute_name = "response_hook.operation_name"
        response_hook_result_attribute_name = "response_hook.result"

        def response_hook(span, service_name, operation_name, result):
            hook_attributes = {
                response_hook_service_attribute_name: service_name,
                response_hook_operation_attribute_name: operation_name,
                response_hook_result_attribute_name: len(result["Buckets"]),
            }
            span.set_attributes(hook_attributes)

        BotocoreInstrumentor().uninstrument()
        BotocoreInstrumentor().instrument(response_hook=response_hook)

        s3 = self._make_client("s3")
        s3.list_buckets()
        self.assert_span(
            "S3",
            "ListBuckets",
            attributes={
                response_hook_service_attribute_name: "s3",
                response_hook_operation_attribute_name: "ListBuckets",
                response_hook_result_attribute_name: 0,
            },
        )
