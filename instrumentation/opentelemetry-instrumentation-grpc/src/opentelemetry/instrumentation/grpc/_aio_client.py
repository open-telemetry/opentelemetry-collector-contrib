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

import functools
from collections import OrderedDict

import grpc
from grpc.aio import ClientCallDetails

from opentelemetry import context
from opentelemetry.instrumentation.grpc._client import (
    OpenTelemetryClientInterceptor,
    _carrier_setter,
)
from opentelemetry.instrumentation.utils import _SUPPRESS_INSTRUMENTATION_KEY
from opentelemetry.propagate import inject
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace.status import Status, StatusCode


def _unary_done_callback(span, code, details):
    def callback(call):
        try:
            span.set_attribute(
                SpanAttributes.RPC_GRPC_STATUS_CODE,
                code.value[0],
            )
            if code != grpc.StatusCode.OK:
                span.set_status(
                    Status(
                        status_code=StatusCode.ERROR,
                        description=details,
                    )
                )
        finally:
            span.end()

    return callback


class _BaseAioClientInterceptor(OpenTelemetryClientInterceptor):
    @staticmethod
    def propagate_trace_in_details(client_call_details):
        metadata = client_call_details.metadata
        if not metadata:
            mutable_metadata = OrderedDict()
        else:
            mutable_metadata = OrderedDict(metadata)

        inject(mutable_metadata, setter=_carrier_setter)
        metadata = tuple(mutable_metadata.items())

        return ClientCallDetails(
            client_call_details.method,
            client_call_details.timeout,
            metadata,
            client_call_details.credentials,
            client_call_details.wait_for_ready,
        )

    @staticmethod
    def add_error_details_to_span(span, exc):
        if isinstance(exc, grpc.RpcError):
            span.set_attribute(
                SpanAttributes.RPC_GRPC_STATUS_CODE,
                exc.code().value[0],
            )
        span.set_status(
            Status(
                status_code=StatusCode.ERROR,
                description=f"{type(exc).__name__}: {exc}",
            )
        )
        span.record_exception(exc)

    def _start_interceptor_span(self, method):
        # method _should_ be a string here but due to a bug in grpc, it is
        # populated with a bytes object. Handle both cases such that we
        # are forward-compatible with a fixed version of grpc
        # More info: https://github.com/grpc/grpc/issues/31092
        if isinstance(method, bytes):
            method = method.decode()

        return self._start_span(
            method,
            end_on_exit=False,
            record_exception=False,
            set_status_on_exception=False,
        )

    async def _wrap_unary_response(self, continuation, span):
        try:
            call = await continuation()

            # code and details are both coroutines that need to be await-ed,
            # the callbacks added with add_done_callback do not allow async
            # code so we need to get the code and details here then pass them
            # to the callback.
            code = await call.code()
            details = await call.details()

            call.add_done_callback(_unary_done_callback(span, code, details))

            return call
        except grpc.aio.AioRpcError as exc:
            self.add_error_details_to_span(span, exc)
            raise exc

    async def _wrap_stream_response(self, span, call):
        try:
            async for response in call:
                yield response
        except Exception as exc:
            self.add_error_details_to_span(span, exc)
            raise exc
        finally:
            span.end()

    def tracing_skipped(self, client_call_details):
        return context.get_value(
            _SUPPRESS_INSTRUMENTATION_KEY
        ) or not self.rpc_matches_filters(client_call_details)

    def rpc_matches_filters(self, client_call_details):
        return self._filter is None or self._filter(client_call_details)


class UnaryUnaryAioClientInterceptor(
    grpc.aio.UnaryUnaryClientInterceptor,
    _BaseAioClientInterceptor,
):
    async def intercept_unary_unary(
        self, continuation, client_call_details, request
    ):
        if self.tracing_skipped(client_call_details):
            return await continuation(client_call_details, request)

        with self._start_interceptor_span(
            client_call_details.method,
        ) as span:
            new_details = self.propagate_trace_in_details(client_call_details)

            continuation_with_args = functools.partial(
                continuation, new_details, request
            )
            return await self._wrap_unary_response(
                continuation_with_args, span
            )


class UnaryStreamAioClientInterceptor(
    grpc.aio.UnaryStreamClientInterceptor,
    _BaseAioClientInterceptor,
):
    async def intercept_unary_stream(
        self, continuation, client_call_details, request
    ):
        if self.tracing_skipped(client_call_details):
            return await continuation(client_call_details, request)

        with self._start_interceptor_span(
            client_call_details.method,
        ) as span:
            new_details = self.propagate_trace_in_details(client_call_details)

            resp = await continuation(new_details, request)

            return self._wrap_stream_response(span, resp)


class StreamUnaryAioClientInterceptor(
    grpc.aio.StreamUnaryClientInterceptor,
    _BaseAioClientInterceptor,
):
    async def intercept_stream_unary(
        self, continuation, client_call_details, request_iterator
    ):
        if self.tracing_skipped(client_call_details):
            return await continuation(client_call_details, request_iterator)

        with self._start_interceptor_span(
            client_call_details.method,
        ) as span:
            new_details = self.propagate_trace_in_details(client_call_details)

            continuation_with_args = functools.partial(
                continuation, new_details, request_iterator
            )
            return await self._wrap_unary_response(
                continuation_with_args, span
            )


class StreamStreamAioClientInterceptor(
    grpc.aio.StreamStreamClientInterceptor,
    _BaseAioClientInterceptor,
):
    async def intercept_stream_stream(
        self, continuation, client_call_details, request_iterator
    ):
        if self.tracing_skipped(client_call_details):
            return await continuation(client_call_details, request_iterator)

        with self._start_interceptor_span(
            client_call_details.method,
        ) as span:
            new_details = self.propagate_trace_in_details(client_call_details)

            resp = await continuation(new_details, request_iterator)

            return self._wrap_stream_response(span, resp)
