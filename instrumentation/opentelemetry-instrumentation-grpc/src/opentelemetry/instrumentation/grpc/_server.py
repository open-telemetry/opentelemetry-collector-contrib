# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# pylint:disable=relative-beyond-top-level
# pylint:disable=arguments-differ
# pylint:disable=no-member
# pylint:disable=signature-differs

"""
Implementation of the service-side open-telemetry interceptor.
"""

import logging
from contextlib import contextmanager

import grpc

from opentelemetry import trace
from opentelemetry.context import attach, detach
from opentelemetry.propagate import extract
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace.status import Status, StatusCode

logger = logging.getLogger(__name__)


# wrap an RPC call
# see https://github.com/grpc/grpc/issues/18191
def _wrap_rpc_behavior(handler, continuation):
    if handler is None:
        return None

    if handler.request_streaming and handler.response_streaming:
        behavior_fn = handler.stream_stream
        handler_factory = grpc.stream_stream_rpc_method_handler
    elif handler.request_streaming and not handler.response_streaming:
        behavior_fn = handler.stream_unary
        handler_factory = grpc.stream_unary_rpc_method_handler
    elif not handler.request_streaming and handler.response_streaming:
        behavior_fn = handler.unary_stream
        handler_factory = grpc.unary_stream_rpc_method_handler
    else:
        behavior_fn = handler.unary_unary
        handler_factory = grpc.unary_unary_rpc_method_handler

    return handler_factory(
        continuation(
            behavior_fn, handler.request_streaming, handler.response_streaming
        ),
        request_deserializer=handler.request_deserializer,
        response_serializer=handler.response_serializer,
    )


# pylint:disable=abstract-method
class _OpenTelemetryServicerContext(grpc.ServicerContext):
    def __init__(self, servicer_context, active_span):
        self._servicer_context = servicer_context
        self._active_span = active_span
        self.code = grpc.StatusCode.OK
        self.details = None
        super().__init__()

    def __getattr__(self, attr):
        return getattr(self._servicer_context, attr)

    def is_active(self, *args, **kwargs):
        return self._servicer_context.is_active(*args, **kwargs)

    def time_remaining(self, *args, **kwargs):
        return self._servicer_context.time_remaining(*args, **kwargs)

    def cancel(self, *args, **kwargs):
        return self._servicer_context.cancel(*args, **kwargs)

    def add_callback(self, *args, **kwargs):
        return self._servicer_context.add_callback(*args, **kwargs)

    def disable_next_message_compression(self):
        return self._service_context.disable_next_message_compression()

    def invocation_metadata(self, *args, **kwargs):
        return self._servicer_context.invocation_metadata(*args, **kwargs)

    def peer(self):
        return self._servicer_context.peer()

    def peer_identities(self):
        return self._servicer_context.peer_identities()

    def peer_identity_key(self):
        return self._servicer_context.peer_identity_key()

    def auth_context(self):
        return self._servicer_context.auth_context()

    def set_compression(self, compression):
        return self._servicer_context.set_compression(compression)

    def send_initial_metadata(self, *args, **kwargs):
        return self._servicer_context.send_initial_metadata(*args, **kwargs)

    def set_trailing_metadata(self, *args, **kwargs):
        return self._servicer_context.set_trailing_metadata(*args, **kwargs)

    def trailing_metadata(self):
        return self._servicer_context.trailing_metadata()

    def abort(self, code, details):
        self.code = code
        self.details = details
        self._active_span.set_attribute(
            SpanAttributes.RPC_GRPC_STATUS_CODE, code.value[0]
        )
        self._active_span.set_status(
            Status(
                status_code=StatusCode.ERROR,
                description=f"{code}:{details}",
            )
        )
        return self._servicer_context.abort(code, details)

    def abort_with_status(self, status):
        return self._servicer_context.abort_with_status(status)

    def set_code(self, code):
        self.code = code
        # use details if we already have it, otherwise the status description
        details = self.details or code.value[1]
        self._active_span.set_attribute(
            SpanAttributes.RPC_GRPC_STATUS_CODE, code.value[0]
        )
        if code != grpc.StatusCode.OK:
            self._active_span.set_status(
                Status(
                    status_code=StatusCode.ERROR,
                    description=f"{code}:{details}",
                )
            )
        return self._servicer_context.set_code(code)

    def set_details(self, details):
        self.details = details
        if self.code != grpc.StatusCode.OK:
            self._active_span.set_status(
                Status(
                    status_code=StatusCode.ERROR,
                    description=f"{self.code}:{details}",
                )
            )
        return self._servicer_context.set_details(details)


