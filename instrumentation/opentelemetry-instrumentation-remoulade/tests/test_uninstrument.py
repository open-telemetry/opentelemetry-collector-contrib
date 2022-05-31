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


@remoulade.actor(max_retries=3)
def actor_div(dividend, divisor):
    return dividend / divisor


class TestRemouladeUninstrumentation(TestBase):
    def setUp(self):
        super().setUp()
        RemouladeInstrumentor().instrument()

        broker = LocalBroker()
        remoulade.set_broker(broker)
        broker.declare_actor(actor_div)

        RemouladeInstrumentor().uninstrument()

    def test_uninstrument_existing_broker(self):
        actor_div.send(1, 1)
        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 0)

    def test_uninstrument_new_brokers(self):
        new_broker = LocalBroker()
        remoulade.set_broker(new_broker)
        new_broker.declare_actor(actor_div)

        actor_div.send(1, 1)
        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 0)

    def test_reinstrument_existing_broker(self):
        RemouladeInstrumentor().instrument()

        actor_div.send(1, 1)
        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 2)
