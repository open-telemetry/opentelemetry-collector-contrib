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

import opentelemetry.instrumentation.grpc
from opentelemetry import trace
from opentelemetry.instrumentation.grpc import GrpcInstrumentorClient
from opentelemetry.sdk.metrics.export.aggregate import (
    MinMaxSumCountAggregator,
    SumAggregator,
)
from opentelemetry.test.test_base import TestBase
from tests.protobuf import test_server_pb2_grpc

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
        GrpcInstrumentorClient().instrument(
            exporter=self.memory_metrics_exporter
        )
        self.server = create_test_server(25565)
        self.server.start()
        self.channel = grpc.insecure_channel("localhost:25565")
        self._stub = test_server_pb2_grpc.GRPCTestServerStub(self.channel)

    def tearDown(self):
        super().tearDown()
        GrpcInstrumentorClient().uninstrument()
        self.memory_metrics_exporter.clear()
        self.server.stop(None)
        self.channel.close()

    def _verify_success_records(self, num_bytes_out, num_bytes_in, method):
        # pylint: disable=protected-access,no-member
        self.channel._interceptor.controller.tick()
        records = self.memory_metrics_exporter.get_exported_metrics()
        self.assertEqual(len(records), 3)

        bytes_out = None
        bytes_in = None
        duration = None

        for record in records:
            if record.instrument.name == "grpcio/client/duration":
                duration = record
            elif record.instrument.name == "grpcio/client/bytes_out":
                bytes_out = record
            elif record.instrument.name == "grpcio/client/bytes_in":
                bytes_in = record

        self.assertIsNotNone(bytes_out)
        self.assertEqual(bytes_out.instrument.name, "grpcio/client/bytes_out")
        self.assertEqual(bytes_out.labels, (("method", method),))

        self.assertIsNotNone(bytes_in)
        self.assertEqual(bytes_in.instrument.name, "grpcio/client/bytes_in")
        self.assertEqual(bytes_in.labels, (("method", method),))

        self.assertIsNotNone(duration)
        self.assertEqual(duration.instrument.name, "grpcio/client/duration")
        self.assertEqual(
            duration.labels,
            (
                ("error", False),
                ("method", method),
                ("status_code", grpc.StatusCode.OK),
            ),
        )

        self.assertEqual(type(bytes_out.aggregator), SumAggregator)
        self.assertEqual(type(bytes_in.aggregator), SumAggregator)
        self.assertEqual(type(duration.aggregator), MinMaxSumCountAggregator)

        self.assertEqual(bytes_out.aggregator.checkpoint, num_bytes_out)
        self.assertEqual(bytes_in.aggregator.checkpoint, num_bytes_in)

        self.assertEqual(duration.aggregator.checkpoint.count, 1)
        self.assertGreaterEqual(duration.aggregator.checkpoint.sum, 0)

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

        self._verify_success_records(8, 8, "/GRPCTestServer/SimpleMethod")

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

        self._verify_success_records(
            8, 40, "/GRPCTestServer/ServerStreamingMethod"
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

        self._verify_success_records(
            40, 8, "/GRPCTestServer/ClientStreamingMethod"
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

        self._verify_success_records(
            40, 40, "/GRPCTestServer/BidirectionalStreamingMethod"
        )

    def _verify_error_records(self, method):
        # pylint: disable=protected-access,no-member
        self.channel._interceptor.controller.tick()
        records = self.memory_metrics_exporter.get_exported_metrics()
        self.assertEqual(len(records), 3)

        bytes_out = None
        errors = None
        duration = None

        for record in records:
            if record.instrument.name == "grpcio/client/duration":
                duration = record
            elif record.instrument.name == "grpcio/client/bytes_out":
                bytes_out = record
            elif record.instrument.name == "grpcio/client/errors":
                errors = record

        self.assertIsNotNone(bytes_out)
        self.assertIsNotNone(errors)
        self.assertIsNotNone(duration)

        self.assertEqual(errors.instrument.name, "grpcio/client/errors")
        self.assertEqual(
            errors.labels,
            (
                ("method", method),
                ("status_code", grpc.StatusCode.INVALID_ARGUMENT),
            ),
        )
        self.assertEqual(errors.aggregator.checkpoint, 1)

        self.assertEqual(
            duration.labels,
            (
                ("error", True),
                ("method", method),
                ("status_code", grpc.StatusCode.INVALID_ARGUMENT),
            ),
        )

    def test_error_simple(self):
        with self.assertRaises(grpc.RpcError):
            simple_method(self._stub, error=True)

        self._verify_error_records("/GRPCTestServer/SimpleMethod")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
        )

    def test_error_stream_unary(self):
        with self.assertRaises(grpc.RpcError):
            client_streaming_method(self._stub, error=True)

        self._verify_error_records("/GRPCTestServer/ClientStreamingMethod")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
        )

    def test_error_unary_stream(self):
        with self.assertRaises(grpc.RpcError):
            server_streaming_method(self._stub, error=True)

        self._verify_error_records("/GRPCTestServer/ServerStreamingMethod")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
        )

    def test_error_stream_stream(self):
        with self.assertRaises(grpc.RpcError):
            bidirectional_streaming_method(self._stub, error=True)

        self._verify_error_records(
            "/GRPCTestServer/BidirectionalStreamingMethod"
        )

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self.assertIs(
            span.status.status_code, trace.status.StatusCode.ERROR,
        )


class TestClientNoMetrics(TestBase):
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
        self.memory_metrics_exporter.clear()
        self.server.stop(None)

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
