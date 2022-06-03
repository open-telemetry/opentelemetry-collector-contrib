from logging import getLogger
from typing import List, Optional

from opentelemetry.propagators import textmap
from opentelemetry.semconv.trace import (
    MessagingDestinationKindValues,
    MessagingOperationValues,
    SpanAttributes,
)

_LOG = getLogger(__name__)


class KafkaPropertiesExtractor:
    @staticmethod
    def extract_bootstrap_servers(instance):
        return instance.config.get("bootstrap_servers")

    @staticmethod
    def _extract_argument(key, position, default_value, args, kwargs):
        if len(args) > position:
            return args[position]
        return kwargs.get(key, default_value)

    @staticmethod
    def extract_produce_topic(args):
        """extract topic from `produce` method arguments in Producer class"""
        if len(args) > 0:
            return args[0]
        return "unknown"

    @staticmethod
    def extract_produce_headers(args, kwargs):
        """extract headers from `produce` method arguments in Producer class"""
        return KafkaPropertiesExtractor._extract_argument(
            "headers", 6, None, args, kwargs
        )


class KafkaContextGetter(textmap.Getter):
    def get(self, carrier: textmap.CarrierT, key: str) -> Optional[List[str]]:
        if carrier is None:
            return None
        for item_key, value in carrier:
            if item_key == key:
                if value is not None:
                    return [value.decode()]
        return None

    def keys(self, carrier: textmap.CarrierT) -> List[str]:
        if carrier is None:
            return []
        return [key for (key, value) in carrier]


class KafkaContextSetter(textmap.Setter):
    def set(self, carrier: textmap.CarrierT, key: str, value: str) -> None:
        if carrier is None or key is None:
            return

        if value:
            value = value.encode()
        carrier.append((key, value))


_kafka_getter = KafkaContextGetter()


def _enrich_span(
    span,
    topic,
    partition: Optional[int] = None,
    offset: Optional[int] = None,
    operation: Optional[MessagingOperationValues] = None,
):

    if not span.is_recording():
        return

    span.set_attribute(SpanAttributes.MESSAGING_SYSTEM, "kafka")
    span.set_attribute(SpanAttributes.MESSAGING_DESTINATION, topic)

    if partition:
        span.set_attribute(SpanAttributes.MESSAGING_KAFKA_PARTITION, partition)

    span.set_attribute(
        SpanAttributes.MESSAGING_DESTINATION_KIND,
        MessagingDestinationKindValues.QUEUE.value,
    )

    if operation:
        span.set_attribute(SpanAttributes.MESSAGING_OPERATION, operation.value)
    else:
        span.set_attribute(SpanAttributes.MESSAGING_TEMP_DESTINATION, True)

    # https://stackoverflow.com/questions/65935155/identify-and-find-specific-message-in-kafka-topic
    # A message within Kafka is uniquely defined by its topic name, topic partition and offset.
    if partition and offset and topic:
        span.set_attribute(
            SpanAttributes.MESSAGING_MESSAGE_ID,
            f"{topic}.{partition}.{offset}",
        )


_kafka_setter = KafkaContextSetter()


def _get_span_name(operation: str, topic: str):
    return f"{topic} {operation}"
