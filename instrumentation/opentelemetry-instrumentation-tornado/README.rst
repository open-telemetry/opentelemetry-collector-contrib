OpenTelemetry Tornado Instrumentation
======================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-tornado.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-tornado/

This library builds on the OpenTelemetry WSGI middleware to track web requests
in Tornado applications.

Installation
------------

::

    pip install opentelemetry-instrumentation-tornado

Configuration
-------------

The following environment variables are supported as configuration options:

- OTEL_PYTHON_TORNADO_EXCLUDED_URLS 

A comma separated list of paths that should not be automatically traced. For example, if this is set to 

::

    export OTEL_PYTHON_TORNADO_EXLUDED_URLS='/healthz,/ping'

Then any requests made to ``/healthz`` and ``/ping`` will not be automatically traced.

Request attributes
********************
To extract certain attributes from Tornado's request object and use them as span attributes, set the environment variable ``OTEL_PYTHON_TORNADO_TRACED_REQUEST_ATTRS`` to a comma
delimited list of request attribute names. 

For example,

::

    export OTEL_PYTHON_TORNADO_TRACED_REQUEST_ATTRS='uri,query'

will extract path_info and content_type attributes from every traced request and add them as span attributes.

References
----------

* `OpenTelemetry Tornado Instrumentation <https://opentelemetry-python.readthedocs.io/en/latest/instrumentation/tornado/tornado.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
