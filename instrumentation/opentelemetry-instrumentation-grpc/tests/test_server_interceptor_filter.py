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

# pylint:disable=unused-argument
# pylint:disable=no-self-use

from concurrent import futures

import grpc

import opentelemetry.instrumentation.grpc
from opentelemetry import trace
from opentelemetry.instrumentation.grpc import (
    GrpcInstrumentorServer,
    filters,
    server_interceptor,
)
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase

from .protobuf.test_server_pb2 import Request, Response
from .protobuf.test_server_pb2_grpc import (
    GRPCTestServerServicer,
    add_GRPCTestServerServicer_to_server,
)


class UnaryUnaryMethodHandler(grpc.RpcMethodHandler):
    def __init__(self, handler):
        self.request_streaming = False
        self.response_streaming = False
        self.request_deserializer = None
        self.response_serializer = None
        self.unary_unary = handler
        self.unary_stream = None
        self.stream_unary = None
        self.stream_stream = None


class UnaryUnaryRpcHandler(grpc.GenericRpcHandler):
    def __init__(self, handler):
        self._unary_unary_handler = handler

    def service(self, handler_call_details):
        return UnaryUnaryMethodHandler(self._unary_unary_handler)


class Servicer(GRPCTestServerServicer):
    """Our test servicer"""

    # pylint:disable=C0103
    def SimpleMethod(self, request, context):
        return Response(
            server_id=request.client_id,
            response_data=request.request_data,
        )

    # pylint:disable=C0103
    def ServerStreamingMethod(self, request, context):
        for data in ("one", "two", "three"):
            yield Response(
                server_id=request.client_id,
                response_data=data,
            )


class TestOpenTelemetryServerInterceptorFilterMethodName(TestBase):
    def test_instrumentor(self):
        def handler(request, context):
            return b""

        grpc_server_instrumentor = GrpcInstrumentorServer(
            filter_=filters.method_name("handler")
        )
        grpc_server_instrumentor.instrument()
        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
            )

            server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel(f"localhost:{port:d}")

            rpc_call = "TestServicer/handler"
            try:
                server.start()
                channel.unary_unary(rpc_call)(b"test")
            finally:
                server.stop(None)

            spans_list = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(spans_list), 1)
            span = spans_list[0]
            self.assertEqual(span.name, rpc_call)
            self.assertIs(span.kind, trace.SpanKind.SERVER)

            # Check version and name in span's instrumentation info
            self.assertEqualSpanInstrumentationInfo(
                span, opentelemetry.instrumentation.grpc
            )

            # Check attributes
            self.assertSpanHasAttributes(
                span,
                {
                    SpanAttributes.NET_PEER_IP: "[::1]",
                    SpanAttributes.NET_PEER_NAME: "localhost",
                    SpanAttributes.RPC_METHOD: "handler",
                    SpanAttributes.RPC_SERVICE: "TestServicer",
                    SpanAttributes.RPC_SYSTEM: "grpc",
                    SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                        0
                    ],
                },
            )

            grpc_server_instrumentor.uninstrument()

    def test_uninstrument(self):
        def handler(request, context):
            return b""

        grpc_server_instrumentor = GrpcInstrumentorServer(
            filter_=filters.method_name("SimpleMethod")
        )
        grpc_server_instrumentor.instrument()
        grpc_server_instrumentor.uninstrument()
        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
            )

            server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel(f"localhost:{port:d}")

            rpc_call = "TestServicer/test"
            try:
                server.start()
                channel.unary_unary(rpc_call)(b"test")
            finally:
                server.stop(None)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 0)

    def test_create_span(self):
        """Check that the interceptor wraps calls with spans server-side."""

        # Intercept gRPC calls...
        interceptor = server_interceptor(
            filter_=filters.method_name("SimpleMethod")
        )

        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
                interceptors=[interceptor],
            )
            add_GRPCTestServerServicer_to_server(Servicer(), server)
            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel(f"localhost:{port:d}")

            rpc_call = "/GRPCTestServer/SimpleMethod"
            request = Request(client_id=1, request_data="test")
            msg = request.SerializeToString()
            try:
                server.start()
                channel.unary_unary(rpc_call)(msg)
            finally:
                server.stop(None)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertEqual(span.name, rpc_call)
        self.assertIs(span.kind, trace.SpanKind.SERVER)

        # Check version and name in span's instrumentation info
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.grpc
        )

        # Check attributes
        self.assertSpanHasAttributes(
            span,
            {
                SpanAttributes.NET_PEER_IP: "[::1]",
                SpanAttributes.NET_PEER_NAME: "localhost",
                SpanAttributes.RPC_METHOD: "SimpleMethod",
                SpanAttributes.RPC_SERVICE: "GRPCTestServer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                    0
                ],
            },
        )
