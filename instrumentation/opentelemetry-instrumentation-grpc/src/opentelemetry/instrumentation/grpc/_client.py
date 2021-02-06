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

"""Implementation of the invocation-side open-telemetry interceptor."""

from collections import OrderedDict
from typing import MutableMapping

import grpc

from opentelemetry import propagators, trace
from opentelemetry.instrumentation.grpc import grpcext
from opentelemetry.instrumentation.grpc._utilities import RpcInfo
from opentelemetry.trace.status import Status, StatusCode


class _GuardedSpan:
    def __init__(self, span):
        self.span = span
        self.generated_span = None
        self._engaged = True

    def __enter__(self):
        self.generated_span = self.span.__enter__()
        return self

    def __exit__(self, *args, **kwargs):
        if self._engaged:
            self.generated_span = None
            return self.span.__exit__(*args, **kwargs)
        return False

    def release(self):
        self._engaged = False
        return self.span


def _inject_span_context(metadata: MutableMapping[str, str]) -> None:
    # pylint:disable=unused-argument
    def append_metadata(
        carrier: MutableMapping[str, str], key: str, value: str
    ):
        metadata[key] = value

    # Inject current active span from the context
    propagators.inject(append_metadata, metadata)


def _make_future_done_callback(span, rpc_info):
    def callback(response_future):
        with span:
            code = response_future.code()
            if code != grpc.StatusCode.OK:
                rpc_info.error = code
                return
            response = response_future.result()
            rpc_info.response = response

    return callback


class OpenTelemetryClientInterceptor(
    grpcext.UnaryClientInterceptor, grpcext.StreamClientInterceptor
):
    def __init__(self, tracer):
        self._tracer = tracer

    def _start_span(self, method):
        service, meth = method.lstrip("/").split("/", 1)
        attributes = {
            "rpc.system": "grpc",
            "rpc.grpc.status_code": grpc.StatusCode.OK.value[0],
            "rpc.method": meth,
            "rpc.service": service,
        }

        return self._tracer.start_as_current_span(
            name=method, kind=trace.SpanKind.CLIENT, attributes=attributes
        )

    # pylint:disable=no-self-use
    def _trace_result(self, guarded_span, rpc_info, result):
        # If the RPC is called asynchronously, release the guard and add a
        # callback so that the span can be finished once the future is done.
        if isinstance(result, grpc.Future):
            result.add_done_callback(
                _make_future_done_callback(guarded_span.release(), rpc_info)
            )
            return result
        response = result
        # Handle the case when the RPC is initiated via the with_call
        # method and the result is a tuple with the first element as the
        # response.
        # http://www.grpc.io/grpc/python/grpc.html#grpc.UnaryUnaryMultiCallable.with_call
        if isinstance(result, tuple):
            response = result[0]
        rpc_info.response = response

        return result

    def _start_guarded_span(self, *args, **kwargs):
        return _GuardedSpan(self._start_span(*args, **kwargs))

    def intercept_unary(self, request, metadata, client_info, invoker):
        if not metadata:
            mutable_metadata = OrderedDict()
        else:
            mutable_metadata = OrderedDict(metadata)

        with self._start_guarded_span(client_info.full_method) as guarded_span:
            _inject_span_context(mutable_metadata)
            metadata = tuple(mutable_metadata.items())

            rpc_info = RpcInfo(
                full_method=client_info.full_method,
                metadata=metadata,
                timeout=client_info.timeout,
                request=request,
            )

            try:
                result = invoker(request, metadata)
            except grpc.RpcError as err:
                guarded_span.generated_span.set_status(
                    Status(StatusCode.ERROR)
                )
                guarded_span.generated_span.set_attribute(
                    "rpc.grpc.status_code", err.code().value[0]
                )
                raise err

            return self._trace_result(guarded_span, rpc_info, result)

    # For RPCs that stream responses, the result can be a generator. To record
    # the span across the generated responses and detect any errors, we wrap
    # the result in a new generator that yields the response values.
    def _intercept_server_stream(
        self, request_or_iterator, metadata, client_info, invoker
    ):
        if not metadata:
            mutable_metadata = OrderedDict()
        else:
            mutable_metadata = OrderedDict(metadata)

        with self._start_span(client_info.full_method) as span:
            _inject_span_context(mutable_metadata)
            metadata = tuple(mutable_metadata.items())
            rpc_info = RpcInfo(
                full_method=client_info.full_method,
                metadata=metadata,
                timeout=client_info.timeout,
            )

            if client_info.is_client_stream:
                rpc_info.request = request_or_iterator

            try:
                result = invoker(request_or_iterator, metadata)

                for response in result:
                    yield response
            except grpc.RpcError as err:
                span.set_status(Status(StatusCode.ERROR))
                span.set_attribute("rpc.grpc.status_code", err.code().value[0])
                raise err

    def intercept_stream(
        self, request_or_iterator, metadata, client_info, invoker
    ):
        if client_info.is_server_stream:
            return self._intercept_server_stream(
                request_or_iterator, metadata, client_info, invoker
            )

        if not metadata:
            mutable_metadata = OrderedDict()
        else:
            mutable_metadata = OrderedDict(metadata)

        with self._start_guarded_span(client_info.full_method) as guarded_span:
            _inject_span_context(mutable_metadata)
            metadata = tuple(mutable_metadata.items())
            rpc_info = RpcInfo(
                full_method=client_info.full_method,
                metadata=metadata,
                timeout=client_info.timeout,
                request=request_or_iterator,
            )

            rpc_info.request = request_or_iterator

            try:
                result = invoker(request_or_iterator, metadata)
            except grpc.RpcError as err:
                guarded_span.generated_span.set_status(
                    Status(StatusCode.ERROR)
                )
                guarded_span.generated_span.set_attribute(
                    "rpc.grpc.status_code", err.code().value[0],
                )
                raise err

            return self._trace_result(guarded_span, rpc_info, result)
