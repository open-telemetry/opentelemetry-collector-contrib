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
import io
import json
import zipfile
from unittest.mock import Mock, patch

import botocore.session
from botocore.exceptions import ParamValidationError
from moto import (  # pylint: disable=import-error
    mock_dynamodb2,
    mock_ec2,
    mock_iam,
    mock_kinesis,
    mock_kms,
    mock_lambda,
    mock_s3,
    mock_sqs,
    mock_sts,
    mock_xray,
)

from opentelemetry import trace as trace_api
from opentelemetry.context import attach, detach, set_value
from opentelemetry.instrumentation.botocore import BotocoreInstrumentor
from opentelemetry.instrumentation.utils import _SUPPRESS_INSTRUMENTATION_KEY
from opentelemetry.propagate import get_global_textmap, set_global_textmap
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.mock_textmap import MockTextMapPropagator
from opentelemetry.test.test_base import TestBase


def get_as_zip_file(file_name, content):
    zip_output = io.BytesIO()
    with zipfile.ZipFile(zip_output, "w", zipfile.ZIP_DEFLATED) as zip_file:
        zip_file.writestr(file_name, content)
    zip_output.seek(0)
    return zip_output.read()


def return_headers_lambda_str():
    pfunc = """
def lambda_handler(event, context):
    print("custom log event")
    headers = event.get('headers', event.get('attributes', {}))
    return headers
"""
    return pfunc


