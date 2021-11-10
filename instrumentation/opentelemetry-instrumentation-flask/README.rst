OpenTelemetry Flask Tracing
===========================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-flask.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-flask/

This library builds on the OpenTelemetry WSGI middleware to track web requests
in Flask applications.

Installation
------------

::

    pip install opentelemetry-instrumentation-flask

Configuration
-------------

Exclude lists
*************
To exclude certain URLs from being tracked, set the environment variable ``OTEL_PYTHON_FLASK_EXCLUDED_URLS``
(or ``OTEL_PYTHON_EXCLUDED_URLS`` as fallback) with comma delimited regexes representing which URLs to exclude.

For example,

::

    export OTEL_PYTHON_FLASK_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.

You can also pass the comma delimited regexes to the ``instrument_app`` method directly:

.. code-block:: python

    FlaskInstrumentor().instrument_app(app, excluded_urls="client/.*/info,healthcheck")

Request/Response hooks
**********************

Utilize request/reponse hooks to execute custom logic to be performed before/after performing a request. Environ is an instance of WSGIEnvironment (flask.request.environ).
Response_headers is a list of key-value (tuples) representing the response headers returned from the response.

.. code-block:: python

    def request_hook(span: Span, environ: WSGIEnvironment):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_request_hook", "some-value")

    def response_hook(span: Span, status: str, response_headers: List):
        if span and span.is_recording():
            span.set_attribute("custom_user_attribute_from_response_hook", "some-value")

    FlaskInstrumentation().instrument(request_hook=request_hook, response_hook=response_hook)

Flask Request object reference: https://flask.palletsprojects.com/en/2.0.x/api/#flask.Request

References
----------

* `OpenTelemetry Flask Instrumentation <https://opentelemetry-python-contrib.readthedocs.io/en/stable/instrumentation/flask/flask.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
* `OpenTelemetry Python Examples <https://github.com/open-telemetry/opentelemetry-python/tree/main/docs/examples>`_