# pylint:disable=abstract-method
# pylint:disable=no-self-use
# pylint:disable=unused-argument
class OpenTelemetryServerInterceptor(grpc.ServerInterceptor):
    """
    A gRPC server interceptor, to add OpenTelemetry.

    Usage::

        tracer = some OpenTelemetry tracer

        interceptors = [
            OpenTelemetryServerInterceptor(tracer),
        ]

        server = grpc.server(
            futures.ThreadPoolExecutor(max_workers=concurrency),
            interceptors = interceptors)

    """

    def __init__(self, tracer):
        self._tracer = tracer

    @contextmanager
    def _set_remote_context(self, servicer_context):
        metadata = servicer_context.invocation_metadata()
        if metadata:
            md_dict = {md.key: md.value for md in metadata}
            ctx = extract(md_dict)
            token = attach(ctx)
            try:
                yield
            finally:
                detach(token)
        else:
            yield

    def _start_span(
        self, handler_call_details, context, set_status_on_exception=False
    ):

        # standard attributes
        attributes = {
            SpanAttributes.RPC_SYSTEM: "grpc",
            SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[0],
        }

        # if we have details about the call, split into service and method
        if handler_call_details.method:
            service, method = handler_call_details.method.lstrip("/").split(
                "/", 1
            )
            attributes.update(
                {
                    SpanAttributes.RPC_METHOD: method,
                    SpanAttributes.RPC_SERVICE: service,
                }
            )

        # add some attributes from the metadata
        metadata = dict(context.invocation_metadata())
        if "user-agent" in metadata:
            attributes["rpc.user_agent"] = metadata["user-agent"]

        # Split up the peer to keep with how other telemetry sources
        # do it.  This looks like:
        # * ipv6:[::1]:57284
        # * ipv4:127.0.0.1:57284
        # * ipv4:10.2.1.1:57284,127.0.0.1:57284
        #
        try:
            ip, port = (
                context.peer().split(",")[0].split(":", 1)[1].rsplit(":", 1)
            )
            attributes.update(
                {
                    SpanAttributes.NET_PEER_IP: ip,
                    SpanAttributes.NET_PEER_PORT: port,
                }
            )

            # other telemetry sources add this, so we will too
            if ip in ("[::1]", "127.0.0.1"):
                attributes[SpanAttributes.NET_PEER_NAME] = "localhost"

        except IndexError:
            logger.warning("Failed to parse peer address '%s'", context.peer())

        return self._tracer.start_as_current_span(
            name=handler_call_details.method,
            kind=trace.SpanKind.SERVER,
            attributes=attributes,
            set_status_on_exception=set_status_on_exception,
        )

    def intercept_service(self, continuation, handler_call_details):
        def telemetry_wrapper(behavior, request_streaming, response_streaming):
            def telemetry_interceptor(request_or_iterator, context):

                # handle streaming responses specially
                if response_streaming:
                    return self._intercept_server_stream(
                        behavior,
                        handler_call_details,
                        request_or_iterator,
                        context,
                    )

                with self._set_remote_context(context):
                    with self._start_span(
                        handler_call_details,
                        context,
                        set_status_on_exception=False,
                    ) as span:
                        # wrap the context
                        context = _OpenTelemetryServicerContext(context, span)

                        # And now we run the actual RPC.
                        try:
                            return behavior(request_or_iterator, context)

                        except Exception as error:
                            # Bare exceptions are likely to be gRPC aborts, which
                            # we handle in our context wrapper.
                            # Here, we're interested in uncaught exceptions.
                            # pylint:disable=unidiomatic-typecheck
                            if type(error) != Exception:
                                span.record_exception(error)
                            raise error

            return telemetry_interceptor

        return _wrap_rpc_behavior(
            continuation(handler_call_details), telemetry_wrapper
        )

    # Handle streaming responses separately - we have to do this
    # to return a *new* generator or various upstream things
    # get confused, or we'll lose the consistent trace
    def _intercept_server_stream(
        self, behavior, handler_call_details, request_or_iterator, context
    ):

        with self._set_remote_context(context):
            with self._start_span(
                handler_call_details, context, set_status_on_exception=False
            ) as span:
                context = _OpenTelemetryServicerContext(context, span)

                try:
                    yield from behavior(request_or_iterator, context)

                except Exception as error:
                    # pylint:disable=unidiomatic-typecheck
                    if type(error) != Exception:
                        span.record_exception(error)
                    raise error
