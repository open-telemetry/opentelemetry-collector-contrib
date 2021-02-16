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
The **OpenTelemetry Datadog Exporter** provides a span exporter from
`OpenTelemetry`_ traces to `Datadog`_ by using the Datadog Agent.

Installation
------------

::

    pip install opentelemetry-exporter-datadog


Usage
-----

The Datadog exporter provides a span processor that must be added along with the
exporter. In addition, a formatter is provided to handle propagation of trace
context between OpenTelemetry-instrumented and Datadog-instrumented services in
a distributed trace.

.. code:: python

    from opentelemetry.propagate import set_global_textmap
    from opentelemetry import trace
    from opentelemetry.exporter.datadog import DatadogExportSpanProcessor, DatadogSpanExporter
    from opentelemetry.exporter.datadog.propagator import DatadogFormat
    from opentelemetry.sdk.trace import TracerProvider

    trace.set_tracer_provider(TracerProvider())
    tracer = trace.get_tracer(__name__)

    exporter = DatadogSpanExporter(
        agent_url="http://agent:8126", service="my-helloworld-service"
    )

    span_processor = DatadogExportSpanProcessor(exporter)
    trace.get_tracer_provider().add_span_processor(span_processor)

    # Optional: use Datadog format for propagation in distributed traces
    set_global_textmap(DatadogFormat())

    with tracer.start_as_current_span("foo"):
        print("Hello world!")


Examples
--------

The `docs/examples/datadog_exporter`_ includes examples for using the Datadog
exporter with OpenTelemetry instrumented applications.

API
---
.. _Datadog: https://www.datadoghq.com/
.. _OpenTelemetry: https://github.com/open-telemetry/opentelemetry-python/
.. _docs/examples/datadog_exporter: https://github.com/open-telemetry/opentelemetry-python/tree/main/docs/examples/datadog_exporter
"""
# pylint: disable=import-error

from .exporter import DatadogSpanExporter
from .spanprocessor import DatadogExportSpanProcessor

__all__ = ["DatadogExportSpanProcessor", "DatadogSpanExporter"]
