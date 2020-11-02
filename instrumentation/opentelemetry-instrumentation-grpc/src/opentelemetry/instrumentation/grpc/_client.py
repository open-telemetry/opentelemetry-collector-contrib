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

from opentelemetry import metrics, propagators, trace
from opentelemetry.sdk.metrics.export.controller import PushController
from opentelemetry.trace.status import Status, StatusCode

from . import grpcext
from ._utilities import RpcInfo, TimedMetricRecorder


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


def _make_future_done_callback(span, rpc_info, client_info, metrics_recorder):
    def callback(response_future):
        with span:
            code = response_future.code()
            if code != grpc.StatusCode.OK:
                rpc_info.error = code
                return
            response = response_future.result()
            rpc_info.response = response
            if "ByteSize" in dir(response):
                metrics_recorder.record_bytes_in(
                    response.ByteSize(), client_info.full_method
                )

    return callback


class OpenTelemetryClientInterceptor(
    grpcext.UnaryClientInterceptor, grpcext.StreamClientInterceptor
):
    def __init__(self, tracer, exporter, interval):
        self._tracer = tracer

        self._meter = None
        if exporter and interval:
            self._meter = metrics.get_meter(__name__)
            self.controller = PushController(
                meter=self._meter, exporter=exporter, interval=interval
            )
        self._metrics_recorder = TimedMetricRecorder(self._meter, "client")

    def _start_span(self, method):
        return self._tracer.start_as_current_span(
            name=method, kind=trace.SpanKind.CLIENT
        )

    # pylint:disable=no-self-use
    def _trace_result(self, guarded_span, rpc_info, result, client_info):
        # If the RPC is called asynchronously, release the guard and add a
        # callback so that the span can be finished once the future is done.
        if isinstance(result, grpc.Future):
            result.add_done_callback(
                _make_future_done_callback(
                    guarded_span.release(),
                    rpc_info,
                    client_info,
                    self._metrics_recorder,
                )
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

        if "ByteSize" in dir(response):
            self._metrics_recorder.record_bytes_in(
                response.ByteSize(), client_info.full_method
            )
        return result

    def _start_guarded_span(self, *args, **kwargs):
        return _GuardedSpan(self._start_span(*args, **kwargs))

    def _bytes_out_iterator_wrapper(self, iterator, client_info):
        for request in iterator:
            if "ByteSize" in dir(request):
                self._metrics_recorder.record_bytes_out(
                    request.ByteSize(), client_info.full_method
                )
            yield request

    def intercept_unary(self, request, metadata, client_info, invoker):
        if not metadata:
            mutable_metadata = OrderedDict()
        else:
            mutable_metadata = OrderedDict(metadata)

        with self._start_guarded_span(client_info.full_method) as guarded_span:
            with self._metrics_recorder.record_latency(
                client_info.full_method
            ):
                _inject_span_context(mutable_metadata)
                metadata = tuple(mutable_metadata.items())

                # If protobuf is used, we can record the bytes in/out. Otherwise, we have no way
                # to get the size of the request/response properly, so don't record anything
                if "ByteSize" in dir(request):
                    self._metrics_recorder.record_bytes_out(
                        request.ByteSize(), client_info.full_method
                    )

                rpc_info = RpcInfo(
                    full_method=client_info.full_method,
                    metadata=metadata,
                    timeout=client_info.timeout,
                    request=request,
                )

                try:
                    result = invoker(request, metadata)
                except grpc.RpcError:
                    guarded_span.generated_span.set_status(
                        Status(StatusCode.ERROR)
                    )
                    raise

                return self._trace_result(
                    guarded_span, rpc_info, result, client_info
                )

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
            with self._metrics_recorder.record_latency(
                client_info.full_method
            ):
                _inject_span_context(mutable_metadata)
                metadata = tuple(mutable_metadata.items())
                rpc_info = RpcInfo(
                    full_method=client_info.full_method,
                    metadata=metadata,
                    timeout=client_info.timeout,
                )

                if client_info.is_client_stream:
                    rpc_info.request = request_or_iterator
                    request_or_iterator = self._bytes_out_iterator_wrapper(
                        request_or_iterator, client_info
                    )
                else:
                    if "ByteSize" in dir(request_or_iterator):
                        self._metrics_recorder.record_bytes_out(
                            request_or_iterator.ByteSize(),
                            client_info.full_method,
                        )

                try:
                    result = invoker(request_or_iterator, metadata)

                    # Rewrap the result stream into a generator, and record the bytes received
                    for response in result:
                        if "ByteSize" in dir(response):
                            self._metrics_recorder.record_bytes_in(
                                response.ByteSize(), client_info.full_method
                            )
                        yield response
                except grpc.RpcError:
                    span.set_status(Status(StatusCode.ERROR))
                    raise

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
            with self._metrics_recorder.record_latency(
                client_info.full_method
            ):
                _inject_span_context(mutable_metadata)
                metadata = tuple(mutable_metadata.items())
                rpc_info = RpcInfo(
                    full_method=client_info.full_method,
                    metadata=metadata,
                    timeout=client_info.timeout,
                    request=request_or_iterator,
                )

                rpc_info.request = request_or_iterator

                request_or_iterator = self._bytes_out_iterator_wrapper(
                    request_or_iterator, client_info
                )

                try:
                    result = invoker(request_or_iterator, metadata)
                except grpc.RpcError:
                    guarded_span.generated_span.set_status(
                        Status(StatusCode.ERROR)
                    )
                    raise

                return self._trace_result(
                    guarded_span, rpc_info, result, client_info
                )
