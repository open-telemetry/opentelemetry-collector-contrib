import botocore.session
from moto import mock_sqs

from opentelemetry.instrumentation.botocore import BotocoreInstrumentor
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase


class TestSqsExtension(TestBase):
    def setUp(self):
        super().setUp()
        BotocoreInstrumentor().instrument()

        session = botocore.session.get_session()
        session.set_credentials(
            access_key="access-key", secret_key="secret-key"
        )
        self.region = "us-west-2"
        self.client = session.create_client("sqs", region_name=self.region)

    def tearDown(self):
        super().tearDown()
        BotocoreInstrumentor().uninstrument()

    @mock_sqs
    def test_sqs_messaging_send_message(self):
        create_queue_result = self.client.create_queue(
            QueueName="test_queue_name"
        )
        queue_url = create_queue_result["QueueUrl"]
        response = self.client.send_message(
            QueueUrl=queue_url, MessageBody="content"
        )

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 2)
        span = spans[1]
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_SYSTEM], "aws.sqs"
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_URL], queue_url
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_DESTINATION],
            "test_queue_name",
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_MESSAGE_ID],
            response["MessageId"],
        )

    @mock_sqs
    def test_sqs_messaging_send_message_batch(self):
        create_queue_result = self.client.create_queue(
            QueueName="test_queue_name"
        )
        queue_url = create_queue_result["QueueUrl"]
        response = self.client.send_message_batch(
            QueueUrl=queue_url,
            Entries=[
                {"Id": "1", "MessageBody": "content"},
                {"Id": "2", "MessageBody": "content2"},
            ],
        )

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 2)
        span = spans[1]
        self.assertEqual(span.attributes["rpc.method"], "SendMessageBatch")
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_SYSTEM], "aws.sqs"
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_URL], queue_url
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_DESTINATION],
            "test_queue_name",
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_MESSAGE_ID],
            response["Successful"][0]["MessageId"],
        )

    @mock_sqs
    def test_sqs_messaging_receive_message(self):
        create_queue_result = self.client.create_queue(
            QueueName="test_queue_name"
        )
        queue_url = create_queue_result["QueueUrl"]
        self.client.send_message(QueueUrl=queue_url, MessageBody="content")
        message_result = self.client.receive_message(
            QueueUrl=create_queue_result["QueueUrl"]
        )

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 3)
        span = spans[-1]
        self.assertEqual(span.attributes["rpc.method"], "ReceiveMessage")
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_SYSTEM], "aws.sqs"
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_URL], queue_url
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_DESTINATION],
            "test_queue_name",
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_MESSAGE_ID],
            message_result["Messages"][0]["MessageId"],
        )

    @mock_sqs
    def test_sqs_messaging_failed_operation(self):
        with self.assertRaises(Exception):
            self.client.send_message(
                QueueUrl="non-existing", MessageBody="content"
            )

        spans = self.memory_exporter.get_finished_spans()
        assert spans
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertEqual(span.attributes["rpc.method"], "SendMessage")
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_SYSTEM], "aws.sqs"
        )
        self.assertEqual(
            span.attributes[SpanAttributes.MESSAGING_URL], "non-existing"
        )
