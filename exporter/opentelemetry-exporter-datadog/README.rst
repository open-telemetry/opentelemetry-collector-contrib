OpenTelemetry Datadog Span Exporter
===================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-exporter-datadog.svg
   :target: https://pypi.org/project/opentelemetry-exporter-datadog/

This library allows to export tracing data to `Datadog
<https://www.datadoghq.com/>`_. OpenTelemetry span event and links are not
supported.

.. warning:: This exporter has been deprecated. To export your OTLP traces from OpenTelemetry SDK directly to Datadog Agent, please refer to `OTLP Ingest in Datadog Agent <https://docs.datadoghq.com/tracing/setup_overview/open_standards/#otlp-ingest-in-datadog-agent>`_ .


Installation
------------

::

    pip install opentelemetry-exporter-datadog


.. _Datadog: https://www.datadoghq.com/
.. _OpenTelemetry: https://github.com/open-telemetry/opentelemetry-python/


References
----------

* `Datadog <https://www.datadoghq.com/>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
