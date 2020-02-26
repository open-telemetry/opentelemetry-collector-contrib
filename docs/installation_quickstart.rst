.. include:: ./shared.rst


.. _Installation:

Installation + Quickstart
=========================

Before installing be sure to read through the `setup documentation`_ to ensure
your environment is ready to receive traces.


Installation
------------

Install with :code:`pip`::

$ pip install ddtrace

We strongly suggest pinning the version of the library you deploy.

Quickstart
----------

Getting started with ``ddtrace`` is as easy as prefixing your python
entry-point command with ``ddtrace-run``.

For example if you start your application with ``python app.py`` then run::

  $ ddtrace-run python app.py

For more advanced usage of ``ddtrace-run`` refer to the documentation :ref:`here<ddtracerun>`.

To find out how to trace your own code manually refer to the documentation :ref:`here<basic usage>`.


Configuration
~~~~~~~~~~~~~

You can configure some parameters of the library by setting environment
variable before starting your application and importing the library:

.. list-table::
   :header-rows: 1
   :widths: 1 1 2

   * - Configuration Variable
     - Configuration Type
     - Default Value
     - Value Description
   * - ``DD_TRACE_AGENT_URL``
     - URL
     - ``http://localhost:8126``
     - The URL to use to connect the Datadog agent. The url can starts with
       ``http://`` to connect using HTTP or with ``unix://`` to use a Unix
       Domain Socket.


OpenTracing
-----------

``ddtrace`` also provides an OpenTracing API to the Datadog tracer so
that you can use the Datadog tracer in your OpenTracing-compatible
applications.

Installation
^^^^^^^^^^^^

Include OpenTracing with ``ddtrace``::

  $ pip install ddtrace[opentracing]

To include the OpenTracing dependency in your project with ``ddtrace``, ensure
you have the following in ``setup.py``::

    install_requires=[
        "ddtrace[opentracing]",
    ],

Configuration
^^^^^^^^^^^^^

The OpenTracing convention for initializing a tracer is to define an
initialization method that will configure and instantiate a new tracer and
overwrite the global ``opentracing.tracer`` reference.

Typically this method looks something like::

    from ddtrace.opentracer import Tracer, set_global_tracer

    def init_tracer(service_name):
        """
        Initialize a new Datadog opentracer and set it as the
        global tracer.

        This overwrites the opentracing.tracer reference.
        """
        config = {
          'agent_hostname': 'localhost',
          'agent_port': 8126,
        }
        tracer = Tracer(service_name, config=config)
        set_global_tracer(tracer)
        return tracer

For more advanced usage of OpenTracing in ``ddtrace`` refer to the
documentation :ref:`here<adv_opentracing>`.
