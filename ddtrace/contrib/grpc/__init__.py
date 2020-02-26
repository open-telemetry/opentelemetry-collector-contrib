"""
The gRPC integration traces the client and server using interceptor pattern.

gRPC will be automatically instrumented with ``patch_all``, or when using
the ``ddtrace-run`` command.
gRPC is instrumented on import. To instrument gRPC manually use the
``patch`` function.::

    import grpc
    from ddtrace import patch
    patch(grpc=True)

    # use grpc like usual

To configure the gRPC integration on an per-channel basis use the
``Pin`` API::

    import grpc
    from ddtrace import Pin, patch, Tracer

    patch(grpc=True)
    custom_tracer = Tracer()

    # override the pin on the client
    Pin.override(grpc.Channel, service='mygrpc', tracer=custom_tracer)
    with grpc.insecure_channel('localhost:50051') as channel:
        # create stubs and send requests
        pass

To configure the gRPC integration on the server use the ``Pin`` API::

    import grpc
    from grpc.framework.foundation import logging_pool

    from ddtrace import Pin, patch, Tracer

    patch(grpc=True)
    custom_tracer = Tracer()

    # override the pin on the server
    Pin.override(grpc.Server, service='mygrpc', tracer=custom_tracer)
    server = grpc.server(logging_pool.pool(2))
    server.add_insecure_port('localhost:50051')
    add_MyServicer_to_server(MyServicer(), server)
    server.start()
"""


from ...utils.importlib import require_modules

required_modules = ['grpc']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch, unpatch

        __all__ = ['patch', 'unpatch']
