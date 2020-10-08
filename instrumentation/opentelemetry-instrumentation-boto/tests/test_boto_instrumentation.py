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

from unittest import skipUnless
from unittest.mock import Mock, patch

import boto.awslambda
import boto.ec2
import boto.elasticache
import boto.s3
import boto.sts
from moto import (  # pylint: disable=import-error
    mock_ec2_deprecated,
    mock_lambda_deprecated,
    mock_s3_deprecated,
    mock_sts_deprecated,
)

from opentelemetry.instrumentation.boto import BotoInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.test.test_base import TestBase


def assert_span_http_status_code(span, code):
    """Assert on the span's 'http.status_code' tag"""
    tag = span.attributes["http.status_code"]
    assert tag == code, "%r != %r" % (tag, code)


class TestBotoInstrumentor(TestBase):
    """Botocore integration testsuite"""

    def setUp(self):
        super().setUp()
        BotoInstrumentor().instrument()

    def tearDown(self):
        BotoInstrumentor().uninstrument()

    @mock_ec2_deprecated
    def test_ec2_client(self):
        ec2 = boto.ec2.connect_to_region("us-west-2")

        ec2.get_all_instances()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.attributes["aws.operation"], "DescribeInstances")
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.attributes["http.method"], "POST")
        self.assertEqual(span.attributes["aws.region"], "us-west-2")

        # Create an instance
        ec2.run_instances(21)
        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 2)
        span = spans[1]
        self.assertEqual(span.attributes["aws.operation"], "RunInstances")
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(
                attributes={"endpoint": "ec2", "http_method": "runinstances"}
            ),
        )
        self.assertEqual(span.attributes["http.method"], "POST")
        self.assertEqual(span.attributes["aws.region"], "us-west-2")
        self.assertEqual(span.name, "ec2.command")

    @mock_ec2_deprecated
    def test_not_recording(self):
        mock_tracer = Mock()
        mock_span = Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        mock_tracer.use_span.return_value.__enter__ = mock_span
        mock_tracer.use_span.return_value.__exit__ = True
        with patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            ec2 = boto.ec2.connect_to_region("us-west-2")
            ec2.get_all_instances()
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    @mock_ec2_deprecated
    def test_analytics_enabled_with_rate(self):
        ec2 = boto.ec2.connect_to_region("us-west-2")

        ec2.get_all_instances()

        spans = self.memory_exporter.get_finished_spans()
        assert spans

    @mock_ec2_deprecated
    def test_analytics_enabled_without_rate(self):
        ec2 = boto.ec2.connect_to_region("us-west-2")

        ec2.get_all_instances()

        spans = self.memory_exporter.get_finished_spans()
        assert spans

    @mock_s3_deprecated
    def test_s3_client(self):
        s3 = boto.s3.connect_to_region("us-east-1")

        s3.get_all_buckets()
        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 1)
        span = spans[0]
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.attributes["http.method"], "GET")
        self.assertEqual(span.attributes["aws.operation"], "get_all_buckets")

        # Create a bucket command
        s3.create_bucket("cheese")
        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 2)
        span = spans[1]
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.attributes["http.method"], "PUT")
        self.assertEqual(span.attributes["path"], "/")
        self.assertEqual(span.attributes["aws.operation"], "create_bucket")

        # Get the created bucket
        s3.get_bucket("cheese")
        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 3)
        span = spans[2]
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(attributes={"endpoint": "s3", "http_method": "head"}),
        )
        self.assertEqual(span.attributes["http.method"], "HEAD")
        self.assertEqual(span.attributes["aws.operation"], "head_bucket")
        self.assertEqual(span.name, "s3.command")

        # Checking for resource incase of error
        try:
            s3.get_bucket("big_bucket")
        except Exception:  # pylint: disable=broad-except
            spans = self.memory_exporter.get_finished_spans()
            assert spans
            span = spans[2]
            self.assertEqual(
                span.resource,
                Resource(attributes={"endpoint": "s3", "http_method": "head"}),
            )

    @mock_s3_deprecated
    def test_s3_put(self):
        s3 = boto.s3.connect_to_region("us-east-1")
        s3.create_bucket("mybucket")
        bucket = s3.get_bucket("mybucket")
        key = boto.s3.key.Key(bucket)
        key.key = "foo"
        key.set_contents_from_string("bar")

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        # create bucket
        self.assertEqual(len(spans), 3)
        self.assertEqual(spans[0].attributes["aws.operation"], "create_bucket")
        assert_span_http_status_code(spans[0], 200)
        self.assertEqual(
            spans[0].resource,
            Resource(attributes={"endpoint": "s3", "http_method": "put"}),
        )
        # get bucket
        self.assertEqual(spans[1].attributes["aws.operation"], "head_bucket")
        self.assertEqual(
            spans[1].resource,
            Resource(attributes={"endpoint": "s3", "http_method": "head"}),
        )
        # put object
        self.assertEqual(
            spans[2].attributes["aws.operation"], "_send_file_internal"
        )
        self.assertEqual(
            spans[2].resource,
            Resource(attributes={"endpoint": "s3", "http_method": "put"}),
        )

    @mock_lambda_deprecated
    def test_unpatch(self):

        lamb = boto.awslambda.connect_to_region("us-east-2")

        BotoInstrumentor().uninstrument()

        # multiple calls
        lamb.list_functions()
        spans = self.memory_exporter.get_finished_spans()
        assert not spans, spans

    @mock_s3_deprecated
    def test_double_patch(self):
        s3 = boto.s3.connect_to_region("us-east-1")

        BotoInstrumentor().instrument()
        BotoInstrumentor().instrument()

        # Get the created bucket
        s3.create_bucket("cheese")
        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 1)

    @mock_lambda_deprecated
    def test_lambda_client(self):
        lamb = boto.awslambda.connect_to_region("us-east-2")

        # multiple calls
        lamb.list_functions()
        lamb.list_functions()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 2)
        span = spans[0]
        assert_span_http_status_code(span, 200)
        self.assertEqual(
            span.resource,
            Resource(attributes={"endpoint": "lambda", "http_method": "get"}),
        )
        self.assertEqual(span.attributes["http.method"], "GET")
        self.assertEqual(span.attributes["aws.region"], "us-east-2")
        self.assertEqual(span.attributes["aws.operation"], "list_functions")

    @mock_sts_deprecated
    def test_sts_client(self):
        sts = boto.sts.connect_to_region("us-west-2")

        sts.get_federation_token(12, duration=10)

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(
            span.resource,
            Resource(
                attributes={
                    "endpoint": "sts",
                    "http_method": "getfederationtoken",
                }
            ),
        )
        self.assertEqual(span.attributes["aws.region"], "us-west-2")
        self.assertEqual(
            span.attributes["aws.operation"], "GetFederationToken"
        )

        # checking for protection on sts against security leak
        self.assertTrue("args.path" not in span.attributes.keys())

    @skipUnless(
        False,
        (
            "Test to reproduce the case where args sent to patched function "
            "are None, can't be mocked: needs AWS credentials"
        ),
    )
    def test_elasticache_client(self):
        elasticache = boto.elasticache.connect_to_region("us-west-2")

        elasticache.describe_cache_clusters()

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        span = spans[0]
        self.assertEqual(
            span.resource, Resource(attributes={"endpoint": "elasticcache"})
        )
        self.assertEqual(span.attributes["aws.region"], "us-west-2")
