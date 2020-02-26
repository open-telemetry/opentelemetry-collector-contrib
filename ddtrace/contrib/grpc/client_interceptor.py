import collections
import grpc
from ddtrace.vendor import wrapt

from ddtrace import config
from ddtrace.compat import to_unicode
from ddtrace.ext import SpanTypes, errors
from ...internal.logger import get_logger
from ...propagation.http import HTTPPropagator
from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from . import constants
from .utils import parse_method_path


log = get_logger(__name__)

# DEV: Follows Python interceptors RFC laid out in
# https://github.com/grpc/proposal/blob/master/L13-python-interceptors.md

# DEV: __version__ added in v1.21.4
# https://github.com/grpc/grpc/commit/dd4830eae80143f5b0a9a3a1a024af4cf60e7d02


def create_client_interceptor(pin, host, port):
    return _ClientInterceptor(pin, host, port)


def intercept_channel(wrapped, instance, args, kwargs):
    channel = args[0]
    interceptors = args[1:]
    if isinstance(getattr(channel, "_interceptor", None), _ClientInterceptor):
        dd_interceptor = channel._interceptor
        base_channel = getattr(channel, "_channel", None)
        if base_channel:
            new_channel = wrapped(channel._channel, *interceptors)
            return grpc.intercept_channel(new_channel, dd_interceptor)

    return wrapped(*args, **kwargs)


class _ClientCallDetails(
    collections.namedtuple("_ClientCallDetails", ("method", "timeout", "metadata", "credentials")),
    grpc.ClientCallDetails,
):
    pass


def _future_done_callback(span):
    def func(response):
        try:
            # pull out response code from gRPC response to use both for `grpc.status.code`
            # tag and the error type tag if the response is an exception
            response_code = response.code()
            # cast code to unicode for tags
            status_code = to_unicode(response_code)
            span.set_tag(constants.GRPC_STATUS_CODE_KEY, status_code)

            if response_code != grpc.StatusCode.OK:
                _handle_error(span, response, status_code)
        finally:
            span.finish()

    return func


def _handle_response(span, response):
    if isinstance(response, grpc.Future):
        response.add_done_callback(_future_done_callback(span))


def _handle_error(span, response_error, status_code):
    # response_error should be a grpc.Future and so we expect to have cancelled(),
    # exception() and traceback() methods if a computation has resulted in an
    # exception being raised
    if (
        not callable(getattr(response_error, "cancelled", None))
        and not callable(getattr(response_error, "exception", None))
        and not callable(getattr(response_error, "traceback", None))
    ):
        return

    if response_error.cancelled():
        # handle cancelled futures separately to avoid raising grpc.FutureCancelledError
        span.error = 1
        exc_val = to_unicode(response_error.details())
        span.set_tag(errors.ERROR_MSG, exc_val)
        span.set_tag(errors.ERROR_TYPE, status_code)
        return

    exception = response_error.exception()
    traceback = response_error.traceback()

    if exception is not None and traceback is not None:
        span.error = 1
        if isinstance(exception, grpc.RpcError):
            # handle internal gRPC exceptions separately to get status code and
            # details as tags properly
            exc_val = to_unicode(response_error.details())
            span.set_tag(errors.ERROR_MSG, exc_val)
            span.set_tag(errors.ERROR_TYPE, status_code)
            span.set_tag(errors.ERROR_STACK, traceback)
        else:
            exc_type = type(exception)
            span.set_exc_info(exc_type, exception, traceback)
            status_code = to_unicode(response_error.code())


class _WrappedResponseCallFuture(wrapt.ObjectProxy):
    def __init__(self, wrapped, span):
        super(_WrappedResponseCallFuture, self).__init__(wrapped)
        self._span = span

    def __iter__(self):
        return self

    def __next__(self):
        try:
            return next(self.__wrapped__)
        except StopIteration:
            # at end of iteration handle response status from wrapped future
            _handle_response(self._span, self.__wrapped__)
            raise
        except grpc.RpcError as rpc_error:
            # DEV: grpcio<1.18.0 grpc.RpcError is raised rather than returned as response
            # https://github.com/grpc/grpc/commit/8199aff7a66460fbc4e9a82ade2e95ef076fd8f9
            # handle as a response
            _handle_response(self._span, rpc_error)
            raise
        except Exception:
            # DEV: added for safety though should not be reached since wrapped response
            log.debug("unexpected non-grpc exception raised, closing open span", exc_info=True)
            self._span.set_traceback()
            self._span.finish()
            raise

    def next(self):
        return self.__next__()