class TestBotocoreInstrumentor(TestBase):
    """Botocore integration testsuite"""

    def setUp(self):
        super().setUp()
        BotocoreInstrumentor().instrument()

        self.session = botocore.session.get_session()
        self.session.set_credentials(
            access_key="access-key", secret_key="secret-key"
        )

    def tearDown(self):
        super().tearDown()
        BotocoreInstrumentor().uninstrument()

    @mock_ec2
    def test_traced_client(self):
        ec2 = self.session.create_client("ec2", region_name="us-west-2")

        ec2.describe_instances()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            span.attributes,
            {
                "aws.operation": "DescribeInstances",
                "aws.region": "us-west-2",
                "aws.request_id": "fdcdcab1-ae5c-489e-9c33-4637c5dda355",
                "aws.service": "ec2",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )
        self.assertEqual(span.name, "ec2")

    @mock_ec2
    def test_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            ec2 = self.session.create_client("ec2", region_name="us-west-2")
            ec2.describe_instances()
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    @mock_ec2
    def test_traced_client_analytics(self):
        ec2 = self.session.create_client("ec2", region_name="us-west-2")
        ec2.describe_instances()

        spans = self.memory_exporter.get_finished_spans()
        assert spans

    @mock_s3
    def test_s3_client(self):
        s3 = self.session.create_client("s3", region_name="us-west-2")

        s3.list_buckets()
        s3.list_buckets()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 2)
        self.assertEqual(
            span.attributes,
            {
                "aws.operation": "ListBuckets",
                "aws.region": "us-west-2",
                "aws.service": "s3",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

        # testing for span error
        self.memory_exporter.get_finished_spans()
        with self.assertRaises(ParamValidationError):
            s3.list_objects(bucket="mybucket")
        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[2]
        self.assertEqual(
            span.attributes,
            {
                "aws.operation": "ListObjects",
                "aws.region": "us-west-2",
                "aws.service": "s3",
            },
        )
        self.assertIs(
            span.status.status_code, trace_api.StatusCode.ERROR,
        )

    # Comment test for issue 1088
    @mock_s3
    def test_s3_put(self):
        params = dict(Key="foo", Bucket="mybucket", Body=b"bar")
        s3 = self.session.create_client("s3", region_name="us-west-2")
        location = {"LocationConstraint": "us-west-2"}
        s3.create_bucket(Bucket="mybucket", CreateBucketConfiguration=location)
        s3.put_object(**params)
        s3.get_object(Bucket="mybucket", Key="foo")

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 3)
        create_bucket_attributes = spans[0].attributes
        self.assertEqual(
            create_bucket_attributes,
            {
                "aws.operation": "CreateBucket",
                "aws.region": "us-west-2",
                "aws.service": "s3",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )
        put_object_attributes = spans[1].attributes
        self.assertEqual(
            put_object_attributes,
            {
                "aws.operation": "PutObject",
                "aws.region": "us-west-2",
                "aws.service": "s3",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )
        self.assertTrue("params.Body" not in spans[1].attributes.keys())
        get_object_attributes = spans[2].attributes
        self.assertEqual(
            get_object_attributes,
            {
                "aws.operation": "GetObject",
                "aws.region": "us-west-2",
                "aws.service": "s3",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

    @mock_sqs
    def test_sqs_client(self):
        sqs = self.session.create_client("sqs", region_name="us-east-1")

        sqs.list_queues()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        actual = span.attributes
        self.assertRegex(actual["aws.request_id"], r"[A-Z0-9]{52}")
        self.assertEqual(
            actual,
            {
                "aws.operation": "ListQueues",
                "aws.region": "us-east-1",
                "aws.request_id": actual["aws.request_id"],
                "aws.service": "sqs",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

    @mock_sqs
    def test_sqs_send_message(self):
        sqs = self.session.create_client("sqs", region_name="us-east-1")

        test_queue_name = "test_queue_name"

        response = sqs.create_queue(QueueName=test_queue_name)

        sqs.send_message(
            QueueUrl=response["QueueUrl"], MessageBody="Test SQS MESSAGE!"
        )

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 2)
        create_queue_attributes = spans[0].attributes
        self.assertRegex(
            create_queue_attributes["aws.request_id"], r"[A-Z0-9]{52}"
        )
        self.assertEqual(
            create_queue_attributes,
            {
                "aws.operation": "CreateQueue",
                "aws.region": "us-east-1",
                "aws.request_id": create_queue_attributes["aws.request_id"],
                "aws.service": "sqs",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )
        send_msg_attributes = spans[1].attributes
        self.assertRegex(
            send_msg_attributes["aws.request_id"], r"[A-Z0-9]{52}"
        )
        self.assertEqual(
            send_msg_attributes,
            {
                "aws.operation": "SendMessage",
                "aws.queue_url": response["QueueUrl"],
                "aws.region": "us-east-1",
                "aws.request_id": send_msg_attributes["aws.request_id"],
                "aws.service": "sqs",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

    @mock_kinesis
    def test_kinesis_client(self):
        kinesis = self.session.create_client(
            "kinesis", region_name="us-east-1"
        )

        kinesis.list_streams()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            span.attributes,
            {
                "aws.operation": "ListStreams",
                "aws.region": "us-east-1",
                "aws.service": "kinesis",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

    @mock_kinesis
    def test_unpatch(self):
        kinesis = self.session.create_client(
            "kinesis", region_name="us-east-1"
        )

        BotocoreInstrumentor().uninstrument()

        kinesis.list_streams()
        spans = self.memory_exporter.get_finished_spans()
        assert not spans, spans

    @mock_sqs
    def test_double_patch(self):
        sqs = self.session.create_client("sqs", region_name="us-east-1")

        BotocoreInstrumentor().instrument()
        BotocoreInstrumentor().instrument()

        sqs.list_queues()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 1)

    @mock_lambda
    def test_lambda_client(self):
        lamb = self.session.create_client("lambda", region_name="us-east-1")

        lamb.list_functions()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            span.attributes,
            {
                "aws.operation": "ListFunctions",
                "aws.region": "us-east-1",
                "aws.service": "lambda",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

    @mock_iam
    def get_role_name(self):
        iam = self.session.create_client("iam", "us-east-1")
        return iam.create_role(
            RoleName="my-role",
            AssumeRolePolicyDocument="some policy",
            Path="/my-path/",
        )["Role"]["Arn"]

    @mock_lambda
    def test_lambda_invoke_propagation(self):

        previous_propagator = get_global_textmap()
        try:
            set_global_textmap(MockTextMapPropagator())

            lamb = self.session.create_client(
                "lambda", region_name="us-east-1"
            )
            lamb.create_function(
                FunctionName="testFunction",
                Runtime="python2.7",
                Role=self.get_role_name(),
                Handler="lambda_function.lambda_handler",
                Code={
                    "ZipFile": get_as_zip_file(
                        "lambda_function.py", return_headers_lambda_str()
                    )
                },
                Description="test lambda function",
                Timeout=3,
                MemorySize=128,
                Publish=True,
            )
            response = lamb.invoke(
                Payload=json.dumps({}),
                FunctionName="testFunction",
                InvocationType="RequestResponse",
            )

            spans = self.memory_exporter.get_finished_spans()
            assert spans
            self.assertEqual(len(spans), 3)

            results = response["Payload"].read().decode("utf-8")
            headers = json.loads(results)

            self.assertIn(MockTextMapPropagator.TRACE_ID_KEY, headers)
            self.assertEqual(
                "0", headers[MockTextMapPropagator.TRACE_ID_KEY],
            )
            self.assertIn(MockTextMapPropagator.SPAN_ID_KEY, headers)
            self.assertEqual(
                "0", headers[MockTextMapPropagator.SPAN_ID_KEY],
            )
        finally:
            set_global_textmap(previous_propagator)

    @mock_kms
    def test_kms_client(self):
        kms = self.session.create_client("kms", region_name="us-east-1")

        kms.list_keys(Limit=21)

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            span.attributes,
            {
                "aws.operation": "ListKeys",
                "aws.region": "us-east-1",
                "aws.service": "kms",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

        # checking for protection on kms against security leak
        self.assertTrue("params" not in span.attributes.keys())

    @mock_sts
    def test_sts_client(self):
        sts = self.session.create_client("sts", region_name="us-east-1")

        sts.get_caller_identity()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(
            span.attributes,
            {
                "aws.operation": "GetCallerIdentity",
                "aws.region": "us-east-1",
                "aws.request_id": "c6104cbe-af31-11e0-8154-cbc7ccf896c7",
                "aws.service": "sts",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )

        # checking for protection on sts against security leak
        self.assertTrue("params" not in span.attributes.keys())

    @mock_ec2
    def test_propagator_injects_into_request(self):
        headers = {}
        previous_propagator = get_global_textmap()

        def check_headers(**kwargs):
            nonlocal headers
            headers = kwargs["request"].headers

        try:
            set_global_textmap(MockTextMapPropagator())

            ec2 = self.session.create_client("ec2", region_name="us-west-2")
            ec2.meta.events.register_first(
                "before-send.ec2.DescribeInstances", check_headers
            )
            ec2.describe_instances()

            spans = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(spans), 1)
            span = spans[0]
            describe_instances_attributes = spans[0].attributes
            self.assertEqual(
                describe_instances_attributes,
                {
                    "aws.operation": "DescribeInstances",
                    "aws.region": "us-west-2",
                    "aws.request_id": "fdcdcab1-ae5c-489e-9c33-4637c5dda355",
                    "aws.service": "ec2",
                    "retry_attempts": 0,
                    SpanAttributes.HTTP_STATUS_CODE: 200,
                },
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
        xray_client = self.session.create_client(
            "xray", region_name="us-east-1"
        )
        token = attach(set_value(_SUPPRESS_INSTRUMENTATION_KEY, True))
        xray_client.put_trace_segments(TraceSegmentDocuments=["str1"])
        xray_client.put_trace_segments(TraceSegmentDocuments=["str2"])
        detach(token)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(0, len(spans))

    @mock_dynamodb2
    def test_dynamodb_client(self):
        ddb = self.session.create_client("dynamodb", region_name="us-west-2")

        test_table_name = "test_table_name"

        ddb.create_table(
            AttributeDefinitions=[
                {"AttributeName": "id", "AttributeType": "S"},
            ],
            KeySchema=[{"AttributeName": "id", "KeyType": "HASH"}],
            ProvisionedThroughput={
                "ReadCapacityUnits": 5,
                "WriteCapacityUnits": 5,
            },
            TableName=test_table_name,
        )

        ddb.put_item(TableName=test_table_name, Item={"id": {"S": "test_key"}})

        ddb.get_item(TableName=test_table_name, Key={"id": {"S": "test_key"}})

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 3)
        create_table_attributes = spans[0].attributes
        self.assertRegex(
            create_table_attributes["aws.request_id"], r"[A-Z0-9]{52}"
        )
        self.assertEqual(
            create_table_attributes,
            {
                "aws.operation": "CreateTable",
                "aws.region": "us-west-2",
                "aws.service": "dynamodb",
                "aws.request_id": create_table_attributes["aws.request_id"],
                "aws.table_name": "test_table_name",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )
        put_item_attributes = spans[1].attributes
        self.assertRegex(
            put_item_attributes["aws.request_id"], r"[A-Z0-9]{52}"
        )
        self.assertEqual(
            put_item_attributes,
            {
                "aws.operation": "PutItem",
                "aws.region": "us-west-2",
                "aws.request_id": put_item_attributes["aws.request_id"],
                "aws.service": "dynamodb",
                "aws.table_name": "test_table_name",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )
        get_item_attributes = spans[2].attributes
        self.assertRegex(
            get_item_attributes["aws.request_id"], r"[A-Z0-9]{52}"
        )
        self.assertEqual(
            get_item_attributes,
            {
                "aws.operation": "GetItem",
                "aws.region": "us-west-2",
                "aws.request_id": get_item_attributes["aws.request_id"],
                "aws.service": "dynamodb",
                "aws.table_name": "test_table_name",
                "retry_attempts": 0,
                SpanAttributes.HTTP_STATUS_CODE: 200,
            },
        )
