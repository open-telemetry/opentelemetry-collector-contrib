# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
Installation
------------

::

    pip install opentelemetry-sdk-extension-aws

AWS X-Ray IDs Generator
-----------------------

The **AWS X-Ray IDs Generator** provides a custom IDs Generator to make
traces generated using the OpenTelemetry SDKs `TracerProvider` compatible
with the AWS X-Ray backend service `trace ID format`_.

Usage
-----

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

API
---
.. _trace ID format: https://docs.aws.amazon.com/xray/latest/devguide/xray-api-sendingdata.html#xray-api-traceids
"""

import random
import time

from opentelemetry.sdk.trace.id_generator import IdGenerator, RandomIdGenerator


class AwsXRayIdGenerator(IdGenerator):
    """Generates tracing IDs compatible with the AWS X-Ray tracing service. In
    the X-Ray system, the first 32 bits of the `TraceId` are the Unix epoch time
    in seconds. Since spans (AWS calls them segments) with an embedded timestamp
    more than 30 days ago are rejected, a purely random `TraceId` risks being
    rejected by the service.

    See: https://docs.aws.amazon.com/xray/latest/devguide/xray-api-sendingdata.html#xray-api-traceids
    """

    random_id_generator = RandomIdGenerator()

    def generate_span_id(self) -> int:
        return self.random_id_generator.generate_span_id()

    @staticmethod
    def generate_trace_id() -> int:
        trace_time = int(time.time())
        trace_identifier = random.getrandbits(96)
        return (trace_time << 96) + trace_identifier
