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

    from opentelemetry.instrumentation.celery import CeleryInstrumentor

    CeleryInstrumentor().instrument()

    from celery import Celery

    app = Celery("tasks", broker="amqp://localhost")

    @app.task
    def add(x, y):
        return x + y

    add.delay(42, 50)

References
----------
* `OpenTelemetry Celery Instrumentation <https://opentelemetry-python.readthedocs.io/en/latest/ext/celery/celery.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_

