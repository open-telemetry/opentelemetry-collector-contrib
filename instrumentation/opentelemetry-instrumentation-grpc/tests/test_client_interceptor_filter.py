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

import os
from unittest import mock

import grpc
from tests.protobuf import (  # pylint: disable=no-name-in-module
    test_server_pb2_grpc,
)

import opentelemetry.instrumentation.grpc
from opentelemetry import context, trace
from opentelemetry.instrumentation.grpc import GrpcInstrumentorClient, filters
from opentelemetry.instrumentation.grpc._client import (
    OpenTelemetryClientInterceptor,
)
from opentelemetry.instrumentation.grpc.grpcext._interceptor import (
    _UnaryClientInfo,
)
from opentelemetry.instrumentation.utils import _SUPPRESS_INSTRUMENTATION_KEY
from opentelemetry.propagate import get_global_textmap, set_global_textmap
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.mock_textmap import MockTextMapPropagator
from opentelemetry.test.test_base import TestBase

from ._client import (
    bidirectional_streaming_method,
    client_streaming_method,
    server_streaming_method,
    simple_method,
    simple_method_future,
)
from ._server import create_test_server
from .protobuf.test_server_pb2 import Request


# User defined interceptor. Is used in the tests along with the opentelemetry client interceptor.
class Interceptor(
    grpc.UnaryUnaryClientInterceptor,
    grpc.UnaryStreamClientInterceptor,
    grpc.StreamUnaryClientInterceptor,
    grpc.StreamStreamClientInterceptor,
):
    def __init__(self):
        pass

    def intercept_unary_unary(
        self, continuation, client_call_details, request
    ):
        return self._intercept_call(continuation, client_call_details, request)

    def intercept_unary_stream(
        self, continuation, client_call_details, request
    ):
        return self._intercept_call(continuation, client_call_details, request)

    def intercept_stream_unary(
        self, continuation, client_call_details, request_iterator
    ):
        return self._intercept_call(
            continuation, client_call_details, request_iterator
        )

    def intercept_stream_stream(
        self, continuation, client_call_details, request_iterator
    ):
        return self._intercept_call(
            continuation, client_call_details, request_iterator
        )

    @staticmethod
    def _intercept_call(
        continuation, client_call_details, request_or_iterator
    ):
        return continuation(client_call_details, request_or_iterator)


class TestClientProtoFilterMethodName(TestBase):
    def setUp(self):
        super().setUp()
        GrpcInstrumentorClient(
            filter_=filters.method_name("SimpleMethod")
        ).instrument()
        self.server = create_test_server(25565)
        self.server.start()
        # use a user defined interceptor along with the opentelemetry client interceptor
        interceptors = [Interceptor()]
        self.channel = grpc.insecure_channel("localhost:25565")
        self.channel = grpc.intercept_channel(self.channel, *interceptors)
        self._stub = test_server_pb2_grpc.GRPCTestServerStub(self.channel)

    def tearDown(self):
        super().tearDown()
        GrpcInstrumentorClient().uninstrument()
        self.server.stop(None)
        self.channel.close()

    def test_unary_unary_future(self):
        simple_method_future(self._stub).result()
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(span.name, "/GRPCTestServer/SimpleMethod")
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

    def test_unary_unary(self):
        simple_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(span.name, "/GRPCTestServer/SimpleMethod")
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.RPC_METHOD: "SimpleMethod",
                SpanAttributes.RPC_SERVICE: "GRPCTestServer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                    0
                ],
            },
        )

    def test_unary_stream(self):
        server_streaming_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_stream_unary(self):
        client_streaming_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_stream_stream(self):
        bidirectional_streaming_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_error_simple(self):
        with self.assertRaises(grpc.RpcError):
            simple_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code,
            trace.StatusCode.ERROR,
        )

    def test_error_stream_unary(self):
        with self.assertRaises(grpc.RpcError):
            client_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_error_unary_stream(self):
        with self.assertRaises(grpc.RpcError):
            server_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_error_stream_stream(self):
        with self.assertRaises(grpc.RpcError):
            bidirectional_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_client_interceptor_trace_context_propagation(
        self,
    ):  # pylint: disable=no-self-use
        """ensure that client interceptor correctly inject trace context into all outgoing requests."""
        previous_propagator = get_global_textmap()
        try:
            set_global_textmap(MockTextMapPropagator())
            interceptor = OpenTelemetryClientInterceptor(trace.NoOpTracer())

            carrier = tuple()

            def invoker(request, metadata):
                nonlocal carrier
                carrier = metadata
                return {}

            request = Request(client_id=1, request_data="data")
            interceptor.intercept_unary(
                request,
                {},
                _UnaryClientInfo(
                    full_method="/GRPCTestServer/SimpleMethod", timeout=None
                ),
                invoker=invoker,
            )

            assert len(carrier) == 2
            assert carrier[0][0] == "mock-traceid"
            assert carrier[0][1] == "0"
            assert carrier[1][0] == "mock-spanid"
            assert carrier[1][1] == "0"

        finally:
            set_global_textmap(previous_propagator)


