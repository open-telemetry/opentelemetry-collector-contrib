OpenTelemetry urllib3 Instrumentation
======================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-urllib3.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-urllib3/

This library allows tracing HTTP requests made by the
`urllib3 <https://urllib3.readthedocs.io/>`_ library.

Installation
------------

::

     pip install opentelemetry-instrumentation-urllib3

Configuration
-------------

Request/Response hooks
**********************

The urllib3 instrumentation supports extending tracing behavior with the help of
request and response hooks. These are functions that are called back by the instrumentation
right after a Span is created for a request and right before the span is finished processing a response respectively.
The hooks can be configured as follows:

.. code:: python

    # `request` is an instance of urllib3.connectionpool.HTTPConnectionPool
    def request_hook(span, request):
        pass

    # `request` is an instance of urllib3.connectionpool.HTTPConnectionPool
    # `response` is an instance of urllib3.response.HTTPResponse
    def response_hook(span, request, response):
        pass

    URLLib3Instrumentor.instrument(
        request_hook=request_hook, response_hook=response_hook)
    )

References
----------

* `OpenTelemetry urllib3 Instrumentation <https://opentelemetry-python-contrib.readthedocs.io/en/latest/instrumentation/urllib3/urllib3.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
* `OpenTelemetry Python Examples <https://github.com/open-telemetry/opentelemetry-python/tree/main/docs/examples>`_
