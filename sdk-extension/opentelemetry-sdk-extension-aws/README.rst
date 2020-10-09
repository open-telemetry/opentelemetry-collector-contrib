OpenTelemetry SDK Extension for AWS X-Ray Compatibility
=======================================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-sdk-extension-aws.svg
   :target: https://pypi.org/project/opentelemetry-sdk-extension-aws/


This library provides components necessary to configure the OpenTelemetry SDK
for tracing with AWS X-Ray.

Installation
------------

::

    pip install opentelemetry-sdk-extension-aws


Usage (AWS X-Ray IDs Generator)
-------------------------------

Configure the OTel SDK TracerProvider with the provided custom IDs Generator to 
make spans compatible with the AWS X-Ray backend tracing service.

.. code-block:: python

    from opentelemetry.sdk.extension.aws.trace import AwsXRayIdsGenerator

    trace.set_tracer_provider(
        TracerProvider(ids_generator=AwsXRayIdsGenerator())
    )


Usage (AWS X-Ray Propagator)
----------------------------

Set this environment variable to have the OTel SDK use the provided AWS X-Ray 
Propagator:

::

    export OTEL_PYTHON_PROPAGATORS = aws_xray


References
----------

* `OpenTelemetry Project <https://opentelemetry.io/>`_
* `AWS X-Ray Trace IDs Format <https://docs.aws.amazon.com/xray/latest/devguide/xray-api-sendingdata.html#xray-api-traceids>`_
