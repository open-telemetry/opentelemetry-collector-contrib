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

# pylint:disable=import-outside-toplevel
# pylint:disable=import-self
# pylint:disable=no-name-in-module

import abc


class UnaryClientInfo(abc.ABC):
    """Consists of various information about a unary RPC on the
      invocation-side.

    Attributes:
      full_method: A string of the full RPC method, i.e.,
          /package.service/method.
      timeout: The length of time in seconds to wait for the computation to
        terminate or be cancelled, or None if this method should block until
        the computation is terminated or is cancelled no matter how long that
        takes.
    """


class StreamClientInfo(abc.ABC):
    """Consists of various information about a stream RPC on the
      invocation-side.

    Attributes:
      full_method: A string of the full RPC method, i.e.,
          /package.service/method.
      is_client_stream: Indicates whether the RPC is client-streaming.
      is_server_stream: Indicates whether the RPC is server-streaming.
      timeout: The length of time in seconds to wait for the computation to
        terminate or be cancelled, or None if this method should block until
        the computation is terminated or is cancelled no matter how long that
        takes.
    """


class UnaryClientInterceptor(abc.ABC):
    """Affords intercepting unary-unary RPCs on the invocation-side."""

    @abc.abstractmethod
    def intercept_unary(self, request, metadata, client_info, invoker):
        """Intercepts unary-unary RPCs on the invocation-side.

        Args:
          request: The request value for the RPC.
          metadata: Optional :term:`metadata` to be transmitted to the
            service-side of the RPC.
          client_info: A UnaryClientInfo containing various information about
            the RPC.
          invoker: The handler to complete the RPC on the client. It is the
            interceptor's responsibility to call it.

        Returns:
          The result from calling invoker(request, metadata).
        """
        raise NotImplementedError()


class StreamClientInterceptor(abc.ABC):
    """Affords intercepting stream RPCs on the invocation-side."""

    @abc.abstractmethod
    def intercept_stream(
        self, request_or_iterator, metadata, client_info, invoker
    ):
        """Intercepts stream RPCs on the invocation-side.

        Args:
          request_or_iterator: The request value for the RPC if
            `client_info.is_client_stream` is `false`; otherwise, an iterator of
            request values.
          metadata: Optional :term:`metadata` to be transmitted to the service-side
            of the RPC.
          client_info: A StreamClientInfo containing various information about
            the RPC.
          invoker:  The handler to complete the RPC on the client. It is the
            interceptor's responsibility to call it.

          Returns:
            The result from calling invoker(metadata).
        """
        raise NotImplementedError()


def intercept_channel(channel, *interceptors):
    """Creates an intercepted channel.

    Args:
      channel: A Channel.
      interceptors: Zero or more UnaryClientInterceptors or
        StreamClientInterceptors

    Returns:
      A Channel.

    Raises:
      TypeError: If an interceptor derives from neither UnaryClientInterceptor
        nor StreamClientInterceptor.
    """
    from . import _interceptor

    return _interceptor.intercept_channel(channel, *interceptors)


__all__ = (
    "UnaryClientInterceptor",
    "StreamClientInfo",
    "StreamClientInterceptor",
    "intercept_channel",
)
