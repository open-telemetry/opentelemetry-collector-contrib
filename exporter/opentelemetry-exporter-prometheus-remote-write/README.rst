OpenTelemetry Prometheus Remote Write Exporter
==============================================

This library allows exporting metric data to `Prometheus Remote Write Integrated Backends
<https://prometheus.io/docs/operating/integrations/>`_. Latest `types.proto
<https://github.com/prometheus/prometheus/blob/master/prompb/types.proto>` and `remote.proto
<https://github.com/prometheus/prometheus/blob/master/prompb/remote.proto>` Protocol Buffers 
used to create WriteRequest objects were taken from Prometheus repository. Development is 
currently in progress.

Installation
------------

::

    pip install opentelemetry-exporter-prometheus-remote-write


.. _Prometheus: https://prometheus.io/
.. _OpenTelemetry: https://github.com/open-telemetry/opentelemetry-python/


References
----------

* `Prometheus <https://prometheus.io/>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
