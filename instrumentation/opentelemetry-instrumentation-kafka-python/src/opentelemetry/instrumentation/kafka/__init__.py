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

"""
Instrument `kafka-python` to report instrumentation-kafka produced and consumed messages

Usage
-----

..code:: python

    from opentelemetry.instrumentation.kafka import KafkaInstrumentor
    from kafka import KafkaProducer, KafkaConsumer

    # Instrument kafka
    KafkaInstrumentor().instrument()

    # report a span of type producer with the default settings
    producer = KafkaProducer(bootstrap_servers=['localhost:9092'])
    producer.send('my-topic', b'raw_bytes')


    # report a span of type consumer with the default settings
    consumer = KafkaConsumer('my-topic',
                             group_id='my-group',
                             bootstrap_servers=['localhost:9092'])
    for message in consumer:
        # process message

The `_instrument` method accepts the following keyword args:
tracer_provider (TracerProvider) - an optional tracer provider
produce_hook (Callable) - a function with extra user-defined logic to be performed before sending the message
                          this function signature is:
                          def produce_hook(span: Span, args, kwargs)
consume_hook (Callable) - a function with extra user-defined logic to be performed after consuming a message
                          this function signature is:
                          def consume
                          _hook(span: Span, record: kafka.record.ABCRecord, args, kwargs)
for example:
.. code: python
    from opentelemetry.instrumentation.kafka import KafkaInstrumentor
    from kafka import KafkaProducer, KafkaConsumer

    def produce_hook(span, args, kwargs):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_produce_hook", "some-value")
    def consume_hook(span, record, args, kwargs):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_consume_hook", "some-value")

    # instrument kafka with produce and consume hooks
    KafkaInstrumentor().instrument(produce_hook=produce_hook, consume_hook=consume_hook)

    # Using kafka as normal now will automatically generate spans,
    # including user custom attributes added from the hooks
    producer = KafkaProducer(bootstrap_servers=['localhost:9092'])
    producer.send('my-topic', b'raw_bytes')

API
___
"""
from typing import Collection

import kafka
from wrapt import wrap_function_wrapper

from opentelemetry import trace
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.kafka.package import _instruments
from opentelemetry.instrumentation.kafka.utils import _wrap_next, _wrap_send
from opentelemetry.instrumentation.kafka.version import __version__
from opentelemetry.instrumentation.utils import unwrap


class KafkaInstrumentor(BaseInstrumentor):
    """An instrumentor for kafka module
    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Instruments the kafka module

        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global.
                ``produce_hook``: a callable to be executed just before producing a message
                ``consume_hook``: a callable to be executed just after consuming a message
        """
        tracer_provider = kwargs.get("tracer_provider")
        produce_hook = kwargs.get("produce_hook")
        consume_hook = kwargs.get("consume_hook")

        tracer = trace.get_tracer(
            __name__, __version__, tracer_provider=tracer_provider
        )

        wrap_function_wrapper(
            kafka.KafkaProducer, "send", _wrap_send(tracer, produce_hook)
        )
        wrap_function_wrapper(
            kafka.KafkaConsumer,
            "__next__",
            _wrap_next(tracer, consume_hook),
        )

    def _uninstrument(self, **kwargs):
        unwrap(kafka.KafkaProducer, "send")
        unwrap(kafka.KafkaConsumer, "__next__")
