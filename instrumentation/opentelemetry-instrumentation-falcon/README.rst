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

Request attributes
********************
To extract certain attributes from Falcon's request object and use them as span attributes, set the environment variable ``OTEL_PYTHON_FALCON_TRACED_REQUEST_ATTRS`` to a comma
delimited list of request attribute names. 

For example,

::

    export OTEL_PYTHON_FALCON_TRACED_REQUEST_ATTRS='query_string,uri_template'

will extract path_info and content_type attributes from every traced request and add them as span attritbues.

Falcon Request object reference: https://falcon.readthedocs.io/en/stable/api/request_and_response.html#id1

References
----------

* `OpenTelemetry Falcon Instrumentation <https://opentelemetry-python.readthedocs.io/en/latest/instrumentation/falcon/falcon.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
