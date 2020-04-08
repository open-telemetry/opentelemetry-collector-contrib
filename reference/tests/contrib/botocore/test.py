# 3p
import botocore.session
from moto import mock_s3, mock_ec2, mock_lambda, mock_sqs, mock_kinesis, mock_kms

# project
from ddtrace import Pin
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.botocore.patch import patch, unpatch
from ddtrace.compat import stringify

# testing
from tests.opentracer.utils import init_tracer
from ...base import BaseTracerTestCase
from ...utils import assert_span_http_status_code


class BotocoreTest(BaseTracerTestCase):
    """Botocore integration testsuite"""

    TEST_SERVICE = 'test-botocore-tracing'

    def setUp(self):
        patch()

        self.session = botocore.session.get_session()
        self.session.set_credentials(access_key='access-key', secret_key='secret-key')

        super(BotocoreTest, self).setUp()

    def tearDown(self):
        super(BotocoreTest, self).tearDown()

        unpatch()

    @mock_ec2
    def test_traced_client(self):
        ec2 = self.session.create_client('ec2', region_name='us-west-2')
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(ec2)

        ec2.describe_instances()

        spans = self.get_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(span.get_tag('aws.agent'), 'botocore')
        self.assertEqual(span.get_tag('aws.region'), 'us-west-2')
        self.assertEqual(span.get_tag('aws.operation'), 'DescribeInstances')
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.get_metric('retry_attempts'), 0)
        self.assertEqual(span.service, 'test-botocore-tracing.ec2')
        self.assertEqual(span.resource, 'ec2.describeinstances')
        self.assertEqual(span.name, 'ec2.command')
        self.assertEqual(span.span_type, 'http')
        self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    @mock_ec2
    def test_traced_client_analytics(self):
        with self.override_config(
                'botocore',
                dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            ec2 = self.session.create_client('ec2', region_name='us-west-2')
            Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(ec2)
            ec2.describe_instances()

        spans = self.get_spans()
        assert spans
        span = spans[0]
        self.assertEqual(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY), 0.5)

    @mock_s3
    def test_s3_client(self):
        s3 = self.session.create_client('s3', region_name='us-west-2')
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(s3)

        s3.list_buckets()
        s3.list_buckets()

        spans = self.get_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 2)
        self.assertEqual(span.get_tag('aws.operation'), 'ListBuckets')
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.service, 'test-botocore-tracing.s3')
        self.assertEqual(span.resource, 's3.listbuckets')

        # testing for span error
        self.reset()
        try:
            s3.list_objects(bucket='mybucket')
        except Exception:
            spans = self.get_spans()
            assert spans
            span = spans[0]
            self.assertEqual(span.error, 1)
            self.assertEqual(span.resource, 's3.listobjects')

    @mock_s3
    def test_s3_put(self):
        params = dict(Key='foo', Bucket='mybucket', Body=b'bar')
        s3 = self.session.create_client('s3', region_name='us-west-2')
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(s3)
        s3.create_bucket(Bucket='mybucket')
        s3.put_object(**params)

        spans = self.get_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 2)
        self.assertEqual(span.get_tag('aws.operation'), 'CreateBucket')
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.service, 'test-botocore-tracing.s3')
        self.assertEqual(span.resource, 's3.createbucket')
        self.assertEqual(spans[1].get_tag('aws.operation'), 'PutObject')
        self.assertEqual(spans[1].resource, 's3.putobject')
        self.assertEqual(spans[1].get_tag('params.Key'), stringify(params['Key']))
        self.assertEqual(spans[1].get_tag('params.Bucket'), stringify(params['Bucket']))
        # confirm blacklisted
        self.assertIsNone(spans[1].get_tag('params.Body'))

    @mock_sqs
    def test_sqs_client(self):
        sqs = self.session.create_client('sqs', region_name='us-east-1')
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(sqs)

        sqs.list_queues()

        spans = self.get_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(span.get_tag('aws.region'), 'us-east-1')
        self.assertEqual(span.get_tag('aws.operation'), 'ListQueues')
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.service, 'test-botocore-tracing.sqs')
        self.assertEqual(span.resource, 'sqs.listqueues')

    @mock_kinesis
    def test_kinesis_client(self):
        kinesis = self.session.create_client('kinesis', region_name='us-east-1')
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(kinesis)

        kinesis.list_streams()

        spans = self.get_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(span.get_tag('aws.region'), 'us-east-1')
        self.assertEqual(span.get_tag('aws.operation'), 'ListStreams')
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.service, 'test-botocore-tracing.kinesis')
        self.assertEqual(span.resource, 'kinesis.liststreams')

    @mock_kinesis
    def test_unpatch(self):
        kinesis = self.session.create_client('kinesis', region_name='us-east-1')
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(kinesis)

        unpatch()

        kinesis.list_streams()
        spans = self.get_spans()
        assert not spans, spans

    @mock_sqs
    def test_double_patch(self):
        sqs = self.session.create_client('sqs', region_name='us-east-1')
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(sqs)

        patch()
        patch()

        sqs.list_queues()

        spans = self.get_spans()
        assert spans
        self.assertEqual(len(spans), 1)

    @mock_lambda
    def test_lambda_client(self):
        lamb = self.session.create_client('lambda', region_name='us-east-1')
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(lamb)

        lamb.list_functions()

        spans = self.get_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(span.get_tag('aws.region'), 'us-east-1')
        self.assertEqual(span.get_tag('aws.operation'), 'ListFunctions')
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.service, 'test-botocore-tracing.lambda')
        self.assertEqual(span.resource, 'lambda.listfunctions')

    @mock_kms
    def test_kms_client(self):
        kms = self.session.create_client('kms', region_name='us-east-1')
        Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(kms)

        kms.list_keys(Limit=21)

        spans = self.get_spans()
        assert spans
        span = spans[0]
        self.assertEqual(len(spans), 1)
        self.assertEqual(span.get_tag('aws.region'), 'us-east-1')
        self.assertEqual(span.get_tag('aws.operation'), 'ListKeys')
        assert_span_http_status_code(span, 200)
        self.assertEqual(span.service, 'test-botocore-tracing.kms')
        self.assertEqual(span.resource, 'kms.listkeys')

        # checking for protection on sts against security leak
        self.assertIsNone(span.get_tag('params'))

    @mock_ec2
    def test_traced_client_ot(self):
        """OpenTracing version of test_traced_client."""
        ot_tracer = init_tracer('ec2_svc', self.tracer)

        with ot_tracer.start_active_span('ec2_op'):
            ec2 = self.session.create_client('ec2', region_name='us-west-2')
            Pin(service=self.TEST_SERVICE, tracer=self.tracer).onto(ec2)
            ec2.describe_instances()

        spans = self.get_spans()
        assert spans
        self.assertEqual(len(spans), 2)

        ot_span, dd_span = spans

        # confirm the parenting
        self.assertIsNone(ot_span.parent_id)
        self.assertEqual(dd_span.parent_id, ot_span.span_id)

        self.assertEqual(ot_span.name, 'ec2_op')
        self.assertEqual(ot_span.service, 'ec2_svc')

        self.assertEqual(dd_span.get_tag('aws.agent'), 'botocore')
        self.assertEqual(dd_span.get_tag('aws.region'), 'us-west-2')
        self.assertEqual(dd_span.get_tag('aws.operation'), 'DescribeInstances')
        assert_span_http_status_code(dd_span, 200)
        self.assertEqual(dd_span.get_metric('retry_attempts'), 0)
        self.assertEqual(dd_span.service, 'test-botocore-tracing.ec2')
        self.assertEqual(dd_span.resource, 'ec2.describeinstances')
        self.assertEqual(dd_span.name, 'ec2.command')
