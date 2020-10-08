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

from unittest.mock import Mock, patch

import botocore.session
from botocore.exceptions import ParamValidationError
from moto import (  # pylint: disable=import-error
    mock_ec2,
    mock_kinesis,
    mock_kms,
    mock_lambda,
    mock_s3,
    mock_sqs,
)

from opentelemetry.instrumentation.botocore import BotocoreInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.test.test_base import TestBase


def assert_span_http_status_code(span, code):
    """Assert on the span"s "http.status_code" tag"""
    tag = span.attributes["http.status_code"]
    assert tag == code, "%r != %r" % (tag, code)


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
        self.assertEqual(span.attributes["aws.agent"], "botocore")
        self.assertEqual(span.attributes["aws.region"], "us-west-2")
        self.assertEqual(span.attributes["aws.operation"], "DescribeInstances")
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(
                attributes={
                    "endpoint": "ec2",
                    "operation": "describeinstances",
                }
            ),
        )
        self.assertEqual(span.name, "ec2.command")

    @mock_ec2
    def test_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        mock_tracer.use_span.return_value.__enter__ = mock_span
        mock_tracer.use_span.return_value.__exit__ = True
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
        self.assertEqual(span.attributes["aws.operation"], "ListBuckets")
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(
                attributes={"endpoint": "s3", "operation": "listbuckets"}
            ),
        )

        # testing for span error
        self.memory_exporter.get_finished_spans()
        with self.assertRaises(ParamValidationError):
            s3.list_objects(bucket="mybucket")
        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[2]
        self.assertEqual(
            span.resource,
            Resource(
                attributes={"endpoint": "s3", "operation": "listobjects"}
            ),
        )

    # Comment test for issue 1088
    @mock_s3
    def test_s3_put(self):
        params = dict(Key="foo", Bucket="mybucket", Body=b"bar")
        s3 = self.session.create_client("s3", region_name="us-west-2")
        location = {"LocationConstraint": "us-west-2"}
        s3.create_bucket(Bucket="mybucket", CreateBucketConfiguration=location)
        s3.put_object(**params)

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 2)
        self.assertEqual(span.attributes["aws.operation"], "CreateBucket")
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(
                attributes={"endpoint": "s3", "operation": "createbucket"}
            ),
        )
        self.assertEqual(spans[1].attributes["aws.operation"], "PutObject")
        self.assertEqual(
            spans[1].resource,
            Resource(attributes={"endpoint": "s3", "operation": "putobject"}),
        )
        self.assertEqual(spans[1].attributes["params.Key"], str(params["Key"]))
        self.assertEqual(
            spans[1].attributes["params.Bucket"], str(params["Bucket"])
        )
        self.assertTrue("params.Body" not in spans[1].attributes.keys())

    @mock_sqs
    def test_sqs_client(self):
        sqs = self.session.create_client("sqs", region_name="us-east-1")

        sqs.list_queues()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(span.attributes["aws.region"], "us-east-1")
        self.assertEqual(span.attributes["aws.operation"], "ListQueues")
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(
                attributes={"endpoint": "sqs", "operation": "listqueues"}
            ),
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
        self.assertEqual(span.attributes["aws.region"], "us-east-1")
        self.assertEqual(span.attributes["aws.operation"], "ListStreams")
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(
                attributes={"endpoint": "kinesis", "operation": "liststreams"}
            ),
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
        self.assertEqual(span.attributes["aws.region"], "us-east-1")
        self.assertEqual(span.attributes["aws.operation"], "ListFunctions")
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(
                attributes={"endpoint": "lambda", "operation": "listfunctions"}
            ),
        )

    @mock_kms
    def test_kms_client(self):
        kms = self.session.create_client("kms", region_name="us-east-1")

        kms.list_keys(Limit=21)

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(span.attributes["aws.region"], "us-east-1")
        self.assertEqual(span.attributes["aws.operation"], "ListKeys")
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(attributes={"endpoint": "kms", "operation": "listkeys"}),
        )

        # checking for protection on sts against security leak
        self.assertTrue("params" not in span.attributes.keys())
