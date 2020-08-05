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

"""Implementation of the service-side open-telemetry interceptor.

This library borrows heavily from the OpenTracing gRPC integration:
https://github.com/opentracing-contrib/python-grpc
"""

from contextlib import contextmanager
from typing import List

import grpc

from opentelemetry import propagators, trace
from opentelemetry.context import attach, detach

from . import grpcext
from ._utilities import RpcInfo


# pylint:disable=abstract-method
class _OpenTelemetryServicerContext(grpc.ServicerContext):
    def __init__(self, servicer_context, active_span):
        self._servicer_context = servicer_context
        self._active_span = active_span
        self.code = grpc.StatusCode.OK
        self.details = None
        super(_OpenTelemetryServicerContext, self).__init__()

    def is_active(self, *args, **kwargs):
        return self._servicer_context.is_active(*args, **kwargs)

    def time_remaining(self, *args, **kwargs):
        return self._servicer_context.time_remaining(*args, **kwargs)

    def cancel(self, *args, **kwargs):
        return self._servicer_context.cancel(*args, **kwargs)

    def add_callback(self, *args, **kwargs):
        return self._servicer_context.add_callback(*args, **kwargs)

    def invocation_metadata(self, *args, **kwargs):
        return self._servicer_context.invocation_metadata(*args, **kwargs)

    def peer(self, *args, **kwargs):
        return self._servicer_context.peer(*args, **kwargs)

    def peer_identities(self, *args, **kwargs):
        return self._servicer_context.peer_identities(*args, **kwargs)

    def peer_identity_key(self, *args, **kwargs):
        return self._servicer_context.peer_identity_key(*args, **kwargs)

    def auth_context(self, *args, **kwargs):
        return self._servicer_context.auth_context(*args, **kwargs)

    def send_initial_metadata(self, *args, **kwargs):
        return self._servicer_context.send_initial_metadata(*args, **kwargs)

    def set_trailing_metadata(self, *args, **kwargs):
        return self._servicer_context.set_trailing_metadata(*args, **kwargs)

    def abort(self, *args, **kwargs):
        if not hasattr(self._servicer_context, "abort"):
            raise RuntimeError(
                "abort() is not supported with the installed version of grpcio"
            )
        return self._servicer_context.abort(*args, **kwargs)

    def abort_with_status(self, *args, **kwargs):
        if not hasattr(self._servicer_context, "abort_with_status"):
            raise RuntimeError(
                "abort_with_status() is not supported with the installed "
                "version of grpcio"
            )
        return self._servicer_context.abort_with_status(*args, **kwargs)

    def set_code(self, code):
        self.code = code
        return self._servicer_context.set_code(code)

    def set_details(self, details):
        self.details = details
        return self._servicer_context.set_details(details)


# On the service-side, errors can be signaled either by exceptions or by
# calling `set_code` on the `servicer_context`. This function checks for the
# latter and updates the span accordingly.
# pylint:disable=unused-argument
def _check_error_code(span, servicer_context, rpc_info):
    if servicer_context.code != grpc.StatusCode.OK:
        rpc_info.error = servicer_context.code


class OpenTelemetryServerInterceptor(
    grpcext.UnaryServerInterceptor, grpcext.StreamServerInterceptor
):
    def __init__(self, tracer):
        self._tracer = tracer

    @contextmanager
    # pylint:disable=no-self-use
    def _set_remote_context(self, servicer_context):
        metadata = servicer_context.invocation_metadata()
        if metadata:
            md_dict = {md.key: md.value for md in metadata}

            def get_from_grpc_metadata(metadata, key) -> List[str]:
                return [md_dict[key]] if key in md_dict else []

            # Update the context with the traceparent from the RPC metadata.
            ctx = propagators.extract(get_from_grpc_metadata, metadata)
            token = attach(ctx)
            try:
                yield
            finally:
                detach(token)
        else:
            yield

    def _start_span(self, method):
        span = self._tracer.start_as_current_span(
            name=method, kind=trace.SpanKind.SERVER
        )
        return span

    def intercept_unary(self, request, servicer_context, server_info, handler):

        with self._set_remote_context(servicer_context):
            with self._start_span(server_info.full_method) as span:
                rpc_info = RpcInfo(
                    full_method=server_info.full_method,
                    metadata=servicer_context.invocation_metadata(),
                    timeout=servicer_context.time_remaining(),
                    request=request,
                )
                servicer_context = _OpenTelemetryServicerContext(
                    servicer_context, span
                )
                response = handler(request, servicer_context)

                _check_error_code(span, servicer_context, rpc_info)

                rpc_info.response = response

                return response

    # For RPCs that stream responses, the result can be a generator. To record
    # the span across the generated responses and detect any errors, we wrap
    # the result in a new generator that yields the response values.
    def _intercept_server_stream(
        self, request_or_iterator, servicer_context, server_info, handler
    ):
        with self._set_remote_context(servicer_context):
            with self._start_span(server_info.full_method) as span:
                rpc_info = RpcInfo(
                    full_method=server_info.full_method,
                    metadata=servicer_context.invocation_metadata(),
                    timeout=servicer_context.time_remaining(),
                )
                if not server_info.is_client_stream:
                    rpc_info.request = request_or_iterator
                servicer_context = _OpenTelemetryServicerContext(
                    servicer_context, span
                )
                result = handler(request_or_iterator, servicer_context)
                for response in result:
                    yield response
                _check_error_code(span, servicer_context, rpc_info)

    def intercept_stream(
        self, request_or_iterator, servicer_context, server_info, handler
    ):
        if server_info.is_server_stream:
            return self._intercept_server_stream(
                request_or_iterator, servicer_context, server_info, handler
            )
        with self._set_remote_context(servicer_context):
            with self._start_span(server_info.full_method) as span:
                rpc_info = RpcInfo(
                    full_method=server_info.full_method,
                    metadata=servicer_context.invocation_metadata(),
                    timeout=servicer_context.time_remaining(),
                )
                servicer_context = _OpenTelemetryServicerContext(
                    servicer_context, span
                )
                response = handler(request_or_iterator, servicer_context)
                _check_error_code(span, servicer_context, rpc_info)
                rpc_info.response = response
                return response
