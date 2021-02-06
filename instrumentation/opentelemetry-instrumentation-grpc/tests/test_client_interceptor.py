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

import grpc
from tests.protobuf import (  # pylint: disable=no-name-in-module
    test_server_pb2_grpc,
)

import opentelemetry.instrumentation.grpc
from opentelemetry import trace
from opentelemetry.instrumentation.grpc import GrpcInstrumentorClient
from opentelemetry.test.test_base import TestBase

from ._client import (
    bidirectional_streaming_method,
    client_streaming_method,
    server_streaming_method,
    simple_method,
)
from ._server import create_test_server


class TestClientProto(TestBase):
    def setUp(self):
        super().setUp()
        GrpcInstrumentorClient().instrument()
        self.server = create_test_server(25565)
        self.server.start()
        self.channel = grpc.insecure_channel("localhost:25565")
        self._stub = test_server_pb2_grpc.GRPCTestServerStub(self.channel)

    def tearDown(self):
        super().tearDown()
        GrpcInstrumentorClient().uninstrument()
        self.server.stop(None)
        self.channel.close()

    def test_unary_unary(self):
        simple_method(self._stub)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]

        self.assertEqual(span.name, "/GRPCTestServer/SimpleMethod")
        self.assertIs(span.kind, trace.SpanKind.CLIENT)

        # Check version and name in span's instrumentation info
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.grpc
        )

        self.assert_span_has_attributes(
            span,
            {
                "rpc.method": "SimpleMethod",
                "rpc.service": "GRPCTestServer",
                "rpc.system": "grpc",
                "rpc.grpc.status_code": grpc.StatusCode.OK.value[0],
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
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.grpc
        )

        self.assert_span_has_attributes(
            span,
            {
                "rpc.method": "ServerStreamingMethod",
                "rpc.service": "GRPCTestServer",
                "rpc.system": "grpc",
                "rpc.grpc.status_code": grpc.StatusCode.OK.value[0],
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
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.grpc
        )

        self.assert_span_has_attributes(
            span,
            {
                "rpc.method": "ClientStreamingMethod",
                "rpc.service": "GRPCTestServer",
                "rpc.system": "grpc",
                "rpc.grpc.status_code": grpc.StatusCode.OK.value[0],
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
        self.check_span_instrumentation_info(
            span, opentelemetry.instrumentation.grpc
        )

        self.assert_span_has_attributes(
            span,
            {
                "rpc.method": "BidirectionalStreamingMethod",
                "rpc.service": "GRPCTestServer",
                "rpc.system": "grpc",
                "rpc.grpc.status_code": grpc.StatusCode.OK.value[0],
            },
        )

    def test_error_simple(self):
        with self.assertRaises(grpc.RpcError):
            simple_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
        )

    def test_error_stream_unary(self):
        with self.assertRaises(grpc.RpcError):
            client_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
        )

    def test_error_unary_stream(self):
        with self.assertRaises(grpc.RpcError):
            server_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
        )

    def test_error_stream_stream(self):
        with self.assertRaises(grpc.RpcError):
            bidirectional_streaming_method(self._stub, error=True)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
        )
