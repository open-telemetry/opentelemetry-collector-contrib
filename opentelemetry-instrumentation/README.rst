OpenTelemetry Instrumentation
=============================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation/

Installation
------------

::

    pip install opentelemetry-instrumentation


This package provides a couple of commands that help automatically instruments a program:

.. note::
    You need to install a distro package to get auto instrumentation working. The ``opentelemetry-distro``
    package contains the default distro and automatically configures some of the common options for users.
    For more info about ``opentelemetry-distro`` check `here <https://opentelemetry-python.readthedocs.io/en/latest/examples/distro/README.html>`__
    ::

        pip install opentelemetry-distro[otlp]


opentelemetry-bootstrap
-----------------------

::

    opentelemetry-bootstrap --action=install|requirements

This commands inspects the active Python site-packages and figures out which
instrumentation packages the user might want to install. By default it prints out
a list of the suggested instrumentation packages which can be added to a requirements.txt
file. It also supports installing the suggested packages when run with :code:`--action=install`
flag.


opentelemetry-instrument
------------------------

::

    opentelemetry-instrument python program.py

The instrument command will try to automatically detect packages used by your python program
and when possible, apply automatic tracing instrumentation on them. This means your program
will get automatic distributed tracing for free without having to make any code changes
at all. This will also configure a global tracer and tracing exporter without you having to
make any code changes. By default, the instrument command will use the OTLP exporter but
this can be overridden when needed.

The command supports the following configuration options as CLI arguments and environment vars:


* ``--traces_exporter`` or ``OTEL_TRACES_EXPORTER``
* ``--metrics_exporter`` or ``OTEL_METRICS_EXPORTER``

Used to specify which trace exporter to use. Can be set to one or more of the well-known exporter
names (see below).

    - Defaults to `otlp`.
    - Can be set to `none` to disable automatic tracer initialization.

You can pass multiple values to configure multiple exporters e.g, ``zipkin,prometheus``

Well known trace exporter names:

    - jaeger_proto
    - jaeger_thrift
    - opencensus
    - zipkin_json
    - zipkin_proto
    - otlp
    - otlp_proto_grpc (`deprecated`)
    - otlp_proto_http (`deprecated`)

Note: The default transport protocol for ``otlp`` is gRPC.
HTTP is currently supported for traces only, and should be set using ``OTEL_EXPORTER_OTLP_TRACES_PROTOCOL=http/protobuf``

* ``--id-generator`` or ``OTEL_PYTHON_ID_GENERATOR``

Used to specify which IDs Generator to use for the global Tracer Provider. By default, it
will use the random IDs generator.

The code in ``program.py`` needs to use one of the packages for which there is
an OpenTelemetry integration. For a list of the available integrations please
check `here <https://opentelemetry-python.readthedocs.io/en/stable/index.html#integrations>`_

* ``OTEL_PYTHON_DISABLED_INSTRUMENTATIONS``

If set by the user, opentelemetry-instrument will read this environment variable to disable specific instrumentations.
e.g OTEL_PYTHON_DISABLED_INSTRUMENTATIONS = "requests,django"


Examples
^^^^^^^^

::

    opentelemetry-instrument --traces_exporter otlp flask run --port=3000

The above command will pass ``--traces_exporter otlp`` to the instrument command and ``--port=3000`` to ``flask run``.

::

    opentelemetry-instrument --traces_exporter zipkin_json,otlp celery -A tasks worker --loglevel=info

The above command will configure global trace provider, attach zipkin and otlp exporters to it and then
start celery with the rest of the arguments.

::

    opentelemetry-instrument --id_generator random flask run --port=3000

The above command will configure the global trace provider to use the Random IDs Generator, and then
pass ``--port=3000`` to ``flask run``.

References
----------

* `OpenTelemetry Project <https://opentelemetry.io/>`_
