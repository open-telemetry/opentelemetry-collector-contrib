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

# pylint:disable=relative-beyond-top-level
# pylint:disable=arguments-differ
# pylint:disable=no-member
# pylint:disable=signature-differs

"""Implementation of gRPC Python interceptors."""


import collections

import grpc

from .. import grpcext


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


class _InterceptorUnaryUnaryMultiCallable(grpc.UnaryUnaryMultiCallable):
    def __init__(self, method, base_callable, interceptor):
        self._method = method
        self._base_callable = base_callable
        self._interceptor = interceptor

    def __call__(self, request, timeout=None, metadata=None, credentials=None):
        def invoker(request, metadata):
            return self._base_callable(request, timeout, metadata, credentials)

        client_info = _UnaryClientInfo(self._method, timeout)
        return self._interceptor.intercept_unary(
            request, metadata, client_info, invoker
        )

    def with_call(
        self, request, timeout=None, metadata=None, credentials=None
    ):
        def invoker(request, metadata):
            return self._base_callable.with_call(
                request, timeout, metadata, credentials
            )

        client_info = _UnaryClientInfo(self._method, timeout)
        return self._interceptor.intercept_unary(
            request, metadata, client_info, invoker
        )

    def future(self, request, timeout=None, metadata=None, credentials=None):
        def invoker(request, metadata):
            return self._base_callable.future(
                request, timeout, metadata, credentials
            )

        client_info = _UnaryClientInfo(self._method, timeout)
        return self._interceptor.intercept_unary(
            request, metadata, client_info, invoker
        )


class _InterceptorUnaryStreamMultiCallable(grpc.UnaryStreamMultiCallable):
    def __init__(self, method, base_callable, interceptor):
        self._method = method
        self._base_callable = base_callable
        self._interceptor = interceptor

    def __call__(self, request, timeout=None, metadata=None, credentials=None):
        def invoker(request, metadata):
            return self._base_callable(request, timeout, metadata, credentials)

        client_info = _StreamClientInfo(self._method, False, True, timeout)
        return self._interceptor.intercept_stream(
            request, metadata, client_info, invoker
        )


class _InterceptorStreamUnaryMultiCallable(grpc.StreamUnaryMultiCallable):
    def __init__(self, method, base_callable, interceptor):
        self._method = method
        self._base_callable = base_callable
        self._interceptor = interceptor

    def __call__(
        self, request_iterator, timeout=None, metadata=None, credentials=None
    ):
        def invoker(request_iterator, metadata):
            return self._base_callable(
                request_iterator, timeout, metadata, credentials
            )

        client_info = _StreamClientInfo(self._method, True, False, timeout)
        return self._interceptor.intercept_stream(
            request_iterator, metadata, client_info, invoker
        )

    def with_call(
        self, request_iterator, timeout=None, metadata=None, credentials=None
    ):
        def invoker(request_iterator, metadata):
            return self._base_callable.with_call(
                request_iterator, timeout, metadata, credentials
            )

        client_info = _StreamClientInfo(self._method, True, False, timeout)
        return self._interceptor.intercept_stream(
            request_iterator, metadata, client_info, invoker
        )

    def future(
        self, request_iterator, timeout=None, metadata=None, credentials=None
    ):
        def invoker(request_iterator, metadata):
            return self._base_callable.future(
                request_iterator, timeout, metadata, credentials
            )

        client_info = _StreamClientInfo(self._method, True, False, timeout)
        return self._interceptor.intercept_stream(
            request_iterator, metadata, client_info, invoker
        )


class _InterceptorStreamStreamMultiCallable(grpc.StreamStreamMultiCallable):
    def __init__(self, method, base_callable, interceptor):
        self._method = method
        self._base_callable = base_callable
        self._interceptor = interceptor

    def __call__(
        self, request_iterator, timeout=None, metadata=None, credentials=None
    ):
        def invoker(request_iterator, metadata):
            return self._base_callable(
                request_iterator, timeout, metadata, credentials
            )

        client_info = _StreamClientInfo(self._method, True, True, timeout)
        return self._interceptor.intercept_stream(
            request_iterator, metadata, client_info, invoker
        )


class _InterceptorChannel(grpc.Channel):
    def __init__(self, channel, interceptor):
        self._channel = channel
        self._interceptor = interceptor

    def subscribe(self, *args, **kwargs):
        self._channel.subscribe(*args, **kwargs)

    def unsubscribe(self, *args, **kwargs):
        self._channel.unsubscribe(*args, **kwargs)

    def unary_unary(
        self, method, request_serializer=None, response_deserializer=None
    ):
        base_callable = self._channel.unary_unary(
            method, request_serializer, response_deserializer
        )
        if isinstance(self._interceptor, grpcext.UnaryClientInterceptor):
            return _InterceptorUnaryUnaryMultiCallable(
                method, base_callable, self._interceptor
            )
        return base_callable

    def unary_stream(
        self, method, request_serializer=None, response_deserializer=None
    ):
        base_callable = self._channel.unary_stream(
            method, request_serializer, response_deserializer
        )
        if isinstance(self._interceptor, grpcext.StreamClientInterceptor):
            return _InterceptorUnaryStreamMultiCallable(
                method, base_callable, self._interceptor
            )
        return base_callable

    def stream_unary(
        self, method, request_serializer=None, response_deserializer=None
    ):
        base_callable = self._channel.stream_unary(
            method, request_serializer, response_deserializer
        )
        if isinstance(self._interceptor, grpcext.StreamClientInterceptor):
            return _InterceptorStreamUnaryMultiCallable(
                method, base_callable, self._interceptor
            )
        return base_callable

    def stream_stream(
        self, method, request_serializer=None, response_deserializer=None
    ):
        base_callable = self._channel.stream_stream(
            method, request_serializer, response_deserializer
        )
        if isinstance(self._interceptor, grpcext.StreamClientInterceptor):
            return _InterceptorStreamStreamMultiCallable(
                method, base_callable, self._interceptor
            )
        return base_callable

    def close(self):
        if not hasattr(self._channel, "close"):
            raise RuntimeError(
                "close() is not supported with the installed version of grpcio"
            )
        self._channel.close()

    def __enter__(self):
        """Enters the runtime context related to the channel object."""
        raise NotImplementedError()

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Exits the runtime context related to the channel object."""
        raise NotImplementedError()


def intercept_channel(channel, *interceptors):
    result = channel
    for interceptor in interceptors:
        if not isinstance(
            interceptor, grpcext.UnaryClientInterceptor
        ) and not isinstance(interceptor, grpcext.StreamClientInterceptor):
            raise TypeError(
                "interceptor must be either a "
                "grpcext.UnaryClientInterceptor or a "
                "grpcext.StreamClientInterceptor"
            )
        result = _InterceptorChannel(result, interceptor)
    return result
