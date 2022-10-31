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

    grpc_client_instrumentor = GrpcInstrumentorClient()
    grpc_client_instrumentor.instrument()

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

You can also add the interceptor manually, rather than using
:py:class:`~opentelemetry.instrumentation.grpc.GrpcInstrumentorServer`:

.. code-block:: python

    from opentelemetry.instrumentation.grpc import server_interceptor

    server = grpc.server(futures.ThreadPoolExecutor(),
                         interceptors = [server_interceptor()])

Usage Aio Client
----------------
.. code-block:: python

    import logging
    import asyncio

    import grpc

    from opentelemetry import trace
    from opentelemetry.instrumentation.grpc import GrpcAioInstrumentorClient
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

    grpc_client_instrumentor = GrpcAioInstrumentorClient()
    grpc_client_instrumentor.instrument()

    async def run():
        with grpc.aio.insecure_channel("localhost:50051") as channel:

            stub = helloworld_pb2_grpc.GreeterStub(channel)
            response = await stub.SayHello(helloworld_pb2.HelloRequest(name="YOU"))

        print("Greeter client received: " + response.message)


    if __name__ == "__main__":
        logging.basicConfig()
        asyncio.run(run())

You can also add the interceptor manually, rather than using
:py:class:`~opentelemetry.instrumentation.grpc.GrpcAioInstrumentorClient`:

.. code-block:: python

    from opentelemetry.instrumentation.grpc import aio_client_interceptors

    channel = grpc.aio.insecure_channel("localhost:12345", interceptors=aio_client_interceptors())


Usage Aio Server
----------------
.. code-block:: python

    import logging
    import asyncio

    import grpc

    from opentelemetry import trace
    from opentelemetry.instrumentation.grpc import GrpcAioInstrumentorServer
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

    grpc_server_instrumentor = GrpcAioInstrumentorServer()
    grpc_server_instrumentor.instrument()

    class Greeter(helloworld_pb2_grpc.GreeterServicer):
        async def SayHello(self, request, context):
            return helloworld_pb2.HelloReply(message="Hello, %s!" % request.name)


    async def serve():

        server = grpc.aio.server()

        helloworld_pb2_grpc.add_GreeterServicer_to_server(Greeter(), server)
        server.add_insecure_port("[::]:50051")
        await server.start()
        await server.wait_for_termination()


    if __name__ == "__main__":
        logging.basicConfig()
        asyncio.run(serve())

You can also add the interceptor manually, rather than using
:py:class:`~opentelemetry.instrumentation.grpc.GrpcAioInstrumentorServer`:

.. code-block:: python

    from opentelemetry.instrumentation.grpc import aio_server_interceptor

    server = grpc.aio.server(interceptors = [aio_server_interceptor()])

Filters
-------

If you prefer to filter specific requests to be instrumented, you can specify
the condition by assigning filters to instrumentors.

You can write a global server instrumentor as follows:

.. code-block::

    from opentelemetry.instrumentation.grpc import filters, GrpcInstrumentorServer

    grpc_server_instrumentor = GrpcInstrumentorServer(
        filter_ = filters.any_of(
            filters.method_name("SimpleMethod"),
            filters.method_name("ComplexMethod"),
        )
    )
    grpc_server_instrumentor.instrument()

You can also use the filters directly on the provided interceptors:

.. code-block::

    my_interceptor = server_interceptor(
        filter_ = filters.negate(filters.method_name("TestMethod"))
    )
    server = grpc.server(futures.ThreadPoolExecutor(),
                         interceptors = [my_interceptor])

``filter_`` option also applies to both global and manual client intrumentors.


Environment variable
--------------------

If you'd like to exclude specific services for the instrumentations, you can use
``OTEL_PYTHON_GRPC_EXCLUDED_SERVICES`` environment variables.

For example, if you assign ``"GRPCTestServer,GRPCHealthServer"`` to the variable,
then the global interceptor automatically adds the filters to exclude requests to
services ``GRPCTestServer`` and ``GRPCHealthServer``.

