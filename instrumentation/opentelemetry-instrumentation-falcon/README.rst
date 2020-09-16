OpenTelemetry Falcon Tracing
============================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-falcon.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-falcon/

This library builds on the OpenTelemetry WSGI middleware to track web requests
in Falcon applications.

Installation
------------

::

    pip install opentelemetry-instrumentation-falcon

Configuration
-------------

Exclude lists
*************
To exclude certain URLs from being tracked, set the environment variable ``OTEL_PYTHON_FALCON_EXCLUDED_URLS`` with comma delimited regexes representing which URLs to exclude.

For example,

::

    export OTEL_PYTHON_FALCON_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.

References
----------

* `OpenTelemetry Falcon Instrumentation <https://opentelemetry-python.readthedocs.io/en/latest/instrumentation/falcon/falcon.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
