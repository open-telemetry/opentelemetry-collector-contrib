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
from unittest import TestCase

from kafka import KafkaConsumer, KafkaProducer
from wrapt import BoundFunctionWrapper

from opentelemetry.instrumentation.kafka import KafkaInstrumentor


class TestKafka(TestCase):
    def test_instrument_api(self) -> None:
        instrumentation = KafkaInstrumentor()

        instrumentation.instrument()
        self.assertTrue(isinstance(KafkaProducer.send, BoundFunctionWrapper))
        self.assertTrue(
            isinstance(KafkaConsumer.__next__, BoundFunctionWrapper)
        )

        instrumentation.uninstrument()
        self.assertFalse(isinstance(KafkaProducer.send, BoundFunctionWrapper))
        self.assertFalse(
            isinstance(KafkaConsumer.__next__, BoundFunctionWrapper)
        )
