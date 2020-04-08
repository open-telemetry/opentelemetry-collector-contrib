import aiobotocore

from ddtrace.contrib.aiobotocore.patch import patch, unpatch

from ..utils import aiobotocore_client
from ...asyncio.utils import AsyncioTestCase, mark_asyncio
from ....test_tracer import get_dummy_tracer
from ....utils import assert_span_http_status_code


class AIOBotocoreTest(AsyncioTestCase):
    """Botocore integration testsuite"""
    def setUp(self):
        super(AIOBotocoreTest, self).setUp()
        patch()
        self.tracer = get_dummy_tracer()

    def tearDown(self):
        super(AIOBotocoreTest, self).tearDown()
        unpatch()
        self.tracer = None

    @mark_asyncio
    async def test_response_context_manager(self):
        # the client should call the wrapped __aenter__ and return the
        # object proxy
        with aiobotocore_client('s3', self.tracer) as s3:
            # prepare S3 and flush traces if any
            await s3.create_bucket(Bucket='tracing')
            await s3.put_object(Bucket='tracing', Key='apm', Body=b'')
            self.tracer.writer.pop_traces()
            # `async with` under test
            response = await s3.get_object(Bucket='tracing', Key='apm')
            async with response['Body'] as stream:
                await stream.read()

        traces = self.tracer.writer.pop_traces()

        version = aiobotocore.__version__.split('.')
        pre_08 = int(version[0]) == 0 and int(version[1]) < 8
        # Version 0.8+ generates only one span for reading an object.
        if pre_08:
            assert len(traces) == 2
            assert len(traces[0]) == 1
            assert len(traces[1]) == 1

            span = traces[0][0]
            assert span.get_tag('aws.operation') == 'GetObject'
            assert_span_http_status_code(span, 200)
            assert span.service == 'aws.s3'
            assert span.resource == 's3.getobject'

            read_span = traces[1][0]
            assert read_span.get_tag('aws.operation') == 'GetObject'
            assert_span_http_status_code(read_span, 200)
            assert read_span.service == 'aws.s3'
            assert read_span.resource == 's3.getobject'
            assert read_span.name == 's3.command.read'
            # enforce parenting
            assert read_span.parent_id == span.span_id
            assert read_span.trace_id == span.trace_id
        else:
            assert len(traces[0]) == 1
            assert len(traces[0]) == 1

            span = traces[0][0]
            assert span.get_tag('aws.operation') == 'GetObject'
            assert_span_http_status_code(span, 200)
            assert span.service == 'aws.s3'
            assert span.resource == 's3.getobject'
            assert span.name == 's3.command'
