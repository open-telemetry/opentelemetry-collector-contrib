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
from typing import Callable, TypeVar

import grpc

TCallDetails = TypeVar(
    "TCallDetails",
    grpc.HandlerCallDetails,
    grpc.ClientCallDetails,
    grpc.aio.ClientCallDetails,
)
Condition = Callable[[TCallDetails], bool]


def _full_method(metadata):
    name = ""
    if isinstance(metadata, grpc.HandlerCallDetails):
        name = metadata.method
    elif isinstance(metadata, grpc.aio.ClientCallDetails):
        name = metadata.method
        # name _should_ be a string here but due to a bug in grpc, it is
        # populated with a bytes object. Handle both cases such that we
        # are forward-compatible with a fixed version of grpc
        # More info: https://github.com/grpc/grpc/issues/31092
        if isinstance(name, bytes):
            name = name.decode()
    # NOTE: replace here if there's better way to match cases to handle
    # grpcext._interceptor._UnaryClientInfo/_StreamClientInfo
    elif hasattr(metadata, "full_method"):
        name = metadata.full_method
    # NOTE: this is to handle the grpc.aio Server case. The type interface
    # indicates that metadata should be a grpc.HandlerCallDetails and be
    # matched prior to this but it is in fact an internal C-extension level
    # object.
    elif hasattr(metadata, "method"):
        name = metadata.method

    return name


def _split_full_method(metadata):
    name = _full_method(metadata)
    service, method = os.path.split(name)
    if service != "":
        service = os.path.normpath(service)
        service = service.lstrip("/")
    return (service, method)


def all_of(*args: Condition[TCallDetails]) -> Condition[TCallDetails]:
    """Returns a filter function that returns True if all filter functions
    assigned matches conditions.

    Args:
        args (function): a list of filter function

    Returns:
        A filter function that returns True if all filter functions
    assigned matches conditions.
    """

    def filter_fn(metadata):
        return all(func(metadata) for func in args)

    return filter_fn


def any_of(*args: Condition[TCallDetails]) -> Condition[TCallDetails]:
    """Returns a filter function that returns True if any of filter functions
    assigned matches conditions.

    Args:
        args (function): a list of filter function

    Returns:
        A filter function that returns True if any of filter functions
    assigned matches conditions.
    """

    def filter_fn(metadata):
        return any(func(metadata) for func in args)

    return filter_fn


def negate(func: Condition[TCallDetails]) -> Condition[TCallDetails]:
    """Returns a filter function that negate the result of func

    Args:
        func (function): filter function to negate the result

    Returns:
        A filter function that negate the result of func
    """

    def filter_fn(metadata):
        return not func(metadata)

    return filter_fn


def method_name(name: str) -> Condition[TCallDetails]:
    """Returns a filter function that return True if
    request's gRPC method name matches name.

    Args:
        name (str): method name to match

    Returns:
        A filter function that returns True if request's gRPC method
        name matches name
    """

    def filter_fn(metadata):
        _, method = _split_full_method(metadata)
        return method == name

    return filter_fn


def method_prefix(prefix: str) -> Condition[TCallDetails]:
    """Returns a filter function that return True if
    request's gRPC method name starts with prefix.

    Args:
        prefix (str): method prefix to match

    Returns:
        A filter function that returns True if request's gRPC method
        name starts with prefix
    """

    def filter_fn(metadata):
        _, method = _split_full_method(metadata)
        return method.startswith(prefix)

    return filter_fn


def full_method_name(name: str) -> Condition[TCallDetails]:
    """Returns a filter function that return True if
    request's gRPC full method name matches name.

    Args:
        name (str): full method name to match

    Returns:
        A filter function that returns True if request's gRPC full
        method name matches name
    """

    def filter_fn(metadata):
        fm = _full_method(metadata)
        return fm == name

    return filter_fn


def service_name(name: str) -> Condition[TCallDetails]:
    """Returns a filter function that return True if
    request's gRPC service name matches name.

    Args:
        name (str): service name to match

    Returns:
        A filter function that returns True if request's gRPC service
        name matches name
    """

    def filter_fn(metadata):
        service, _ = _split_full_method(metadata)
        return service == name

    return filter_fn


def service_prefix(prefix: str) -> Condition[TCallDetails]:
    """Returns a filter function that return True if
    request's gRPC service name starts with prefix.

    Args:
        prefix (str): service prefix to match

    Returns:
        A filter function that returns True if request's gRPC service
        name starts with prefix
    """

    def filter_fn(metadata):
        service, _ = _split_full_method(metadata)
        return service.startswith(prefix)

    return filter_fn


def health_check() -> Condition[TCallDetails]:
    """Returns a Filter that returns true if the request's
    service name is health check defined by gRPC Health Checking Protocol.
    https://github.com/grpc/grpc/blob/master/doc/health-checking.md
    """
    return service_prefix("grpc.health.v1.Health")


__all__ = [
    "method_name",
    "method_prefix",
    "full_method_name",
    "service_name",
    "service_prefix",
    "health_check",
]
