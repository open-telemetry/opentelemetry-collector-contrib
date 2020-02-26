# 3p
import kombu
from ddtrace.vendor import wrapt

# project
from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...ext import SpanTypes, kombu as kombux
from ...pin import Pin
from ...propagation.http import HTTPPropagator
from ...settings import config
from ...utils.formats import get_env
from ...utils.wrappers import unwrap

from .constants import DEFAULT_SERVICE
from .utils import (
    get_exchange_from_args,
    get_body_length_from_args,
    get_routing_key_from_args,
    extract_conn_tags,
    HEADER_POS
)

# kombu default settings
config._add('kombu', {
    'service_name': get_env('kombu', 'service_name', DEFAULT_SERVICE)
})

propagator = HTTPPropagator()


def patch():
    """Patch the instrumented methods

    This duplicated doesn't look nice. The nicer alternative is to use an ObjectProxy on top
    of Kombu. However, it means that any "import kombu.Connection" won't be instrumented.
    """
    if getattr(kombu, '_datadog_patch', False):
        return
    setattr(kombu, '_datadog_patch', True)

    _w = wrapt.wrap_function_wrapper
    # We wrap the _publish method because the publish method:
    # *  defines defaults in its kwargs
    # *  potentially overrides kwargs with values from self
    # *  extracts/normalizes things like exchange
    _w('kombu', 'Producer._publish', traced_publish)
    _w('kombu', 'Consumer.receive', traced_receive)
    Pin(
        service=config.kombu['service_name'],
        app='kombu'
    ).onto(kombu.messaging.Producer)

    Pin(
        service=config.kombu['service_name'],
        app='kombu'
    ).onto(kombu.messaging.Consumer)


def unpatch():
    if getattr(kombu, '_datadog_patch', False):
        setattr(kombu, '_datadog_patch', False)
        unwrap(kombu.Producer, '_publish')
        unwrap(kombu.Consumer, 'receive')

#
# tracing functions
#


def traced_receive(func, instance, args, kwargs):
    pin = Pin.get_from(instance)
    if not pin or not pin.enabled():
        return func(*args, **kwargs)

    # Signature only takes 2 args: (body, message)
    message = args[1]
    context = propagator.extract(message.headers)
    # only need to active the new context if something was propagated
    if context.trace_id:
        pin.tracer.context_provider.activate(context)
    with pin.tracer.trace(kombux.RECEIVE_NAME, service=pin.service, span_type=SpanTypes.WORKER) as s:
        # run the command
        exchange = message.delivery_info['exchange']
        s.resource = exchange
        s.set_tag(kombux.EXCHANGE, exchange)

        s.set_tags(extract_conn_tags(message.channel.connection))
        s.set_tag(kombux.ROUTING_KEY, message.delivery_info['routing_key'])
        # set analytics sample rate
        s.set_tag(
            ANALYTICS_SAMPLE_RATE_KEY,
            config.kombu.get_analytics_sample_rate()
        )
        return func(*args, **kwargs)


def traced_publish(func, instance, args, kwargs):
    pin = Pin.get_from(instance)
    if not pin or not pin.enabled():
        return func(*args, **kwargs)

    with pin.tracer.trace(kombux.PUBLISH_NAME, service=pin.service, span_type=SpanTypes.WORKER) as s:
        exchange_name = get_exchange_from_args(args)
        s.resource = exchange_name
        s.set_tag(kombux.EXCHANGE, exchange_name)
        if pin.tags:
            s.set_tags(pin.tags)
        s.set_tag(kombux.ROUTING_KEY, get_routing_key_from_args(args))
        s.set_tags(extract_conn_tags(instance.channel.connection))
        s.set_metric(kombux.BODY_LEN, get_body_length_from_args(args))
        # set analytics sample rate
        s.set_tag(
            ANALYTICS_SAMPLE_RATE_KEY,
            config.kombu.get_analytics_sample_rate()
        )
        # run the command
        propagator.inject(s.context, args[HEADER_POS])
        return func(*args, **kwargs)
