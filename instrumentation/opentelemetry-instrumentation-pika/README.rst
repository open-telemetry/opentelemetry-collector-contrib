OpenTelemetry pika Instrumentation
==================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-pika.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-pika/

This library allows tracing requests made by the pika library.

Installation
------------

::

    pip install opentelemetry-instrumentation-pika

Usage
-----

* Start broker backend

.. code-block:: python

    docker run -p 5672:5672 rabbitmq

* Run instrumented task

.. code-block:: python

    import pika
    from opentelemetry.instrumentation.pika import PikaInstrumentor

    PikaInstrumentor().instrument()

    connection = pika.BlockingConnection(pika.URLParameters('amqp://localhost'))
    channel = connection.channel()
    channel.queue_declare(queue='hello')
    channel.basic_publish(exchange='', routing_key='hello', body=b'Hello World!')

* PikaInstrumentor also supports instrumentation of a single channel

.. code-block:: python

    import pika
    from opentelemetry.instrumentation.pika import PikaInstrumentor

    connection = pika.BlockingConnection(pika.URLParameters('amqp://localhost'))
    channel = connection.channel()
    channel.queue_declare(queue='hello')

    pika_instrumentation = PikaInstrumentor()
    pika_instrumentation.instrument_channel(channel=channel)


    channel.basic_publish(exchange='', routing_key='hello', body=b'Hello World!')

    pika_instrumentation.uninstrument_channel(channel=channel)

* PikaInstrumentor also supports instrumentation without creating an object, and receiving a tracer_provider

.. code-block:: python

    PikaInstrumentor.instrument_channel(channel, tracer_provider=tracer_provider)

References
----------

* `OpenTelemetry pika/ Tracing <https://opentelemetry-python-contrib.readthedocs.io/en/latest/instrumentation/pika/pika.html>`_
* `OpenTelemetry Project <https://opentelemetry.io/>`_
