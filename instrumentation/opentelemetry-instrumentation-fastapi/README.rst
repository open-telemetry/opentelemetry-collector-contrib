OpenTelemetry FastAPI Instrumentation
=======================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-fastapi.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-fastapi/


This library provides automatic and manual instrumentation of FastAPI web frameworks,
instrumenting http requests served by applications utilizing the framework.

auto-instrumentation using the opentelemetry-instrumentation package is also supported.

Installation
------------

::

    pip install opentelemetry-instrumentation-fastapi

Configuration
-------------

Exclude lists
*************
To exclude certain URLs from being tracked, set the environment variable ``OTEL_PYTHON_FASTAPI_EXCLUDED_URLS`` with comma delimited regexes representing which URLs to exclude.

For example,

::

    export OTEL_PYTHON_FASTAPI_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.


Usage
-----

.. code-block:: python

    import fastapi
    from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor

    app = fastapi.FastAPI()

    @app.get("/foobar")
    async def foobar():
        return {"message": "hello world"}

    FastAPIInstrumentor.instrument_app(app)

You can also pass the list of urls to exclude explicitly to the instrumentation call:

.. code-block:: python

    FastAPIInstrumentor.instrument_app(app, excluded_urls="client/.*/info,healthcheck")

References
----------

* `OpenTelemetry Project <https://opentelemetry.io/>`_
* `OpenTelemetry Python Examples <https://github.com/open-telemetry/opentelemetry-python/tree/main/docs/examples>`_
