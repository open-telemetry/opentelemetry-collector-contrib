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

Install the OpenTelemetry SDK package.

::

    pip install opentelemetry-sdk

Next, use the provided `AwsXRayIdGenerator` to initialize the `TracerProvider`.

.. code-block:: python

    import opentelemetry.trace as trace
    from opentelemetry.sdk.extension.aws.trace import AwsXRayIdGenerator
    from opentelemetry.sdk.trace import TracerProvider

    trace.set_tracer_provider(
        TracerProvider(id_generator=AwsXRayIdGenerator())
    )


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
    from opentelemetry.sdk.extension.aws.trace.propagation.aws_xray_format import AwsXRayFormat

    set_global_textmap(AwsXRayFormat())

Usage (AWS Resource Detectors)
------------------------------

Use the provided `Resource Detectors` to automatically populate attributes under the `resource`
namespace of each generated span.

For example, if tracing with OpenTelemetry on an AWS EC2 instance, you can automatically
populate `resource` attributes by creating a `TraceProvider` using the `AwsEc2ResourceDetector`:

.. code-block:: python

    import opentelemetry.trace as trace
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.extension.aws.resource.ec2 import (
        AwsEc2ResourceDetector,
    )
    from opentelemetry.sdk.resources import get_aggregated_resources

    trace.set_tracer_provider(
        TracerProvider(
            resource=get_aggregated_resources(
                [
                    AwsEc2ResourceDetector(),
                ]
            ),
        )
    )

Refer to each detectors' docstring to determine any possible requirements for that
detector.

References
----------

* `OpenTelemetry Project <https://opentelemetry.io/>`_
* `AWS X-Ray Trace IDs Format <https://docs.aws.amazon.com/xray/latest/devguide/xray-api-sendingdata.html#xray-api-traceids>`_
* `OpenTelemetry Specification for Resource Attributes <https://github.com/open-telemetry/opentelemetry-specification/tree/main/specification/resource/semantic_conventions>`_
