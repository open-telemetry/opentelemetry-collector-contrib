import grpc
import os

from ddtrace.vendor.wrapt import wrap_function_wrapper as _w
from ddtrace import config, Pin

from ...utils.wrappers import unwrap as _u

from . import constants
from .client_interceptor import create_client_interceptor, intercept_channel
from .server_interceptor import create_server_interceptor


config._add('grpc_server', dict(
    service_name=os.environ.get('DATADOG_SERVICE_NAME', constants.GRPC_SERVICE_SERVER),
    distributed_tracing_enabled=True,
))

# TODO[tbutt]: keeping name for client config unchanged to maintain backwards
# compatibility but should change in future
config._add('grpc', dict(
    service_name='{}-{}'.format(
        os.environ.get('DATADOG_SERVICE_NAME'), constants.GRPC_SERVICE_CLIENT
    ) if os.environ.get('DATADOG_SERVICE_NAME') else constants.GRPC_SERVICE_CLIENT,
    distributed_tracing_enabled=True,
))


def patch():
    _patch_client()
    _patch_server()


def unpatch():
    _unpatch_client()
    _unpatch_server()


def _patch_client():
    if getattr(constants.GRPC_PIN_MODULE_CLIENT, '__datadog_patch', False):
        return
    setattr(constants.GRPC_PIN_MODULE_CLIENT, '__datadog_patch', True)

    Pin(service=config.grpc.service_name).onto(constants.GRPC_PIN_MODULE_CLIENT)

    _w('grpc', 'insecure_channel', _client_channel_interceptor)
    _w('grpc', 'secure_channel', _client_channel_interceptor)
    _w('grpc', 'intercept_channel', intercept_channel)


def _unpatch_client():
    if not getattr(constants.GRPC_PIN_MODULE_CLIENT, '__datadog_patch', False):
        return
    setattr(constants.GRPC_PIN_MODULE_CLIENT, '__datadog_patch', False)

    pin = Pin.get_from(constants.GRPC_PIN_MODULE_CLIENT)
    if pin:
        pin.remove_from(constants.GRPC_PIN_MODULE_CLIENT)

    _u(grpc, 'secure_channel')
    _u(grpc, 'insecure_channel')


def _patch_server():
    if getattr(constants.GRPC_PIN_MODULE_SERVER, '__datadog_patch', False):
        return
    setattr(constants.GRPC_PIN_MODULE_SERVER, '__datadog_patch', True)

    Pin(service=config.grpc_server.service_name).onto(constants.GRPC_PIN_MODULE_SERVER)

    _w('grpc', 'server', _server_constructor_interceptor)


def _unpatch_server():
    if not getattr(constants.GRPC_PIN_MODULE_SERVER, '__datadog_patch', False):
        return
    setattr(constants.GRPC_PIN_MODULE_SERVER, '__datadog_patch', False)

    pin = Pin.get_from(constants.GRPC_PIN_MODULE_SERVER)
    if pin:
        pin.remove_from(constants.GRPC_PIN_MODULE_SERVER)

    _u(grpc, 'server')


def _client_channel_interceptor(wrapped, instance, args, kwargs):
    channel = wrapped(*args, **kwargs)

    pin = Pin.get_from(constants.GRPC_PIN_MODULE_CLIENT)
    if not pin or not pin.enabled():
        return channel

    (host, port) = _parse_target_from_arguments(args, kwargs)

    interceptor_function = create_client_interceptor(pin, host, port)
    return grpc.intercept_channel(channel, interceptor_function)


def _server_constructor_interceptor(wrapped, instance, args, kwargs):
    # DEV: we clone the pin on the grpc module and configure it for the server
    # interceptor

    pin = Pin.get_from(constants.GRPC_PIN_MODULE_SERVER)
    if not pin or not pin.enabled():
        return wrapped(*args, **kwargs)

    interceptor = create_server_interceptor(pin)

    # DEV: Inject our tracing interceptor first in the list of interceptors
    if 'interceptors' in kwargs:
        kwargs['interceptors'] = (interceptor,) + tuple(kwargs['interceptors'])
    else:
        kwargs['interceptors'] = (interceptor,)

    return wrapped(*args, **kwargs)


def _parse_target_from_arguments(args, kwargs):
    if 'target' in kwargs:
        target = kwargs['target']
    else:
        target = args[0]

    split = target.rsplit(':', 2)

    return (split[0], split[1] if len(split) > 1 else None)
