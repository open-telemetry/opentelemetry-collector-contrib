OpenTelemetry Starlette Instrumentation
=======================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-starlette.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-starlette/


This library provides automatic and manual instrumentation of Starlette web frameworks,
instrumenting http requests served by applications utilizing the framework.

auto-instrumentation using the opentelemetry-instrumentation package is also supported.

Installation
------------

::

    pip install opentelemetry-instrumentation-starlette


Usage
-----

.. code-block:: python

    from opentelemetry.instrumentation.starlette import StarletteInstrumentor
    from starlette import applications
    from starlette.responses import PlainTextResponse
    from starlette.routing import Route

    def home(request):
        return PlainTextResponse("hi")

    app = applications.Starlette(
        routes=[Route("/foobar", home)]
    )
    StarletteInstrumentor.instrument_app(app)


References
----------

* `OpenTelemetry Project <https://opentelemetry.io/>`_