"""
import os
from typing import Callable, Collection, List, Union

import grpc  # pylint:disable=import-self
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry import trace
from opentelemetry.instrumentation.grpc.filters import (
    any_of,
    negate,
    service_name,
)
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

        If you want to add a filter that only intercept requests
        to match the condition, pass ``filter_`` to GrpcInstrumentorServer.

        grpc_server_instrumentor = GrpcInstrumentorServer(
            filter_=filters.method_prefix("SimpleMethod"))
        grpc_server_instrumentor.instrument()

    """

    # pylint:disable=attribute-defined-outside-init, redefined-outer-name

    def __init__(self, filter_=None):
        excluded_service_filter = _excluded_service_filter()
        if excluded_service_filter is not None:
            if filter_ is None:
                filter_ = excluded_service_filter
            else:
                filter_ = any_of(filter_, excluded_service_filter)
        self._filter = filter_

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        self._original_func = grpc.server
        tracer_provider = kwargs.get("tracer_provider")

        def server(*args, **kwargs):
            if "interceptors" in kwargs:
                # add our interceptor as the first
                kwargs["interceptors"].insert(
                    0,
                    server_interceptor(
                        tracer_provider=tracer_provider, filter_=self._filter
                    ),
                )
            else:
                kwargs["interceptors"] = [
                    server_interceptor(
                        tracer_provider=tracer_provider, filter_=self._filter
                    )
                ]
            return self._original_func(*args, **kwargs)

        grpc.server = server

    def _uninstrument(self, **kwargs):
        grpc.server = self._original_func


class GrpcAioInstrumentorServer(BaseInstrumentor):
    """
    Globally instrument the grpc.aio server.

    Usage::

        grpc_aio_server_instrumentor = GrpcAioInstrumentorServer()
        grpc_aio_server_instrumentor.instrument()

    """

    # pylint:disable=attribute-defined-outside-init, redefined-outer-name

    def __init__(self, filter_=None):
        excluded_service_filter = _excluded_service_filter()
        if excluded_service_filter is not None:
            if filter_ is None:
                filter_ = excluded_service_filter
            else:
                filter_ = any_of(filter_, excluded_service_filter)
        self._filter = filter_

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        self._original_func = grpc.aio.server
        tracer_provider = kwargs.get("tracer_provider")

        def server(*args, **kwargs):
            if "interceptors" in kwargs:
                # add our interceptor as the first
                kwargs["interceptors"].insert(
                    0,
                    aio_server_interceptor(
                        tracer_provider=tracer_provider, filter_=self._filter
                    ),
                )
            else:
                kwargs["interceptors"] = [
                    aio_server_interceptor(
                        tracer_provider=tracer_provider, filter_=self._filter
                    )
                ]
            return self._original_func(*args, **kwargs)

        grpc.aio.server = server

    def _uninstrument(self, **kwargs):
        grpc.aio.server = self._original_func


class GrpcInstrumentorClient(BaseInstrumentor):
    """
    Globally instrument the grpc client

    Usage::

        grpc_client_instrumentor = GrpcInstrumentorClient()
        grpc_client_instrumentor.instrument()

        If you want to add a filter that only intercept requests
        to match the condition, pass ``filter_`` option to GrpcInstrumentorClient.

        grpc_client_instrumentor = GrpcInstrumentorClient(
            filter_=filters.negate(filters.health_check())
        )
        grpc_client_instrumentor.instrument()

    """

    def __init__(self, filter_=None):
        excluded_service_filter = _excluded_service_filter()
        if excluded_service_filter is not None:
            if filter_ is None:
                filter_ = excluded_service_filter
            else:
                filter_ = any_of(filter_, excluded_service_filter)
        self._filter = filter_

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
                "grpc",
                ctype,
                self.wrapper_fn,
            )

    def _uninstrument(self, **kwargs):
        for ctype in self._which_channel(kwargs):
            unwrap(grpc, ctype)

    def wrapper_fn(self, original_func, instance, args, kwargs):
        channel = original_func(*args, **kwargs)
        tracer_provider = kwargs.get("tracer_provider")
        return intercept_channel(
            channel,
            client_interceptor(
                tracer_provider=tracer_provider,
                filter_=self._filter,
            ),
        )


