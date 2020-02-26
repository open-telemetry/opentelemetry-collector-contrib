from concurrent import futures

from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

from .threading import _wrap_submit
from ...utils.wrappers import unwrap as _u


def patch():
    """Enables Context Propagation between threads"""
    if getattr(futures, '__datadog_patch', False):
        return
    setattr(futures, '__datadog_patch', True)

    _w('concurrent.futures', 'ThreadPoolExecutor.submit', _wrap_submit)


def unpatch():
    """Disables Context Propagation between threads"""
    if not getattr(futures, '__datadog_patch', False):
        return
    setattr(futures, '__datadog_patch', False)

    _u(futures.ThreadPoolExecutor, 'submit')
