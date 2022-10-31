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

from .protobuf.test_server_pb2 import Request

CLIENT_ID = 1


async def simple_method(stub, error=False):
    request = Request(
        client_id=CLIENT_ID, request_data="error" if error else "data"
    )
    return await stub.SimpleMethod(request)


async def client_streaming_method(stub, error=False):
    # create a generator
    def request_messages():
        for _ in range(5):
            request = Request(
                client_id=CLIENT_ID, request_data="error" if error else "data"
            )
            yield request

    return await stub.ClientStreamingMethod(request_messages())


def server_streaming_method(stub, error=False):
    request = Request(
        client_id=CLIENT_ID, request_data="error" if error else "data"
    )

    return stub.ServerStreamingMethod(request)


def bidirectional_streaming_method(stub, error=False):
    # create a generator
    def request_messages():
        for _ in range(5):
            request = Request(
                client_id=CLIENT_ID, request_data="error" if error else "data"
            )
            yield request

    return stub.BidirectionalStreamingMethod(request_messages())
