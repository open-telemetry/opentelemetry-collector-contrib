import grpc
from ddtrace.vendor import wrapt

from ddtrace import config
from ddtrace.ext import errors
from ddtrace.compat import to_unicode

from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...ext import SpanTypes
from ...propagation.http import HTTPPropagator
from . import constants
from .utils import parse_method_path


def create_server_interceptor(pin):
    def interceptor_function(continuation, handler_call_details):
        if not pin.enabled:
            return continuation(handler_call_details)

        rpc_method_handler = continuation(handler_call_details)
        return _TracedRpcMethodHandler(pin, handler_call_details, rpc_method_handler)

    return _ServerInterceptor(interceptor_function)


def _handle_server_exception(server_context, span):
    if server_context is not None and \
       hasattr(server_context, '_state') and \
       server_context._state is not None:
        code = to_unicode(server_context._state.code)
        details = to_unicode(server_context._state.details)
        span.error = 1
        span.set_tag(errors.ERROR_MSG, details)
        span.set_tag(errors.ERROR_TYPE, code)


def _wrap_response_iterator(response_iterator, server_context, span):
    try:
        for response in response_iterator:
            yield response
    except Exception:
        span.set_traceback()
        _handle_server_exception(server_context, span)
        raise
    finally:
        span.finish()


class _TracedRpcMethodHandler(wrapt.ObjectProxy):
    def __init__(self, pin, handler_call_details, wrapped):
        super(_TracedRpcMethodHandler, self).__init__(wrapped)
        self._pin = pin
        self._handler_call_details = handler_call_details

    def _fn(self, method_kind, behavior, args, kwargs):
        if config.grpc_server.distributed_tracing_enabled:
            headers = dict(self._handler_call_details.invocation_metadata)
            propagator = HTTPPropagator()
            context = propagator.extract(headers)

            if context.trace_id:
                self._pin.tracer.context_provider.activate(context)

        tracer = self._pin.tracer

        span = tracer.trace(
            'grpc',
            span_type=SpanTypes.GRPC,
            service=self._pin.service,
            resource=self._handler_call_details.method,
        )

        method_path = self._handler_call_details.method
        method_package, method_service, method_name = parse_method_path(method_path)
        span.set_tag(constants.GRPC_METHOD_PATH_KEY, method_path)
        span.set_tag(constants.GRPC_METHOD_PACKAGE_KEY, method_package)
        span.set_tag(constants.GRPC_METHOD_SERVICE_KEY, method_service)
        span.set_tag(constants.GRPC_METHOD_NAME_KEY, method_name)
        span.set_tag(constants.GRPC_METHOD_KIND_KEY, method_kind)
        span.set_tag(constants.GRPC_SPAN_KIND_KEY, constants.GRPC_SPAN_KIND_VALUE_SERVER)

        sample_rate = config.grpc_server.get_analytics_sample_rate()
        if sample_rate is not None:
            span.set_tag(ANALYTICS_SAMPLE_RATE_KEY, sample_rate)

        # access server context by taking second argument as server context
        # if not found, skip using context to tag span with server state information
        server_context = args[1] if isinstance(args[1], grpc.ServicerContext) else None

        if self._pin.tags:
            span.set_tags(self._pin.tags)

        try:
            response_or_iterator = behavior(*args, **kwargs)

            if self.__wrapped__.response_streaming:
                response_or_iterator = _wrap_response_iterator(response_or_iterator, server_context, span)
        except Exception:
            span.set_traceback()
            _handle_server_exception(server_context, span)
            raise
        finally:
            if not self.__wrapped__.response_streaming:
                span.finish()

        return response_or_iterator

    def unary_unary(self, *args, **kwargs):
        return self._fn(
            constants.GRPC_METHOD_KIND_UNARY,
            self.__wrapped__.unary_unary,
            args,
            kwargs
        )

    def unary_stream(self, *args, **kwargs):
        return self._fn(
            constants.GRPC_METHOD_KIND_SERVER_STREAMING,
            self.__wrapped__.unary_stream,
            args,
            kwargs
        )

    def stream_unary(self, *args, **kwargs):
        return self._fn(
            constants.GRPC_METHOD_KIND_CLIENT_STREAMING,
            self.__wrapped__.stream_unary,
            args,
            kwargs
        )

    def stream_stream(self, *args, **kwargs):
        return self._fn(
            constants.GRPC_METHOD_KIND_BIDI_STREAMING,
            self.__wrapped__.stream_stream,
            args,
            kwargs
        )


class _ServerInterceptor(grpc.ServerInterceptor):
    def __init__(self, interceptor_function):
        self._fn = interceptor_function

    def intercept_service(self, continuation, handler_call_details):
        return self._fn(continuation, handler_call_details)