class TestClientProtoFilterMethodPrefix(TestBase):
    def setUp(self):
        super().setUp()
        GrpcInstrumentorClient(
            filter_=filters.method_prefix("Simple")
        ).instrument()
        self.server = create_test_server(25565)
        self.server.start()
        # use a user defined interceptor along with the opentelemetry client interceptor
        interceptors = [Interceptor()]
        self.channel = grpc.insecure_channel("localhost:25565")
        self.channel = grpc.intercept_channel(self.channel, *interceptors)
        self._stub = test_server_pb2_grpc.GRPCTestServerStub(self.channel)

    def tearDown(self):
        super().tearDown()
        GrpcInstrumentorClient().uninstrument()
        self.server.stop(None)
        self.channel.close()

    def test_unary_unary_future(self):
        simple_method_future(self._stub).result()
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(span.name, "/GRPCTestServer/SimpleMethod")
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

    def test_unary_unary(self):
        simple_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(span.name, "/GRPCTestServer/SimpleMethod")
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.RPC_METHOD: "SimpleMethod",
                SpanAttributes.RPC_SERVICE: "GRPCTestServer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                    0
                ],
            },
        )

    def test_unary_stream(self):
        server_streaming_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_stream_unary(self):
        client_streaming_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_stream_stream(self):
        bidirectional_streaming_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_error_simple(self):
        with self.assertRaises(grpc.RpcError):
            simple_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code,
            trace.StatusCode.ERROR,
        )

    def test_error_stream_unary(self):
        with self.assertRaises(grpc.RpcError):
            client_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_error_unary_stream(self):
        with self.assertRaises(grpc.RpcError):
            server_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_error_stream_stream(self):
        with self.assertRaises(grpc.RpcError):
            bidirectional_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_client_interceptor_trace_context_propagation(
        self,
    ):  # pylint: disable=no-self-use
        """ensure that client interceptor correctly inject trace context into all outgoing requests."""
        previous_propagator = get_global_textmap()
        try:
            set_global_textmap(MockTextMapPropagator())
            interceptor = OpenTelemetryClientInterceptor(trace.NoOpTracer())

            carrier = tuple()

            def invoker(request, metadata):
                nonlocal carrier
                carrier = metadata
                return {}

            request = Request(client_id=1, request_data="data")
            interceptor.intercept_unary(
                request,
                {},
                _UnaryClientInfo(
                    full_method="/GRPCTestServer/SimpleMethod", timeout=None
                ),
                invoker=invoker,
            )

            assert len(carrier) == 2
            assert carrier[0][0] == "mock-traceid"
            assert carrier[0][1] == "0"
            assert carrier[1][0] == "mock-spanid"
            assert carrier[1][1] == "0"

        finally:
            set_global_textmap(previous_propagator)


class TestClientProtoFilterByEnv(TestBase):
    def setUp(self):
        with mock.patch.dict(
            os.environ,
            {
                "OTEL_PYTHON_GRPC_EXCLUDED_SERVICES": "GRPCMockServer,GRPCTestServer"
            },
        ):
            super().setUp()
            GrpcInstrumentorClient().instrument()
            self.server = create_test_server(25565)
            self.server.start()
            # use a user defined interceptor along with the opentelemetry client interceptor
            interceptors = [Interceptor()]
            self.channel = grpc.insecure_channel("localhost:25565")
            self.channel = grpc.intercept_channel(self.channel, *interceptors)
            self._stub = test_server_pb2_grpc.GRPCTestServerStub(self.channel)

    def tearDown(self):
        super().tearDown()
        GrpcInstrumentorClient().uninstrument()
        self.server.stop(None)
        self.channel.close()

    def test_unary_unary_future(self):
        simple_method_future(self._stub).result()
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_unary_unary(self):
        simple_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)


