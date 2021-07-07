OpenTelemetry Requests Instrumentation
======================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-requests.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-requests/

This library allows tracing HTTP requests made by the
`requests <https://requests.readthedocs.io/en/master/>`_ library.

Installation
------------

::

     pip install opentelemetry-instrumentation-requests
     
Configuration
-------------

.. code-block:: python

     from opentelemetry.instrumentation.requests import RequestsInstrumentor
     RequestsInstrumentor().instrument()


References
----------

* `OpenTelemetry requests Instrumentation <https://opentelemetry-python-contrib.readthedocs.io/en/latest/instrumentation/requests/requests.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
* `OpenTelemetry Python Examples <https://github.com/open-telemetry/opentelemetry-python/tree/main/docs/examples>`_
