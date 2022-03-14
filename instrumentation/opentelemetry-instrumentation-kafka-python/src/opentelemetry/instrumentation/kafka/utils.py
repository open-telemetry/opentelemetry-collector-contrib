import json
from logging import getLogger
from typing import Callable, Dict, List, Optional

from kafka.record.abc import ABCRecord

from opentelemetry import context, propagate, trace
from opentelemetry.propagators import textmap
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import Tracer
from opentelemetry.trace.span import Span

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
    def extract_send_topic(args, kwargs):
        """extract topic from `send` method arguments in KafkaProducer class"""
        return KafkaPropertiesExtractor._extract_argument(
            "topic", 0, "unknown", args, kwargs
        )

    @staticmethod
    def extract_send_value(args, kwargs):
        """extract value from `send` method arguments in KafkaProducer class"""
        return KafkaPropertiesExtractor._extract_argument(
            "value", 1, None, args, kwargs
        )

    @staticmethod
    def extract_send_key(args, kwargs):
        """extract key from `send` method arguments in KafkaProducer class"""
        return KafkaPropertiesExtractor._extract_argument(
            "key", 2, None, args, kwargs
        )

    @staticmethod
    def extract_send_headers(args, kwargs):
        """extract headers from `send` method arguments in KafkaProducer class"""
        return KafkaPropertiesExtractor._extract_argument(
            "headers", 3, None, args, kwargs
        )

    @staticmethod
    def extract_send_partition(instance, args, kwargs):
        """extract partition `send` method arguments, using the `_partition` method in KafkaProducer class"""
        try:
            topic = KafkaPropertiesExtractor.extract_send_topic(args, kwargs)
            key = KafkaPropertiesExtractor.extract_send_key(args, kwargs)
            value = KafkaPropertiesExtractor.extract_send_value(args, kwargs)
            partition = KafkaPropertiesExtractor._extract_argument(
                "partition", 4, None, args, kwargs
            )
            key_bytes = instance._serialize(
                instance.config["key_serializer"], topic, key
            )
            value_bytes = instance._serialize(
                instance.config["value_serializer"], topic, value
            )
            valid_types = (bytes, bytearray, memoryview, type(None))
            if (
                type(key_bytes) not in valid_types
                or type(value_bytes) not in valid_types
            ):
                return None

            all_partitions = instance._metadata.partitions_for_topic(topic)
            if all_partitions is None or len(all_partitions) == 0:
                return None

            return instance._partition(
                topic, partition, key, value, key_bytes, value_bytes
            )
        except Exception as exception:  # pylint: disable=W0703
            _LOG.debug("Unable to extract partition: %s", exception)
            return None


ProduceHookT = Optional[Callable[[Span, List, Dict], None]]
ConsumeHookT = Optional[Callable[[Span, ABCRecord, List, Dict], None]]


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
_kafka_setter = KafkaContextSetter()


def _enrich_span(
    span, bootstrap_servers: List[str], topic: str, partition: int
):
    if span.is_recording():
        span.set_attribute(SpanAttributes.MESSAGING_SYSTEM, "kafka")
        span.set_attribute(SpanAttributes.MESSAGING_DESTINATION, topic)
        span.set_attribute(SpanAttributes.MESSAGING_KAFKA_PARTITION, partition)
        span.set_attribute(
            SpanAttributes.MESSAGING_URL, json.dumps(bootstrap_servers)
        )


def _get_span_name(operation: str, topic: str):
    return f"{topic} {operation}"


def _wrap_send(tracer: Tracer, produce_hook: ProduceHookT) -> Callable:
    def _traced_send(func, instance, args, kwargs):
        headers = KafkaPropertiesExtractor.extract_send_headers(args, kwargs)
        if headers is None:
            headers = []
            kwargs["headers"] = headers

        topic = KafkaPropertiesExtractor.extract_send_topic(args, kwargs)
        bootstrap_servers = KafkaPropertiesExtractor.extract_bootstrap_servers(
            instance
        )
        partition = KafkaPropertiesExtractor.extract_send_partition(
            instance, args, kwargs
        )
        span_name = _get_span_name("send", topic)
        with tracer.start_as_current_span(
            span_name, kind=trace.SpanKind.PRODUCER
        ) as span:
            _enrich_span(span, bootstrap_servers, topic, partition)
            propagate.inject(
                headers,
                context=trace.set_span_in_context(span),
                setter=_kafka_setter,
            )
            try:
                if callable(produce_hook):
                    produce_hook(span, args, kwargs)
            except Exception as hook_exception:  # pylint: disable=W0703
                _LOG.exception(hook_exception)

        return func(*args, **kwargs)

    return _traced_send


def _create_consumer_span(
    tracer,
    consume_hook,
    record,
    extracted_context,
    bootstrap_servers,
    args,
    kwargs,
):
    span_name = _get_span_name("receive", record.topic)
    with tracer.start_as_current_span(
        span_name,
        context=extracted_context,
        kind=trace.SpanKind.CONSUMER,
    ) as span:
        new_context = trace.set_span_in_context(span, extracted_context)
        token = context.attach(new_context)
        _enrich_span(span, bootstrap_servers, record.topic, record.partition)
        try:
            if callable(consume_hook):
                consume_hook(span, record, args, kwargs)
        except Exception as hook_exception:  # pylint: disable=W0703
            _LOG.exception(hook_exception)
        context.detach(token)


def _wrap_next(
    tracer: Tracer,
    consume_hook: ConsumeHookT,
) -> Callable:
    def _traced_next(func, instance, args, kwargs):

        record = func(*args, **kwargs)

        if record:
            bootstrap_servers = (
                KafkaPropertiesExtractor.extract_bootstrap_servers(instance)
            )

            extracted_context = propagate.extract(
                record.headers, getter=_kafka_getter
            )
            _create_consumer_span(
                tracer,
                consume_hook,
                record,
                extracted_context,
                bootstrap_servers,
                args,
                kwargs,
            )
        return record

    return _traced_next
