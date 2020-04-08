import requests

from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

from ddtrace import config

from ...pin import Pin
from ...utils.formats import asbool, get_env
from ...utils.wrappers import unwrap as _u
from .legacy import _distributed_tracing, _distributed_tracing_setter
from .constants import DEFAULT_SERVICE
from .connection import _wrap_send

# requests default settings
config._add('requests', {
    'service_name': get_env('requests', 'service_name', DEFAULT_SERVICE),
    'distributed_tracing': asbool(get_env('requests', 'distributed_tracing', True)),
    'split_by_domain': asbool(get_env('requests', 'split_by_domain', False)),
})


def patch():
    """Activate http calls tracing"""
    if getattr(requests, '__datadog_patch', False):
        return
    setattr(requests, '__datadog_patch', True)

    _w('requests', 'Session.send', _wrap_send)
    Pin(
        service=config.requests['service_name'],
        app='requests',
        _config=config.requests,
    ).onto(requests.Session)

    # [Backward compatibility]: `session.distributed_tracing` should point and
    # update the `Pin` configuration instead. This block adds a property so that
    # old implementations work as expected
    fn = property(_distributed_tracing)
    fn = fn.setter(_distributed_tracing_setter)
    requests.Session.distributed_tracing = fn


def unpatch():
    """Disable traced sessions"""
    if not getattr(requests, '__datadog_patch', False):
        return
    setattr(requests, '__datadog_patch', False)

    _u(requests.Session, 'send')