class _ClientInterceptor(
    grpc.UnaryUnaryClientInterceptor,
    grpc.UnaryStreamClientInterceptor,
    grpc.StreamUnaryClientInterceptor,
    grpc.StreamStreamClientInterceptor,
):
    def __init__(self, pin, host, port):
        self._pin = pin
        self._host = host
        self._port = port

    def _intercept_client_call(self, method_kind, client_call_details):
        tracer = self._pin.tracer

        span = tracer.trace(
            "grpc", span_type=SpanTypes.GRPC, service=self._pin.service, resource=client_call_details.method,
        )

        # tags for method details
        method_path = client_call_details.method
        method_package, method_service, method_name = parse_method_path(method_path)
        span.set_tag(constants.GRPC_METHOD_PATH_KEY, method_path)
        span.set_tag(constants.GRPC_METHOD_PACKAGE_KEY, method_package)
        span.set_tag(constants.GRPC_METHOD_SERVICE_KEY, method_service)
        span.set_tag(constants.GRPC_METHOD_NAME_KEY, method_name)
        span.set_tag(constants.GRPC_METHOD_KIND_KEY, method_kind)
        span.set_tag(constants.GRPC_HOST_KEY, self._host)
        span.set_tag(constants.GRPC_PORT_KEY, self._port)
        span.set_tag(constants.GRPC_SPAN_KIND_KEY, constants.GRPC_SPAN_KIND_VALUE_CLIENT)

        sample_rate = config.grpc.get_analytics_sample_rate()
        if sample_rate is not None:
            span.set_tag(ANALYTICS_SAMPLE_RATE_KEY, sample_rate)

        # inject tags from pin
        if self._pin.tags:
            span.set_tags(self._pin.tags)

        # propagate distributed tracing headers if available
        headers = {}
        if config.grpc.distributed_tracing_enabled:
            propagator = HTTPPropagator()
            propagator.inject(span.context, headers)

        metadata = []
        if client_call_details.metadata is not None:
            metadata = list(client_call_details.metadata)
        metadata.extend(headers.items())

        client_call_details = _ClientCallDetails(
            client_call_details.method, client_call_details.timeout, metadata, client_call_details.credentials,
        )

        return span, client_call_details

    def intercept_unary_unary(self, continuation, client_call_details, request):
        span, client_call_details = self._intercept_client_call(constants.GRPC_METHOD_KIND_UNARY, client_call_details,)
        try:
            response = continuation(client_call_details, request)
            _handle_response(span, response)
        except grpc.RpcError as rpc_error:
            # DEV: grpcio<1.18.0 grpc.RpcError is raised rather than returned as response
            # https://github.com/grpc/grpc/commit/8199aff7a66460fbc4e9a82ade2e95ef076fd8f9
            # handle as a response
            _handle_response(span, rpc_error)
            raise

        return response

    def intercept_unary_stream(self, continuation, client_call_details, request):
        span, client_call_details = self._intercept_client_call(
            constants.GRPC_METHOD_KIND_SERVER_STREAMING, client_call_details,
        )
        response_iterator = continuation(client_call_details, request)
        response_iterator = _WrappedResponseCallFuture(response_iterator, span)
        return response_iterator

    def intercept_stream_unary(self, continuation, client_call_details, request_iterator):
        span, client_call_details = self._intercept_client_call(
            constants.GRPC_METHOD_KIND_CLIENT_STREAMING, client_call_details,
        )
        try:
            response = continuation(client_call_details, request_iterator)
            _handle_response(span, response)
        except grpc.RpcError as rpc_error:
            # DEV: grpcio<1.18.0 grpc.RpcError is raised rather than returned as response
            # https://github.com/grpc/grpc/commit/8199aff7a66460fbc4e9a82ade2e95ef076fd8f9
            # handle as a response
            _handle_response(span, rpc_error)
            raise

        return response

    def intercept_stream_stream(self, continuation, client_call_details, request_iterator):
        span, client_call_details = self._intercept_client_call(
            constants.GRPC_METHOD_KIND_BIDI_STREAMING, client_call_details,
        )
        response_iterator = continuation(client_call_details, request_iterator)
        response_iterator = _WrappedResponseCallFuture(response_iterator, span)
        return response_iterator
