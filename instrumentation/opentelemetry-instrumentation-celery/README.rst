OpenTelemetry Celery Instrumentation
====================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-celery.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-celery/

Instrumentation for Celery.


Installation
------------

::

    pip install opentelemetry-instrumentation-celery

Usage
-----

* Start broker backend

::
    docker run -p 5672:5672 rabbitmq


* Run instrumented task

.. code-block:: python

    from opentelemetry import trace
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchExportSpanProcessor
    from opentelemetry.instrumentation.celery import CeleryInstrumentor

    from celery import Celery
    from celery.signals import worker_process_init

    @worker_process_init.connect(weak=False)
    def init_celery_tracing(*args, **kwargs):
        trace.set_tracer_provider(TracerProvider())
        span_processor = BatchExportSpanProcessor(ConsoleSpanExporter())
        trace.get_tracer_provider().add_span_processor(span_processor)
        CeleryInstrumentor().instrument()

    app = Celery("tasks", broker="amqp://localhost")

    @app.task
    def add(x, y):
        return x + y

    add.delay(42, 50)


Setting up tracing 
--------------------

When tracing a celery worker process, tracing and instrumention both must be initialized after the celery worker
process is initialized. This is required for any tracing components that might use threading to work correctly
such as the BatchExportSpanProcessor. Celery provides a signal called ``worker_process_init`` that can be used to
accomplish this as shown in the example above.

References
----------
* `OpenTelemetry Celery Instrumentation <https://opentelemetry-python.readthedocs.io/en/latest/instrumentation/celery/celery.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_

