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


import os
from unittest import mock

import grpc
import pytest

from opentelemetry.instrumentation.grpc import (
    GrpcAioInstrumentorClient,
    aio_client_interceptors,
    filters,
)
from opentelemetry.test.test_base import TestBase

from ._aio_client import (
    bidirectional_streaming_method,
    client_streaming_method,
    server_streaming_method,
    simple_method,
)
from ._server import create_test_server
from .protobuf import test_server_pb2_grpc  # pylint: disable=no-name-in-module


@pytest.mark.asyncio
class TestAioClientInterceptorFiltered(TestBase, IsolatedAsyncioTestCase):
    def setUp(self):
        super().setUp()
        self.server = create_test_server(25565)
        self.server.start()

        interceptors = aio_client_interceptors(
            filter_=filters.method_name("NotSimpleMethod")
        )
        self._channel = grpc.aio.insecure_channel(
            "localhost:25565", interceptors=interceptors
        )

        self._stub = test_server_pb2_grpc.GRPCTestServerStub(self._channel)

    def tearDown(self):
        super().tearDown()
        self.server.stop(1000)

    async def asyncTearDown(self):
        await self._channel.close()

    async def test_instrument_filtered(self):
        instrumentor = GrpcAioInstrumentorClient(
            filter_=filters.method_name("NotSimpleMethod")
        )

        try:
            instrumentor.instrument()

            channel = grpc.aio.insecure_channel("localhost:25565")
            stub = test_server_pb2_grpc.GRPCTestServerStub(channel)

            response = await simple_method(stub)
            assert response.response_data == "data"

            spans = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(spans), 0)
        finally:
            instrumentor.uninstrument()

    async def test_instrument_filtered_env(self):
        with mock.patch.dict(
            os.environ,
            {
                "OTEL_PYTHON_GRPC_EXCLUDED_SERVICES": "GRPCMockServer,GRPCTestServer"
            },
        ):
            instrumentor = GrpcAioInstrumentorClient()

        try:
            instrumentor.instrument()

            channel = grpc.aio.insecure_channel("localhost:25565")
            stub = test_server_pb2_grpc.GRPCTestServerStub(channel)

            response = await simple_method(stub)
            assert response.response_data == "data"

            spans = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(spans), 0)
        finally:
            instrumentor.uninstrument()

    async def test_instrument_filtered_env_and_option(self):
        with mock.patch.dict(
            os.environ,
            {"OTEL_PYTHON_GRPC_EXCLUDED_SERVICES": "GRPCMockServer"},
        ):
            instrumentor = GrpcAioInstrumentorClient(
                filter_=filters.service_prefix("GRPCTestServer")
            )

        try:
            instrumentor.instrument()

            channel = grpc.aio.insecure_channel("localhost:25565")
            stub = test_server_pb2_grpc.GRPCTestServerStub(channel)

            response = await simple_method(stub)
            assert response.response_data == "data"

            spans = self.memory_exporter.get_finished_spans()
            self.assertEqual(len(spans), 1)
        finally:
            instrumentor.uninstrument()

    async def test_unary_unary_filtered(self):
        response = await simple_method(self._stub)
        assert response.response_data == "data"

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    async def test_unary_stream_filtered(self):
        async for response in server_streaming_method(self._stub):
            self.assertEqual(response.response_data, "data")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    async def test_stream_unary_filtered(self):
        response = await client_streaming_method(self._stub)
        assert response.response_data == "data"

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    async def test_stream_stream_filtered(self):
        async for response in bidirectional_streaming_method(self._stub):
            self.assertEqual(response.response_data, "data")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)
