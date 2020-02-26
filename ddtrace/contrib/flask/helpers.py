from ddtrace import Pin
import flask


def get_current_app():
    """Helper to get the flask.app.Flask from the current app context"""
    appctx = flask._app_ctx_stack.top
    if appctx:
        return appctx.app
    return None


def with_instance_pin(func):
    """Helper to wrap a function wrapper and ensure an enabled pin is available for the `instance`"""
    def wrapper(wrapped, instance, args, kwargs):
        pin = Pin._find(wrapped, instance, get_current_app())
        if not pin or not pin.enabled():
            return wrapped(*args, **kwargs)

        return func(pin, wrapped, instance, args, kwargs)
    return wrapper


def simple_tracer(name, span_type=None):
    """Generate a simple tracer that wraps the function call with `with tracer.trace()`"""
    @with_instance_pin
    def wrapper(pin, wrapped, instance, args, kwargs):
        with pin.tracer.trace(name, service=pin.service, span_type=span_type):
            return wrapped(*args, **kwargs)
    return wrapper


def get_current_span(pin, root=False):
    """Helper to get the current span from the provided pins current call context"""
    if not pin or not pin.enabled():
        return None

    ctx = pin.tracer.get_call_context()
    if not ctx:
        return None

    if root:
        return ctx.get_current_root_span()
    return ctx.get_current_span()
