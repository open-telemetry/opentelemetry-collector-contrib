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
from opentelemetry.test.test_base import TestBase


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


class TestOpenTelemetryServerInterceptor(TestBase):
    def test_instrumentor(self):
        def handler(request, context):
            return b""

        grpc_server_instrumentor = GrpcInstrumentorServer()
        grpc_server_instrumentor.instrument()
        server = grpc.server(
            futures.ThreadPoolExecutor(max_workers=1),
            options=(("grpc.so_reuseport", 0),),
        )

        server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

        port = server.add_insecure_port("[::]:0")
        channel = grpc.insecure_channel("localhost:{:d}".format(port))

        try:
            server.start()
            channel.unary_unary("test")(b"test")
        finally:
            server.stop(None)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]
        self.assertEqual(span.name, "test")
        self.assertIs(span.kind, trace.SpanKind.SERVER)
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.grpc
        )
        grpc_server_instrumentor.uninstrument()

    def test_uninstrument(self):
        def handler(request, context):
            return b""

        grpc_server_instrumentor = GrpcInstrumentorServer()
        grpc_server_instrumentor.instrument()
        grpc_server_instrumentor.uninstrument()
        server = grpc.server(
            futures.ThreadPoolExecutor(max_workers=1),
            options=(("grpc.so_reuseport", 0),),
        )

        server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

        port = server.add_insecure_port("[::]:0")
        channel = grpc.insecure_channel("localhost:{:d}".format(port))

        try:
            server.start()
            channel.unary_unary("test")(b"test")
        finally:
            server.stop(None)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 0)

    def test_create_span(self):
        """Check that the interceptor wraps calls with spans server-side."""

        # Intercept gRPC calls...
        interceptor = server_interceptor()

        # No-op RPC handler
        def handler(request, context):
            return b""

        server = grpc.server(
            futures.ThreadPoolExecutor(max_workers=1),
            options=(("grpc.so_reuseport", 0),),
            interceptors=[interceptor],
        )

        server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

        port = server.add_insecure_port("[::]:0")
        channel = grpc.insecure_channel("localhost:{:d}".format(port))

        try:
            server.start()
            channel.unary_unary("")(b"")
        finally:
            server.stop(None)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertEqual(span.name, "")
        self.assertIs(span.kind, trace.SpanKind.SERVER)

        # Check version and name in span's instrumentation info
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.grpc
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

        server = grpc.server(
            futures.ThreadPoolExecutor(max_workers=1),
            options=(("grpc.so_reuseport", 0),),
            interceptors=[interceptor],
        )
        server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

        port = server.add_insecure_port("[::]:0")
        channel = grpc.insecure_channel("localhost:{:d}".format(port))

        active_span_before_call = trace.get_current_span()
        try:
            server.start()
            channel.unary_unary("")(b"")
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

        server = grpc.server(
            futures.ThreadPoolExecutor(max_workers=1),
            options=(("grpc.so_reuseport", 0),),
            interceptors=[interceptor],
        )
        server.add_generic_rpc_handlers((UnaryUnaryRpcHandler(handler),))

        port = server.add_insecure_port("[::]:0")
        channel = grpc.insecure_channel("localhost:{:d}".format(port))

        try:
            server.start()
            channel.unary_unary("")(b"")
            channel.unary_unary("")(b"")
        finally:
            server.stop(None)

        self.assertEqual(len(active_spans_in_handler), 2)
        # pylint:disable=unbalanced-tuple-unpacking
        span1, span2 = active_spans_in_handler
        # Spans should belong to separate traces, and each should be a root
        # span
        self.assertNotEqual(span1.context.span_id, span2.context.span_id)
        self.assertNotEqual(span1.context.trace_id, span2.context.trace_id)
        self.assertIsNone(span1.parent)
        self.assertIsNone(span1.parent)

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

        server = grpc.server(
            futures.ThreadPoolExecutor(max_workers=2),
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
                f1 = tpe.submit(channel.unary_unary(""), b"")
                f2 = tpe.submit(channel.unary_unary(""), b"")
            futures.wait((f1, f2))
        finally:
            server.stop(None)

        self.assertEqual(len(active_spans_in_handler), 2)
        # pylint:disable=unbalanced-tuple-unpacking
        span1, span2 = active_spans_in_handler
        # Spans should belong to separate traces, and each should be a root
        # span
        self.assertNotEqual(span1.context.span_id, span2.context.span_id)
        self.assertNotEqual(span1.context.trace_id, span2.context.trace_id)
        self.assertIsNone(span1.parent)
        self.assertIsNone(span1.parent)


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
