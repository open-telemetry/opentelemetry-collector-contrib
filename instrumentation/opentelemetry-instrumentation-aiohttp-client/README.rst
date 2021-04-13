OpenTelemetry aiohttp client Integration
========================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-aiohttp-client.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-aiohttp-client/

This library allows tracing HTTP requests made by the
`aiohttp client <https://docs.aiohttp.org/en/stable/client.html>`_ library.

Installation
------------

::

     pip install opentelemetry-instrumentation-aiohttp-client


Example
-------

.. code:: python

   import asyncio
   
   import aiohttp

   from opentelemetry.instrumentation.aiohttp_client import AioHttpClientInstrumentor
   from opentelemetry import trace
   from opentelemetry.exporter.jaeger.thrift import JaegerExporter
   from opentelemetry.sdk.trace import TracerProvider
   from opentelemetry.sdk.trace.export import BatchSpanProcessor


   _JAEGER_EXPORTER = JaegerExporter(
      service_name="example-xxx",
      agent_host_name="localhost",
      agent_port=6831,
   )

   _TRACE_PROVIDER = TracerProvider()
   _TRACE_PROVIDER.add_span_processor(BatchSpanProcessor(_JAEGER_EXPORTER))
   trace.set_tracer_provider(_TRACE_PROVIDER)

   AioHttpClientInstrumentor().instrument()


   async def span_emitter():
      async with aiohttp.ClientSession() as session:
         async with session.get("https://example.com") as resp:
               print(resp.status)


   asyncio.run(span_emitter())


References
----------

* `OpenTelemetry Project <https://opentelemetry.io/>`_
* `aiohttp client Tracing <https://docs.aiohttp.org/en/stable/tracing_reference.html>`_
* `OpenTelemetry Python Examples <https://github.com/open-telemetry/opentelemetry-python/tree/main/docs/examples>`_
