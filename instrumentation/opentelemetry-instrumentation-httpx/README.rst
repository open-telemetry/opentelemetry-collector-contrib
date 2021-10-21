OpenTelemetry HTTPX Instrumentation
===================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-httpx.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-httpx/

This library allows tracing HTTP requests made by the
`httpx <https://www.python-httpx.org/>`_ library.

Installation
------------

::

     pip install opentelemetry-instrumentation-httpx


Usage
-----

Instrumenting all clients
*************************

When using the instrumentor, all clients will automatically trace requests.

.. code-block:: python

     import httpx
     from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

     url = "https://httpbin.org/get"
     HTTPXClientInstrumentor().instrument()

     with httpx.Client() as client:
          response = client.get(url)

     async with httpx.AsyncClient() as client:
          response = await client.get(url)

Instrumenting single clients
****************************

If you only want to instrument requests for specific client instances, you can
use the `instrument_client` method.


.. code-block:: python

    import httpx
    from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

    url = "https://httpbin.org/get"

    with httpx.Client(transport=telemetry_transport) as client:
        HTTPXClientInstrumentor.instrument_client(client)
        response = client.get(url)

    async with httpx.AsyncClient(transport=telemetry_transport) as client:
        HTTPXClientInstrumentor.instrument_client(client)
        response = await client.get(url)


Uninstrument
************

If you need to uninstrument clients, there are two options available.

.. code-block:: python

    import httpx
    from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

    HTTPXClientInstrumentor().instrument()
    client = httpx.Client()

    # Uninstrument a specific client
    HTTPXClientInstrumentor.uninstrument_client(client)

    # Uninstrument all clients
    HTTPXClientInstrumentor().uninstrument()


Using transports directly
*************************

If you don't want to use the instrumentor class, you can use the transport classes directly.


.. code-block:: python

    import httpx
    from opentelemetry.instrumentation.httpx import (
        AsyncOpenTelemetryTransport,
        SyncOpenTelemetryTransport,
    )

    url = "https://httpbin.org/get"
    transport = httpx.HTTPTransport()
    telemetry_transport = SyncOpenTelemetryTransport(transport)

    with httpx.Client(transport=telemetry_transport) as client:
        response = client.get(url)

    transport = httpx.AsyncHTTPTransport()
    telemetry_transport = AsyncOpenTelemetryTransport(transport)

    async with httpx.AsyncClient(transport=telemetry_transport) as client:
        response = await client.get(url)


Request and response hooks
***************************

The instrumentation supports specifying request and response hooks. These are functions that get called back by the instrumentation right after a span is created for a request
and right before the span is finished while processing a response.

.. note::

    The request hook receives the raw arguments provided to the transport layer. The response hook receives the raw return values from the transport layer.

The hooks can be configured as follows:


.. code-block:: python

    from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

    def request_hook(span, request):
        # method, url, headers, stream, extensions = request
        pass

    def response_hook(span, request, response):
        # method, url, headers, stream, extensions = request
        # status_code, headers, stream, extensions = response
        pass

    HTTPXClientInstrumentor().instrument(request_hook=request_hook, response_hook=response_hook)


Or if you are using the transport classes directly:


.. code-block:: python

    from opentelemetry.instrumentation.httpx import SyncOpenTelemetryTransport

    def request_hook(span, request):
        # method, url, headers, stream, extensions = request
        pass

    def response_hook(span, request, response):
        # method, url, headers, stream, extensions = request
        # status_code, headers, stream, extensions = response
        pass

    transport = httpx.HTTPTransport()
    telemetry_transport = SyncOpenTelemetryTransport(
        transport,
        request_hook=request_hook,
        response_hook=response_hook
    )


References
----------

* `OpenTelemetry HTTPX Instrumentation <https://opentelemetry-python-contrib.readthedocs.io/en/latest/instrumentation/httpx/httpx.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
