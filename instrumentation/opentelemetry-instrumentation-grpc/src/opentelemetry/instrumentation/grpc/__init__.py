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

# pylint:disable=no-name-in-module
# pylint:disable=relative-beyond-top-level
# pylint:disable=import-error
# pylint:disable=no-self-use
"""
Usage Client
------------
.. code-block:: python

    import logging

    import grpc

    from opentelemetry import trace
    from opentelemetry.instrumentation.grpc import GrpcInstrumentorClient
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import (
        ConsoleSpanExporter,
        SimpleSpanProcessor,
    )

    try:
        from .gen import helloworld_pb2, helloworld_pb2_grpc
    except ImportError:
        from gen import helloworld_pb2, helloworld_pb2_grpc

    trace.set_tracer_provider(TracerProvider())
    trace.get_tracer_provider().add_span_processor(
        SimpleSpanProcessor(ConsoleSpanExporter())
    )

    instrumentor = GrpcInstrumentorClient().instrument()

    def run():
        with grpc.insecure_channel("localhost:50051") as channel:

            stub = helloworld_pb2_grpc.GreeterStub(channel)
            response = stub.SayHello(helloworld_pb2.HelloRequest(name="YOU"))

        print("Greeter client received: " + response.message)


    if __name__ == "__main__":
        logging.basicConfig()
        run()

Usage Server
------------
.. code-block:: python

    import logging
    from concurrent import futures

    import grpc

    from opentelemetry import trace
    from opentelemetry.instrumentation.grpc import GrpcInstrumentorServer
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import (
        ConsoleSpanExporter,
        SimpleSpanProcessor,
    )

    try:
        from .gen import helloworld_pb2, helloworld_pb2_grpc
    except ImportError:
        from gen import helloworld_pb2, helloworld_pb2_grpc

    trace.set_tracer_provider(TracerProvider())
    trace.get_tracer_provider().add_span_processor(
        SimpleSpanProcessor(ConsoleSpanExporter())
    )

    grpc_server_instrumentor = GrpcInstrumentorServer()
    grpc_server_instrumentor.instrument()

    class Greeter(helloworld_pb2_grpc.GreeterServicer):
        def SayHello(self, request, context):
            return helloworld_pb2.HelloReply(message="Hello, %s!" % request.name)


    def serve():

        server = grpc.server(futures.ThreadPoolExecutor())

        helloworld_pb2_grpc.add_GreeterServicer_to_server(Greeter(), server)
        server.add_insecure_port("[::]:50051")
        server.start()
        server.wait_for_termination()


    if __name__ == "__main__":
        logging.basicConfig()
        serve()

You can also add the instrumentor manually, rather than using
:py:class:`~opentelemetry.instrumentation.grpc.GrpcInstrumentorServer`:

.. code-block:: python

    from opentelemetry.instrumentation.grpc import server_interceptor

    server = grpc.server(futures.ThreadPoolExecutor(),
                         interceptors = [server_interceptor()])

"""
from typing import Collection

import grpc  # pylint:disable=import-self
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry import trace
from opentelemetry.instrumentation.grpc.grpcext import intercept_channel
from opentelemetry.instrumentation.grpc.package import _instruments
from opentelemetry.instrumentation.grpc.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import unwrap

# pylint:disable=import-outside-toplevel
# pylint:disable=import-self
# pylint:disable=unused-argument


class GrpcInstrumentorServer(BaseInstrumentor):
    """
    Globally instrument the grpc server.

    Usage::

        grpc_server_instrumentor = GrpcInstrumentorServer()
        grpc_server_instrumentor.instrument()

    """

    # pylint:disable=attribute-defined-outside-init, redefined-outer-name

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        self._original_func = grpc.server
        tracer_provider = kwargs.get("tracer_provider")

        def server(*args, **kwargs):
            if "interceptors" in kwargs:
                # add our interceptor as the first
                kwargs["interceptors"].insert(
                    0, server_interceptor(tracer_provider=tracer_provider)
                )
            else:
                kwargs["interceptors"] = [
                    server_interceptor(tracer_provider=tracer_provider)
                ]
            return self._original_func(*args, **kwargs)

        grpc.server = server

    def _uninstrument(self, **kwargs):
        grpc.server = self._original_func


class GrpcInstrumentorClient(BaseInstrumentor):
    """
    Globally instrument the grpc client

    Usage::

        grpc_client_instrumentor = GrpcInstrumentorClient()
        grpc.client_instrumentor.instrument()

    """

    # Figures out which channel type we need to wrap
    def _which_channel(self, kwargs):
        # handle legacy argument
        if "channel_type" in kwargs:
            if kwargs.get("channel_type") == "secure":
                return ("secure_channel",)
            return ("insecure_channel",)

        # handle modern arguments
        types = []
        for ctype in ("secure_channel", "insecure_channel"):
            if kwargs.get(ctype, True):
                types.append(ctype)

        return tuple(types)

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        for ctype in self._which_channel(kwargs):
            _wrap(
                "grpc", ctype, self.wrapper_fn,
            )

    def _uninstrument(self, **kwargs):
        for ctype in self._which_channel(kwargs):
            unwrap(grpc, ctype)

    def wrapper_fn(self, original_func, instance, args, kwargs):
        channel = original_func(*args, **kwargs)
        tracer_provider = kwargs.get("tracer_provider")
        return intercept_channel(
            channel, client_interceptor(tracer_provider=tracer_provider),
        )


def client_interceptor(tracer_provider=None):
    """Create a gRPC client channel interceptor.

    Args:
        tracer: The tracer to use to create client-side spans.

    Returns:
        An invocation-side interceptor object.
    """
    from . import _client

    tracer = trace.get_tracer(__name__, __version__, tracer_provider)

    return _client.OpenTelemetryClientInterceptor(tracer)


def server_interceptor(tracer_provider=None):
    """Create a gRPC server interceptor.

    Args:
        tracer: The tracer to use to create server-side spans.

    Returns:
        A service-side interceptor object.
    """
    from . import _server

    tracer = trace.get_tracer(__name__, __version__, tracer_provider)

    return _server.OpenTelemetryServerInterceptor(tracer)
