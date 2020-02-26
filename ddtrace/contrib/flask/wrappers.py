from ddtrace.vendor.wrapt import function_wrapper

from ...pin import Pin
from ...utils.importlib import func_name
from .helpers import get_current_app


def wrap_function(instance, func, name=None, resource=None):
    """
    Helper function to wrap common flask.app.Flask methods.

    This helper will first ensure that a Pin is available and enabled before tracing
    """
    if not name:
        name = func_name(func)

    @function_wrapper
    def trace_func(wrapped, _instance, args, kwargs):
        pin = Pin._find(wrapped, _instance, instance, get_current_app())
        if not pin or not pin.enabled():
            return wrapped(*args, **kwargs)
        with pin.tracer.trace(name, service=pin.service, resource=resource):
            return wrapped(*args, **kwargs)

    return trace_func(func)


def wrap_signal(app, signal, func):
    """
    Helper used to wrap signal handlers

    We will attempt to find the pin attached to the flask.app.Flask app
    """
    name = func_name(func)

    @function_wrapper
    def trace_func(wrapped, instance, args, kwargs):
        pin = Pin._find(wrapped, instance, app, get_current_app())
        if not pin or not pin.enabled():
            return wrapped(*args, **kwargs)

        with pin.tracer.trace(name, service=pin.service) as span:
            span.set_tag('flask.signal', signal)
            return wrapped(*args, **kwargs)

    return trace_func(func)
