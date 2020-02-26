from ddtrace.vendor.wrapt import wrap_function_wrapper as wrap

from rest_framework.views import APIView

from ...utils.wrappers import unwrap


def patch_restframework(tracer):
    """ Patches rest_framework app.

    To trace exceptions occuring during view processing we currently use a TraceExceptionMiddleware.
    However the rest_framework handles exceptions before they come to our middleware.
    So we need to manually patch the rest_framework exception handler
    to set the exception stack trace in the current span.

    """

    def _traced_handle_exception(wrapped, instance, args, kwargs):
        """ Sets the error message, error type and exception stack trace to the current span
            before calling the original exception handler.
        """
        span = tracer.current_span()
        if span is not None:
            span.set_traceback()

        return wrapped(*args, **kwargs)

    # do not patch if already patched
    if getattr(APIView, '_datadog_patch', False):
        return
    else:
        setattr(APIView, '_datadog_patch', True)

    # trace the handle_exception method
    wrap('rest_framework.views', 'APIView.handle_exception', _traced_handle_exception)


def unpatch_restframework():
    """ Unpatches rest_framework app."""
    if getattr(APIView, '_datadog_patch', False):
        setattr(APIView, '_datadog_patch', False)
        unwrap(APIView, 'handle_exception')