class TestClientProtoFilterByEnvAndOption(TestBase):
    def setUp(self):
        with mock.patch.dict(
            os.environ,
            {"OTEL_PYTHON_GRPC_EXCLUDED_SERVICES": "GRPCMockServer"},
        ):
            super().setUp()
            GrpcInstrumentorClient(
                filter_=filters.service_prefix("GRPCTestServer")
            ).instrument()
            self.server = create_test_server(25565)
            self.server.start()
            # use a user defined interceptor along with the opentelemetry client interceptor
            interceptors = [Interceptor()]
            self.channel = grpc.insecure_channel("localhost:25565")
            self.channel = grpc.intercept_channel(self.channel, *interceptors)
            self._stub = test_server_pb2_grpc.GRPCTestServerStub(self.channel)

    def tearDown(self):
        super().tearDown()
        GrpcInstrumentorClient().uninstrument()
        self.server.stop(None)
        self.channel.close()

    def test_unary_unary_future(self):
        simple_method_future(self._stub).result()
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(span.name, "/GRPCTestServer/SimpleMethod")
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

    def test_unary_unary(self):
        simple_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(span.name, "/GRPCTestServer/SimpleMethod")
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.RPC_METHOD: "SimpleMethod",
                SpanAttributes.RPC_SERVICE: "GRPCTestServer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                    0
                ],
            },
        )

    def test_unary_stream(self):
        server_streaming_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(span.name, "/GRPCTestServer/ServerStreamingMethod")
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.RPC_METHOD: "ServerStreamingMethod",
                SpanAttributes.RPC_SERVICE: "GRPCTestServer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                    0
                ],
            },
        )

    def test_stream_unary(self):
        client_streaming_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(span.name, "/GRPCTestServer/ClientStreamingMethod")
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.RPC_METHOD: "ClientStreamingMethod",
                SpanAttributes.RPC_SERVICE: "GRPCTestServer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                    0
                ],
            },
        )

    def test_stream_stream(self):
        bidirectional_streaming_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(
            span.name, "/GRPCTestServer/BidirectionalStreamingMethod"
        )
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.RPC_METHOD: "BidirectionalStreamingMethod",
                SpanAttributes.RPC_SERVICE: "GRPCTestServer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                    0
                ],
            },
        )

    def test_error_simple(self):
        with self.assertRaises(grpc.RpcError):
            simple_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code,
            trace.StatusCode.ERROR,
        )

    def test_error_stream_unary(self):
        with self.assertRaises(grpc.RpcError):
            client_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code,
            trace.StatusCode.ERROR,
        )

    def test_error_unary_stream(self):
        with self.assertRaises(grpc.RpcError):
            server_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code,
            trace.StatusCode.ERROR,
        )

    def test_error_stream_stream(self):
        with self.assertRaises(grpc.RpcError):
            bidirectional_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code,
            trace.StatusCode.ERROR,
        )

    def test_client_interceptor_trace_context_propagation(
        self,
    ):  # pylint: disable=no-self-use
        """ensure that client interceptor correctly inject trace context into all outgoing requests."""
        previous_propagator = get_global_textmap()
        try:
            set_global_textmap(MockTextMapPropagator())
            interceptor = OpenTelemetryClientInterceptor(trace.NoOpTracer())

            carrier = tuple()

            def invoker(request, metadata):
                nonlocal carrier
                carrier = metadata
                return {}

            request = Request(client_id=1, request_data="data")
            interceptor.intercept_unary(
                request,
                {},
                _UnaryClientInfo(
                    full_method="/GRPCTestServer/SimpleMethod", timeout=None
                ),
                invoker=invoker,
            )

            assert len(carrier) == 2
            assert carrier[0][0] == "mock-traceid"
            assert carrier[0][1] == "0"
            assert carrier[1][0] == "mock-spanid"
            assert carrier[1][1] == "0"

        finally:
            set_global_textmap(previous_propagator)

    def test_unary_unary_with_suppress_key(self):
        token = context.attach(
            context.set_value(_SUPPRESS_INSTRUMENTATION_KEY, True)
        )
        try:
            simple_method(self._stub)
            spans = self.memory_exporter.get_finished_spans()
        finally:
            context.detach(token)
        self.assertEqual(len(spans), 0)

    def test_unary_stream_with_suppress_key(self):
        token = context.attach(
            context.set_value(_SUPPRESS_INSTRUMENTATION_KEY, True)
        )
        try:
            server_streaming_method(self._stub)
            spans = self.memory_exporter.get_finished_spans()
        finally:
            context.detach(token)
        self.assertEqual(len(spans), 0)

    def test_stream_unary_with_suppress_key(self):
        token = context.attach(
            context.set_value(_SUPPRESS_INSTRUMENTATION_KEY, True)
        )
        try:
            client_streaming_method(self._stub)
            spans = self.memory_exporter.get_finished_spans()
        finally:
            context.detach(token)
        self.assertEqual(len(spans), 0)

    def test_stream_stream_with_suppress_key(self):
        token = context.attach(
            context.set_value(_SUPPRESS_INSTRUMENTATION_KEY, True)
        )
        try:
            bidirectional_streaming_method(self._stub)
            spans = self.memory_exporter.get_finished_spans()
        finally:
            context.detach(token)
        self.assertEqual(len(spans), 0)
