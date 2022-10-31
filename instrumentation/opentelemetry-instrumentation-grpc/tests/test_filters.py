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

import collections

import grpc
import pytest

from opentelemetry.instrumentation.grpc import filters


class _HandlerCallDetails(
    collections.namedtuple(
        "_HanlderCallDetails",
        (
            "method",
            "invocation_metadata",
        ),
    ),
    grpc.HandlerCallDetails,
):
    pass


class _UnaryClientInfo(
    collections.namedtuple("_UnaryClientInfo", ("full_method", "timeout"))
):
    pass


class _StreamClientInfo(
    collections.namedtuple(
        "_StreamClientInfo",
        ("full_method", "is_client_stream", "is_server_stream", "timeout"),
    )
):
    pass


@pytest.mark.parametrize(
    "test_case",
    [
        (
            True,
            "SimpleMethod",
            _HandlerCallDetails(
                method="SimpleMethod",
                invocation_metadata=[("tracer", "foo"), ("caller", "bar")],
            ),
        ),
        (
            True,
            "SimpleMethod",
            grpc.aio.ClientCallDetails(
                method="SimpleMethod",
                timeout=3000,
                metadata=None,
                credentials=None,
                wait_for_ready=None,
            ),
        ),
        (
            True,
            "SimpleMethod",
            grpc.aio.ClientCallDetails(
                method=b"SimpleMethod",
                timeout=3000,
                metadata=None,
                credentials=None,
                wait_for_ready=None,
            ),
        ),
        (
            False,
            "SimpleMethod",
            grpc.aio.ClientCallDetails(
                method="NotSimpleMethod",
                timeout=3000,
                metadata=None,
                credentials=None,
                wait_for_ready=None,
            ),
        ),
        (
            False,
            "SimpleMethod",
            _HandlerCallDetails(
                method="NotSimpleMethod",
                invocation_metadata=[("tracer", "foo"), ("caller", "bar")],
            ),
        ),
        (
            True,
            "SimpleMethod",
            _UnaryClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            False,
            "SimpleMethod",
            _UnaryClientInfo(
                full_method="/GRPCTestServer/NotSimpleMethod",
                timeout=3000,
            ),
        ),
        (
            True,
            "SimpleMethod",
            _StreamClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
        (
            False,
            "SimpleMethod",
            _StreamClientInfo(
                full_method="/GRPCTestServer/NotSimpleMethod",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
    ],
)
def test_method_name(test_case):
    fn = filters.method_name(test_case[1])
    assert test_case[0] == fn(test_case[2])


@pytest.mark.parametrize(
    "test_case",
    [
        (
            True,
            "Simple",
            _HandlerCallDetails(
                method="SimpleMethod",
                invocation_metadata=[("tracer", "foo"), ("caller", "bar")],
            ),
        ),
        (
            False,
            "Simple",
            _HandlerCallDetails(
                method="NotSimpleMethod",
                invocation_metadata=[("tracer", "foo"), ("caller", "bar")],
            ),
        ),
        (
            True,
            "Simple",
            _UnaryClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            False,
            "Simple",
            _UnaryClientInfo(
                full_method="/GRPCTestServer/NotSimpleMethod",
                timeout=3000,
            ),
        ),
        (
            True,
            "Simple",
            _StreamClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
        (
            False,
            "Simple",
            _StreamClientInfo(
                full_method="/GRPCTestServer/NotSimpleMethod",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
    ],
)
def test_method_prefix(test_case):
    fn = filters.method_prefix(test_case[1])
    assert test_case[0] == fn(test_case[2])


@pytest.mark.parametrize(
    "test_case",
    [
        (
            True,
            "GRPCTestServer",
            _UnaryClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            False,
            "GRPCTestServer",
            _UnaryClientInfo(
                full_method="/GRPCRealServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            True,
            "GRPCTestServer",
            _StreamClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
        (
            False,
            "GRPCTestServer",
            _StreamClientInfo(
                full_method="/GRPCRealServer/SimpleMethod",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
    ],
)
def test_service_name(test_case):
    fn = filters.service_name(test_case[1])
    assert test_case[0] == fn(test_case[2])


@pytest.mark.parametrize(
    "test_case",
    [
        (
            True,
            "GRPCTest",
            _UnaryClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            False,
            "GRPCTest",
            _UnaryClientInfo(
                full_method="/GRPCRealServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            True,
            "GRPCTest",
            _StreamClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
        (
            False,
            "GRPCTest",
            _StreamClientInfo(
                full_method="/GRPCRealServer/SimpleMethod",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
    ],
)
def test_service_prefix(test_case):
    fn = filters.service_prefix(test_case[1])
    assert test_case[0] == fn(test_case[2])


@pytest.mark.parametrize(
    "test_case",
    [
        (
            True,
            _UnaryClientInfo(
                full_method="/grpc.health.v1.Health/Check",
                timeout=3000,
            ),
        ),
        (
            False,
            _UnaryClientInfo(
                full_method="/GRPCRealServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            True,
            _StreamClientInfo(
                full_method="/grpc.health.v1.Health/Check",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
        (
            False,
            _StreamClientInfo(
                full_method="/GRPCRealServer/SimpleMethod",
                is_client_stream=True,
                is_server_stream=False,
                timeout=3000,
            ),
        ),
    ],
)
def test_health_check(test_case):
    fn = filters.health_check()
    assert test_case[0] == fn(test_case[1])


@pytest.mark.parametrize(
    "test_case",
    [
        (
            True,
            filters.all_of(
                filters.method_name("SimpleMethod"),
                filters.service_name("GRPCTestServer"),
            ),
            _UnaryClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            True,
            filters.any_of(
                filters.method_name("NotSimpleMethod"),
                filters.service_name("GRPCTestServer"),
            ),
            _UnaryClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            True,
            filters.any_of(
                filters.service_name("GRPCMockServer"),
                filters.service_name("GRPCTestServer"),
            ),
            _UnaryClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                timeout=3000,
            ),
        ),
        (
            False,
            filters.negate(filters.method_name("SimpleMethod")),
            _UnaryClientInfo(
                full_method="/GRPCTestServer/SimpleMethod",
                timeout=3000,
            ),
        ),
    ],
)
def test_all_any_negate(test_case):
    fn = test_case[1]
    assert test_case[0] == fn(test_case[2])
