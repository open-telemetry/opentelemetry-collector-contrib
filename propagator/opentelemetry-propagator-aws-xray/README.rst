OpenTelemetry Propagator for AWS X-Ray Service
==============================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-propagator-aws-xray.svg
   :target: https://pypi.org/project/opentelemetry-propagator-aws-xray/


This library provides the propagator necessary to inject or extract a tracing
context across AWS services.

Installation
------------

::

    pip install opentelemetry-propagator-aws-xray


Usage (AWS X-Ray Propagator)
----------------------------

**NOTE**: Because the parent context extracted from the `X-Amzn-Trace-Id` header
assumes the context is _not_ sampled by default, users should make sure to add
`Sampled=1` to their `X-Amzn-Trace-Id` headers so that the child spans are
sampled.

Use the provided AWS X-Ray Propagator to inject the necessary context into
traces sent to external systems.

This can be done by either setting this environment variable:

::

    export OTEL_PROPAGATORS = xray


Or by setting this propagator in your instrumented application:

.. code-block:: python

    from opentelemetry.propagate import set_global_textmap
    from opentelemetry.propagators.aws import AwsXRayPropagator

    set_global_textmap(AwsXRayPropagator())


References
----------

* `OpenTelemetry Project <https://opentelemetry.io/>`_
* `AWS X-Ray Propagation Trace Header <https://docs.aws.amazon.com/xray/latest/devguide/xray-concepts.html#xray-concepts-tracingheader>`_
