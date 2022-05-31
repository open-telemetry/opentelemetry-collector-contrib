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

import remoulade
from remoulade.brokers.local import LocalBroker

from opentelemetry.instrumentation.remoulade import RemouladeInstrumentor
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import SpanKind


@remoulade.actor(max_retries=3)
def actor_div(dividend, divisor):
    return dividend / divisor


class TestRemouladeInstrumentation(TestBase):
    def setUp(self):
        super().setUp()
        RemouladeInstrumentor().instrument()

        broker = LocalBroker()
        remoulade.set_broker(broker)

        broker.declare_actor(actor_div)

    def test_message(self):
        actor_div.send(2, 3)

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 2)

        consumer, producer = spans

        self.assertEqual(consumer.name, "remoulade/process")
        self.assertEqual(consumer.kind, SpanKind.CONSUMER)
        self.assertSpanHasAttributes(
            consumer,
            {
                "remoulade.action": "run",
                "remoulade.actor_name": "actor_div",
            },
        )

        self.assertEqual(producer.name, "remoulade/send")
        self.assertEqual(producer.kind, SpanKind.PRODUCER)
        self.assertSpanHasAttributes(
            producer,
            {
                "remoulade.action": "send",
                "remoulade.actor_name": "actor_div",
            },
        )

        self.assertNotEqual(consumer.parent, producer.context)
        self.assertEqual(consumer.parent.span_id, producer.context.span_id)
        self.assertEqual(consumer.context.trace_id, producer.context.trace_id)

    def test_retries(self):
        try:
            actor_div.send(1, 0)
        except ZeroDivisionError:
            pass

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 8)

        consumer_spans = spans[::2]
        producer_spans = spans[1::2]

        self.assertEqual(consumer_spans[0].name, "remoulade/process(retry-3)")
        self.assertSpanHasAttributes(
            consumer_spans[0], {"remoulade.retry_count": 3}
        )
        self.assertEqual(consumer_spans[1].name, "remoulade/process(retry-2)")
        self.assertSpanHasAttributes(
            consumer_spans[1], {"remoulade.retry_count": 2}
        )
        self.assertEqual(consumer_spans[3].name, "remoulade/process")

        self.assertEqual(producer_spans[0].name, "remoulade/send(retry-3)")
        self.assertSpanHasAttributes(
            producer_spans[0], {"remoulade.retry_count": 3}
        )
        self.assertEqual(producer_spans[1].name, "remoulade/send(retry-2)")
        self.assertSpanHasAttributes(
            producer_spans[1], {"remoulade.retry_count": 2}
        )
        self.assertEqual(producer_spans[3].name, "remoulade/send")
