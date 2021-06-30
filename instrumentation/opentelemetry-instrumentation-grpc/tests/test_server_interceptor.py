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

import threading
from concurrent import futures

import grpc

import opentelemetry.instrumentation.grpc
from opentelemetry import trace
from opentelemetry.instrumentation.grpc import (
    GrpcInstrumentorServer,
    server_interceptor,
)
from opentelemetry.sdk import trace as trace_sdk
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import StatusCode

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
            server_id=request.client_id, response_data=request.request_data,
        )

    # pylint:disable=C0103
    def ServerStreamingMethod(self, request, context):
        for data in ("one", "two", "three"):
            yield Response(
                server_id=request.client_id, response_data=data,
            )


class TestOpenTelemetryServerInterceptor(TestBase):
    def test_instrumentor(self):
        def handler(request, context):
            return b""

        grpc_server_instrumentor = GrpcInstrumentorServer()
        grpc_server_instrumentor.instrument()
        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor, options=(("grpc.so_reuseport", 0),),
            )

            server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

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
            self.check_span_instrumentation_info(
                span, opentelemetry.instrumentation.grpc
            )

            # Check attributes
            self.assert_span_has_attributes(
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

        grpc_server_instrumentor = GrpcInstrumentorServer()
        grpc_server_instrumentor.instrument()
        grpc_server_instrumentor.uninstrument()
        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor, options=(("grpc.so_reuseport", 0),),
            )

            server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

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
        interceptor = server_interceptor()

        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
                interceptors=[interceptor],
            )
            add_GRPCTestServerServicer_to_server(Servicer(), server)
            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

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
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.grpc
        )

        # Check attributes
        self.assert_span_has_attributes(
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

    def test_create_two_spans(self):
        """Verify that the interceptor captures sub spans within the given
        trace"""

        class TwoSpanServicer(GRPCTestServerServicer):
            # pylint:disable=C0103
            def SimpleMethod(self, request, context):

                # create another span
                tracer = trace.get_tracer(__name__)
                with tracer.start_as_current_span("child") as child:
                    child.add_event("child event")

                return Response(
                    server_id=request.client_id,
                    response_data=request.request_data,
                )

        # Intercept gRPC calls...
        interceptor = server_interceptor()

        # setup the server
        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
                interceptors=[interceptor],
            )
            add_GRPCTestServerServicer_to_server(TwoSpanServicer(), server)
            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

            # setup the RPC
            rpc_call = "/GRPCTestServer/SimpleMethod"
            request = Request(client_id=1, request_data="test")
            msg = request.SerializeToString()
            try:
                server.start()
                channel.unary_unary(rpc_call)(msg)
            finally:
                server.stop(None)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 2)
        child_span = spans_list[0]
        parent_span = spans_list[1]

        self.assertEqual(parent_span.name, rpc_call)
        self.assertIs(parent_span.kind, trace.SpanKind.SERVER)

        # Check version and name in span's instrumentation info
        self.check_span_instrumentation_info(
            parent_span, opentelemetry.instrumentation.grpc
        )

        # Check attributes
        self.assert_span_has_attributes(
            parent_span,
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

        # Check the child span
        self.assertEqual(child_span.name, "child")
        self.assertEqual(
            parent_span.context.trace_id, child_span.context.trace_id
        )

    def test_create_span_streaming(self):
        """Check that the interceptor wraps calls with spans server-side, on a
        streaming call."""

        # Intercept gRPC calls...
        interceptor = server_interceptor()

        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
                interceptors=[interceptor],
            )
            add_GRPCTestServerServicer_to_server(Servicer(), server)
            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

            # setup the RPC
            rpc_call = "/GRPCTestServer/ServerStreamingMethod"
            request = Request(client_id=1, request_data="test")
            msg = request.SerializeToString()
            try:
                server.start()
                list(channel.unary_stream(rpc_call)(msg))
            finally:
                server.stop(None)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertEqual(span.name, rpc_call)
        self.assertIs(span.kind, trace.SpanKind.SERVER)

        # Check version and name in span's instrumentation info
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.grpc
        )

        # Check attributes
        self.assert_span_has_attributes(
            span,
            {
                SpanAttributes.NET_PEER_IP: "[::1]",
                SpanAttributes.NET_PEER_NAME: "localhost",
                SpanAttributes.RPC_METHOD: "ServerStreamingMethod",
                SpanAttributes.RPC_SERVICE: "GRPCTestServer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                    0
                ],
            },
        )

    def test_create_two_spans_streaming(self):
        """Verify that the interceptor captures sub spans in a
        streaming call, within the given trace"""

        class TwoSpanServicer(GRPCTestServerServicer):
            # pylint:disable=C0103
            def ServerStreamingMethod(self, request, context):

                # create another span
                tracer = trace.get_tracer(__name__)
                with tracer.start_as_current_span("child") as child:
                    child.add_event("child event")

                for data in ("one", "two", "three"):
                    yield Response(
                        server_id=request.client_id, response_data=data,
                    )

        # Intercept gRPC calls...
        interceptor = server_interceptor()

        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
                interceptors=[interceptor],
            )
            add_GRPCTestServerServicer_to_server(TwoSpanServicer(), server)
            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

            # setup the RPC
            rpc_call = "/GRPCTestServer/ServerStreamingMethod"
            request = Request(client_id=1, request_data="test")
            msg = request.SerializeToString()
            try:
                server.start()
                list(channel.unary_stream(rpc_call)(msg))
            finally:
                server.stop(None)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 2)
        child_span = spans_list[0]
        parent_span = spans_list[1]

        self.assertEqual(parent_span.name, rpc_call)
        self.assertIs(parent_span.kind, trace.SpanKind.SERVER)

        # Check version and name in span's instrumentation info
        self.check_span_instrumentation_info(
            parent_span, opentelemetry.instrumentation.grpc
        )

        # Check attributes
        self.assert_span_has_attributes(
            parent_span,
            {
                SpanAttributes.NET_PEER_IP: "[::1]",
                SpanAttributes.NET_PEER_NAME: "localhost",
                SpanAttributes.RPC_METHOD: "ServerStreamingMethod",
                SpanAttributes.RPC_SERVICE: "GRPCTestServer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.OK.value[
                    0
                ],
            },
        )

        # Check the child span
        self.assertEqual(child_span.name, "child")
        self.assertEqual(
            parent_span.context.trace_id, child_span.context.trace_id
        )

    def test_span_lifetime(self):
        """Check that the span is active for the duration of the call."""

        interceptor = server_interceptor()

        # To capture the current span at the time the handler is called
        active_span_in_handler = None

        def handler(request, context):
            nonlocal active_span_in_handler
            active_span_in_handler = trace.get_current_span()
            return b""

        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
                interceptors=[interceptor],
            )
            server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

            active_span_before_call = trace.get_current_span()
            try:
                server.start()
                channel.unary_unary("TestServicer/handler")(b"")
            finally:
                server.stop(None)
        active_span_after_call = trace.get_current_span()

        self.assertEqual(active_span_before_call, trace.INVALID_SPAN)
        self.assertEqual(active_span_after_call, trace.INVALID_SPAN)
        self.assertIsInstance(active_span_in_handler, trace_sdk.Span)
        self.assertIsNone(active_span_in_handler.parent)

    def test_sequential_server_spans(self):
        """Check that sequential RPCs get separate server spans."""

        interceptor = server_interceptor()

        # Capture the currently active span in each thread
        active_spans_in_handler = []

        def handler(request, context):
            active_spans_in_handler.append(trace.get_current_span())
            return b""

        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
                interceptors=[interceptor],
            )
            server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

            try:
                server.start()
                channel.unary_unary("TestServicer/handler")(b"")
                channel.unary_unary("TestServicer/handler")(b"")
            finally:
                server.stop(None)

        self.assertEqual(len(active_spans_in_handler), 2)
        # pylint:disable=unbalanced-tuple-unpacking
        span1, span2 = active_spans_in_handler
        # Spans should belong to separate traces
        self.assertNotEqual(span1.context.span_id, span2.context.span_id)
        self.assertNotEqual(span1.context.trace_id, span2.context.trace_id)

        for span in (span1, span2):
            # each should be a root span
            self.assertIsNone(span2.parent)

            # check attributes
            self.assert_span_has_attributes(
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

    def test_concurrent_server_spans(self):
        """Check that concurrent RPC calls don't interfere with each other.

        This is the same check as test_sequential_server_spans except that the
        RPCs are concurrent. Two handlers are invoked at the same time on two
        separate threads. Each one should see a different active span and
        context.
        """

        interceptor = server_interceptor()

        # Capture the currently active span in each thread
        active_spans_in_handler = []
        latch = get_latch(2)

        def handler(request, context):
            latch()
            active_spans_in_handler.append(trace.get_current_span())
            return b""

        with futures.ThreadPoolExecutor(max_workers=2) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
                interceptors=[interceptor],
            )
            server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

            try:
                server.start()
                # Interleave calls so spans are active on each thread at the same
                # time
                with futures.ThreadPoolExecutor(max_workers=2) as tpe:
                    f1 = tpe.submit(
                        channel.unary_unary("TestServicer/handler"), b""
                    )
                    f2 = tpe.submit(
                        channel.unary_unary("TestServicer/handler"), b""
                    )
                futures.wait((f1, f2))
            finally:
                server.stop(None)

        self.assertEqual(len(active_spans_in_handler), 2)
        # pylint:disable=unbalanced-tuple-unpacking
        span1, span2 = active_spans_in_handler
        # Spans should belong to separate traces
        self.assertNotEqual(span1.context.span_id, span2.context.span_id)
        self.assertNotEqual(span1.context.trace_id, span2.context.trace_id)

        for span in (span1, span2):
            # each should be a root span
            self.assertIsNone(span2.parent)

            # check attributes
            self.assert_span_has_attributes(
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

    def test_abort(self):
        """Check that we can catch an abort properly"""

        # Intercept gRPC calls...
        interceptor = server_interceptor()

        # our detailed failure message
        failure_message = "This is a test failure"

        # aborting RPC handler
        def handler(request, context):
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, failure_message)

        with futures.ThreadPoolExecutor(max_workers=1) as executor:
            server = grpc.server(
                executor,
                options=(("grpc.so_reuseport", 0),),
                interceptors=[interceptor],
            )

            server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

            port = server.add_insecure_port("[::]:0")
            channel = grpc.insecure_channel("localhost:{:d}".format(port))

            rpc_call = "TestServicer/handler"

            server.start()
            # unfortunately, these are just bare exceptions in grpc...
            with self.assertRaises(Exception):
                channel.unary_unary(rpc_call)(b"")
            server.stop(None)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertEqual(span.name, rpc_call)
        self.assertIs(span.kind, trace.SpanKind.SERVER)

        # Check version and name in span's instrumentation info
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.grpc
        )

        # make sure this span errored, with the right status and detail
        self.assertEqual(span.status.status_code, StatusCode.ERROR)
        self.assertEqual(
            span.status.description,
            "{}:{}".format(
                grpc.StatusCode.FAILED_PRECONDITION, failure_message
            ),
        )

        # Check attributes
        self.assert_span_has_attributes(
            span,
            {
                SpanAttributes.NET_PEER_IP: "[::1]",
                SpanAttributes.NET_PEER_NAME: "localhost",
                SpanAttributes.RPC_METHOD: "handler",
                SpanAttributes.RPC_SERVICE: "TestServicer",
                SpanAttributes.RPC_SYSTEM: "grpc",
                SpanAttributes.RPC_GRPC_STATUS_CODE: grpc.StatusCode.FAILED_PRECONDITION.value[
                    0
                ],
            },
        )


def get_latch(num):
    """Get a countdown latch function for use in n threads."""
    cv = threading.Condition()
    count = 0

    def countdown_latch():
        """Block until n-1 other threads have called."""
        nonlocal count
        cv.acquire()
        count += 1
        cv.notify()
        cv.release()
        cv.acquire()
        while count < num:
            cv.wait()
        cv.release()

    return countdown_latch