class GrpcAioInstrumentorClient(BaseInstrumentor):
    """
    Globally instrument the grpc.aio client.

    Usage::

        grpc_aio_client_instrumentor = GrpcAioInstrumentorClient()
        grpc_aio_client_instrumentor.instrument()

    """

    # pylint:disable=attribute-defined-outside-init, redefined-outer-name

    def __init__(self, filter_=None):
        excluded_service_filter = _excluded_service_filter()
        if excluded_service_filter is not None:
            if filter_ is None:
                filter_ = excluded_service_filter
            else:
                filter_ = any_of(filter_, excluded_service_filter)
        self._filter = filter_

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _add_interceptors(self, tracer_provider, kwargs):
        if "interceptors" in kwargs and kwargs["interceptors"]:
            kwargs["interceptors"] = (
                aio_client_interceptors(
                    tracer_provider=tracer_provider, filter_=self._filter
                )
                + kwargs["interceptors"]
            )
        else:
            kwargs["interceptors"] = aio_client_interceptors(
                tracer_provider=tracer_provider, filter_=self._filter
            )

        return kwargs

    def _instrument(self, **kwargs):
        self._original_insecure = grpc.aio.insecure_channel
        self._original_secure = grpc.aio.secure_channel
        tracer_provider = kwargs.get("tracer_provider")

        def insecure(*args, **kwargs):
            kwargs = self._add_interceptors(tracer_provider, kwargs)

            return self._original_insecure(*args, **kwargs)

        def secure(*args, **kwargs):
            kwargs = self._add_interceptors(tracer_provider, kwargs)

            return self._original_secure(*args, **kwargs)

        grpc.aio.insecure_channel = insecure
        grpc.aio.secure_channel = secure

    def _uninstrument(self, **kwargs):
        grpc.aio.insecure_channel = self._original_insecure
        grpc.aio.secure_channel = self._original_secure


def client_interceptor(tracer_provider=None, filter_=None):
    """Create a gRPC client channel interceptor.

    Args:
        tracer: The tracer to use to create client-side spans.

        filter_: filter function that returns True if gRPC requests
                 matches the condition. Default is None and intercept
                 all requests.

    Returns:
        An invocation-side interceptor object.
    """
    from . import _client

    tracer = trace.get_tracer(__name__, __version__, tracer_provider)

    return _client.OpenTelemetryClientInterceptor(tracer, filter_=filter_)


def server_interceptor(tracer_provider=None, filter_=None):
    """Create a gRPC server interceptor.

    Args:
        tracer: The tracer to use to create server-side spans.

        filter_: filter function that returns True if gRPC requests
                 matches the condition. Default is None and intercept
                 all requests.

    Returns:
        A service-side interceptor object.
    """
    from . import _server

    tracer = trace.get_tracer(__name__, __version__, tracer_provider)

    return _server.OpenTelemetryServerInterceptor(tracer, filter_=filter_)


def aio_client_interceptors(tracer_provider=None, filter_=None):
    """Create a gRPC client channel interceptor.

    Args:
        tracer: The tracer to use to create client-side spans.

    Returns:
        An invocation-side interceptor object.
    """
    from . import _aio_client

    tracer = trace.get_tracer(__name__, __version__, tracer_provider)

    return [
        _aio_client.UnaryUnaryAioClientInterceptor(tracer, filter_=filter_),
        _aio_client.UnaryStreamAioClientInterceptor(tracer, filter_=filter_),
        _aio_client.StreamUnaryAioClientInterceptor(tracer, filter_=filter_),
        _aio_client.StreamStreamAioClientInterceptor(tracer, filter_=filter_),
    ]


def aio_server_interceptor(tracer_provider=None, filter_=None):
    """Create a gRPC aio server interceptor.

    Args:
        tracer: The tracer to use to create server-side spans.

    Returns:
        A service-side interceptor object.
    """
    from . import _aio_server

    tracer = trace.get_tracer(__name__, __version__, tracer_provider)

    return _aio_server.OpenTelemetryAioServerInterceptor(
        tracer, filter_=filter_
    )


def _excluded_service_filter() -> Union[Callable[[object], bool], None]:
    services = _parse_services(
        os.environ.get("OTEL_PYTHON_GRPC_EXCLUDED_SERVICES", "")
    )
    if len(services) == 0:
        return None
    filters = (service_name(srv) for srv in services)
    return negate(any_of(*filters))


def _parse_services(excluded_services: str) -> List[str]:
    if excluded_services != "":
        excluded_service_list = [
            s.strip() for s in excluded_services.split(",")
        ]
    else:
        excluded_service_list = []
    return excluded_service_list
