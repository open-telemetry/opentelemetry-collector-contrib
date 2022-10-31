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
try:
    from unittest import IsolatedAsyncioTestCase
except ImportError:
    # unittest.IsolatedAsyncioTestCase was introduced in Python 3.8. It's use
    # simplifies the following tests. Without it, the amount of test code
    # increases significantly, with most of the additional code handling
    # the asyncio set up.
    from unittest import TestCase

    class IsolatedAsyncioTestCase(TestCase):
        def run(self, result=None):
            self.skipTest(
                "This test requires Python 3.8 for unittest.IsolatedAsyncioTestCase"
            )


import grpc
import grpc.aio
import pytest

from opentelemetry import trace
from opentelemetry.instrumentation.grpc import (
    GrpcAioInstrumentorServer,
    aio_server_interceptor,
    filters,
)
from opentelemetry.test.test_base import TestBase

from .protobuf.test_server_pb2 import Request
from .protobuf.test_server_pb2_grpc import add_GRPCTestServerServicer_to_server
from .test_aio_server_interceptor import Servicer

# pylint:disable=unused-argument
# pylint:disable=no-self-use


async def run_with_test_server(
    runnable, filter_=None, servicer=Servicer(), add_interceptor=True
):
    if add_interceptor:
        interceptors = [aio_server_interceptor(filter_=filter_)]
        server = grpc.aio.server(interceptors=interceptors)
    else:
        server = grpc.aio.server()

    add_GRPCTestServerServicer_to_server(servicer, server)

    port = server.add_insecure_port("[::]:0")
    channel = grpc.aio.insecure_channel(f"localhost:{port:d}")

    await server.start()
    resp = await runnable(channel)
    await server.stop(1000)

    return resp


@pytest.mark.asyncio
class TestOpenTelemetryAioServerInterceptor(TestBase, IsolatedAsyncioTestCase):
    async def test_instrumentor(self):
        """Check that automatic instrumentation configures the interceptor"""
        rpc_call = "/GRPCTestServer/SimpleMethod"

        grpc_aio_server_instrumentor = GrpcAioInstrumentorServer(
            filter_=filters.method_name("NotSimpleMethod")
        )
        try:
            grpc_aio_server_instrumentor.instrument()

            async def request(channel):
                request = Request(client_id=1, request_data="test")
                msg = request.SerializeToString()
                return await channel.unary_unary(rpc_call)(msg)

            await run_with_test_server(request, add_interceptor=False)

            spans_list = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(spans_list), 0)

        finally:
            grpc_aio_server_instrumentor.uninstrument()

    async def test_create_span(self):
        """
        Check that the interceptor wraps calls with spans server-side when filter
        passed and RPC matches the filter.
        """
        rpc_call = "/GRPCTestServer/SimpleMethod"

        async def request(channel):
            request = Request(client_id=1, request_data="test")
            msg = request.SerializeToString()
            return await channel.unary_unary(rpc_call)(msg)

        await run_with_test_server(
            request,
            filter_=filters.method_name("SimpleMethod"),
        )

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertEqual(span.name, rpc_call)
        self.assertIs(span.kind, trace.SpanKind.SERVER)

    async def test_create_span_filtered(self):
        """Check that the interceptor wraps calls with spans server-side."""
        rpc_call = "/GRPCTestServer/SimpleMethod"

        async def request(channel):
            request = Request(client_id=1, request_data="test")
            msg = request.SerializeToString()
            return await channel.unary_unary(rpc_call)(msg)

        await run_with_test_server(
            request,
            filter_=filters.method_name("NotSimpleMethod"),
        )

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 0)
