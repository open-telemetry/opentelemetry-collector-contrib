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
    from opentelemetry.instrumentation.grpc import GrpcInstrumentorClient, client_interceptor
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import (
        ConsoleSpanExporter,
        SimpleExportSpanProcessor,
    )

    from opentelemetry import metrics
    from opentelemetry.sdk.metrics import MeterProvider
    from opentelemetry.sdk.metrics.export import ConsoleMetricsExporter

    try:
        from .gen import helloworld_pb2, helloworld_pb2_grpc
    except ImportError:
        from gen import helloworld_pb2, helloworld_pb2_grpc

    trace.set_tracer_provider(TracerProvider())
    trace.get_tracer_provider().add_span_processor(
        SimpleExportSpanProcessor(ConsoleSpanExporter())
    )

    # Set meter provider to opentelemetry-sdk's MeterProvider
    metrics.set_meter_provider(MeterProvider())

    # Optional - export GRPC specific metrics (latency, bytes in/out, errors) by passing an exporter
    instrumentor = GrpcInstrumentorClient(exporter=ConsoleMetricsExporter(), interval=10)
    instrumentor.instrument()

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
    from opentelemetry.instrumentation.grpc import GrpcInstrumentorServer, server_interceptor
    from opentelemetry.instrumentation.grpc.grpcext import intercept_server
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import (
        ConsoleSpanExporter,
        SimpleExportSpanProcessor,
    )

    try:
        from .gen import helloworld_pb2, helloworld_pb2_grpc
    except ImportError:
        from gen import helloworld_pb2, helloworld_pb2_grpc

    trace.set_tracer_provider(TracerProvider())
    trace.get_tracer_provider().add_span_processor(
        SimpleExportSpanProcessor(ConsoleSpanExporter())
    )
    grpc_server_instrumentor = GrpcInstrumentorServer()
    grpc_server_instrumentor.instrument()


    class Greeter(helloworld_pb2_grpc.GreeterServicer):
        def SayHello(self, request, context):
            return helloworld_pb2.HelloReply(message="Hello, %s!" % request.name)


    def serve():

        server = grpc.server(futures.ThreadPoolExecutor())
        server = intercept_server(server, server_interceptor())

        helloworld_pb2_grpc.add_GreeterServicer_to_server(Greeter(), server)
        server.add_insecure_port("[::]:50051")
        server.start()
        server.wait_for_termination()


    if __name__ == "__main__":
        logging.basicConfig()
        serve()
"""
from contextlib import contextmanager
from functools import partial

import grpc
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry import trace
from opentelemetry.instrumentation.grpc.grpcext import (
    intercept_channel,
    intercept_server,
)
from opentelemetry.instrumentation.grpc.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import unwrap

# pylint:disable=import-outside-toplevel
# pylint:disable=import-self
# pylint:disable=unused-argument
# isort:skip


class GrpcInstrumentorServer(BaseInstrumentor):
    def _instrument(self, **kwargs):
        _wrap("grpc", "server", self.wrapper_fn)

    def _uninstrument(self, **kwargs):
        unwrap(grpc, "server")

    def wrapper_fn(self, original_func, instance, args, kwargs):
        server = original_func(*args, **kwargs)
        return intercept_server(server, server_interceptor())


class GrpcInstrumentorClient(BaseInstrumentor):
    def _instrument(self, **kwargs):
        exporter = kwargs.get("exporter", None)
        interval = kwargs.get("interval", 30)
        if kwargs.get("channel_type") == "secure":
            _wrap(
                "grpc",
                "secure_channel",
                partial(self.wrapper_fn, exporter, interval),
            )

        else:
            _wrap(
                "grpc",
                "insecure_channel",
                partial(self.wrapper_fn, exporter, interval),
            )

    def _uninstrument(self, **kwargs):
        if kwargs.get("channel_type") == "secure":
            unwrap(grpc, "secure_channel")

        else:
            unwrap(grpc, "insecure_channel")

    def wrapper_fn(
        self, exporter, interval, original_func, instance, args, kwargs
    ):
        channel = original_func(*args, **kwargs)
        tracer_provider = kwargs.get("tracer_provider")
        return intercept_channel(
            channel,
            client_interceptor(
                tracer_provider=tracer_provider,
                exporter=exporter,
                interval=interval,
            ),
        )


def client_interceptor(tracer_provider=None, exporter=None, interval=30):
    """Create a gRPC client channel interceptor.

    Args:
        tracer: The tracer to use to create client-side spans.
        exporter: The exporter that will receive client metrics
        interval: Time between every export call

    Returns:
        An invocation-side interceptor object.
    """
    from . import _client

    tracer = trace.get_tracer(__name__, __version__, tracer_provider)

    return _client.OpenTelemetryClientInterceptor(tracer, exporter, interval)


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
