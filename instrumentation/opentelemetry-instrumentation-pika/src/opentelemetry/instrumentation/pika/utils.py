from logging import getLogger
from typing import Any, Callable, List, Optional

from pika.channel import Channel
from pika.spec import Basic, BasicProperties

from opentelemetry import context, propagate, trace
from opentelemetry.instrumentation.utils import _SUPPRESS_INSTRUMENTATION_KEY
from opentelemetry.propagators.textmap import CarrierT, Getter
from opentelemetry.semconv.trace import (
    MessagingOperationValues,
    SpanAttributes,
)
from opentelemetry.trace import SpanKind, Tracer
from opentelemetry.trace.span import Span

_LOG = getLogger(__name__)


class _PikaGetter(Getter):  # type: ignore
    def get(self, carrier: CarrierT, key: str) -> Optional[List[str]]:
        value = carrier.get(key, None)
        if value is None:
            return None
        return [value]

    def keys(self, carrier: CarrierT) -> List[str]:
        return []


_pika_getter = _PikaGetter()

HookT = Callable[[Span, bytes, BasicProperties], None]


def dummy_callback(span: Span, body: bytes, properties: BasicProperties):
    ...


def _decorate_callback(
    callback: Callable[[Channel, Basic.Deliver, BasicProperties, bytes], Any],
    tracer: Tracer,
    task_name: str,
    consume_hook: HookT = dummy_callback,
):
    def decorated_callback(
        channel: Channel,
        method: Basic.Deliver,
        properties: BasicProperties,
        body: bytes,
    ) -> Any:
        if not properties:
            properties = BasicProperties(headers={})
        if properties.headers is None:
            properties.headers = {}
        ctx = propagate.extract(properties.headers, getter=_pika_getter)
        if not ctx:
            ctx = context.get_current()
        token = context.attach(ctx)
        span = _get_span(
            tracer,
            channel,
            properties,
            destination=method.exchange
            if method.exchange
            else method.routing_key,
            span_kind=SpanKind.CONSUMER,
            task_name=task_name,
            operation=MessagingOperationValues.RECEIVE,
        )
        try:
            with trace.use_span(span, end_on_exit=True):
                try:
                    consume_hook(span, body, properties)
                except Exception as hook_exception:  # pylint: disable=W0703
                    _LOG.exception(hook_exception)
                retval = callback(channel, method, properties, body)
        finally:
            context.detach(token)
        return retval

    return decorated_callback


def _decorate_basic_publish(
    original_function: Callable[[str, str, bytes, BasicProperties, bool], Any],
    channel: Channel,
    tracer: Tracer,
    publish_hook: HookT = dummy_callback,
):
    def decorated_function(
        exchange: str,
        routing_key: str,
        body: bytes,
        properties: BasicProperties = None,
        mandatory: bool = False,
    ) -> Any:
        if not properties:
            properties = BasicProperties(headers={})
        if properties.headers is None:
            properties.headers = {}
        span = _get_span(
            tracer,
            channel,
            properties,
            destination=exchange if exchange else routing_key,
            span_kind=SpanKind.PRODUCER,
            task_name="(temporary)",
            operation=None,
        )
        if not span:
            return original_function(
                exchange, routing_key, body, properties, mandatory
            )
        with trace.use_span(span, end_on_exit=True):
            if span.is_recording():
                propagate.inject(properties.headers)
                try:
                    publish_hook(span, body, properties)
                except Exception as hook_exception:  # pylint: disable=W0703
                    _LOG.exception(hook_exception)
            retval = original_function(
                exchange, routing_key, body, properties, mandatory
            )
        return retval

    return decorated_function


def _get_span(
    tracer: Tracer,
    channel: Channel,
    properties: BasicProperties,
    task_name: str,
    destination: str,
    span_kind: SpanKind,
    operation: Optional[MessagingOperationValues] = None,
) -> Optional[Span]:
    if context.get_value("suppress_instrumentation") or context.get_value(
        _SUPPRESS_INSTRUMENTATION_KEY
    ):
        return None
    task_name = properties.type if properties.type else task_name
    span = tracer.start_span(
        name=_generate_span_name(destination, operation),
        kind=span_kind,
    )
    if span.is_recording():
        _enrich_span(span, channel, properties, task_name, operation)
    return span


def _generate_span_name(
    task_name: str, operation: Optional[MessagingOperationValues]
) -> str:
    if not operation:
        return f"{task_name} send"
    return f"{task_name} {operation.value}"


def _enrich_span(
    span: Span,
    channel: Channel,
    properties: BasicProperties,
    task_destination: str,
    operation: Optional[MessagingOperationValues] = None,
) -> None:
    span.set_attribute(SpanAttributes.MESSAGING_SYSTEM, "rabbitmq")
    if operation:
        span.set_attribute(SpanAttributes.MESSAGING_OPERATION, operation.value)
    else:
        span.set_attribute(SpanAttributes.MESSAGING_TEMP_DESTINATION, True)
    span.set_attribute(SpanAttributes.MESSAGING_DESTINATION, task_destination)
    if properties.message_id:
        span.set_attribute(
            SpanAttributes.MESSAGING_MESSAGE_ID, properties.message_id
        )
    if properties.correlation_id:
        span.set_attribute(
            SpanAttributes.MESSAGING_CONVERSATION_ID, properties.correlation_id
        )
    if not hasattr(channel.connection, "params"):
        span.set_attribute(
            SpanAttributes.NET_PEER_NAME, channel.connection._impl.params.host
        )
        span.set_attribute(
            SpanAttributes.NET_PEER_PORT, channel.connection._impl.params.port
        )
    else:
        span.set_attribute(
            SpanAttributes.NET_PEER_NAME, channel.connection.params.host
        )
        span.set_attribute(
            SpanAttributes.NET_PEER_PORT, channel.connection.params.port
        )
