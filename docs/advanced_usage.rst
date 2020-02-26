Advanced Usage
==============

Agent Configuration
-------------------

If the Datadog Agent is on a separate host from your application, you can modify
the default ``ddtrace.tracer`` object to utilize another hostname and port. Here
is a small example showcasing this::

    from ddtrace import tracer

    tracer.configure(hostname=<YOUR_HOST>, port=<YOUR_PORT>, https=<True/False>)

By default, these will be set to ``localhost``, ``8126``, and ``False`` respectively.

You can also use a Unix Domain Socket to connect to the agent::

    from ddtrace import tracer

    tracer.configure(uds_path="/path/to/socket")


Distributed Tracing
-------------------

To trace requests across hosts, the spans on the secondary hosts must be linked together by setting `trace_id`, `parent_id` and `sampling_priority`.

- On the server side, it means to read propagated attributes and set them to the active tracing context.
- On the client side, it means to propagate the attributes, commonly as a header/metadata.

`ddtrace` already provides default propagators but you can also implement your own.

Web Frameworks
^^^^^^^^^^^^^^

Some web framework integrations support distributed tracing out of the box.

Supported web frameworks:


+-------------------+---------+
| Framework/Library | Enabled |
+===================+=========+
| :ref:`aiohttp`    | True    |
+-------------------+---------+
| :ref:`bottle`     | True    |
+-------------------+---------+
| :ref:`django`     | True    |
+-------------------+---------+
| :ref:`falcon`     | True    |
+-------------------+---------+
| :ref:`flask`      | True    |
+-------------------+---------+
| :ref:`pylons`     | True    |
+-------------------+---------+
| :ref:`pyramid`    | True    |
+-------------------+---------+
| :ref:`requests`   | True    |
+-------------------+---------+
| :ref:`tornado`    | True    |
+-------------------+---------+


HTTP Client
^^^^^^^^^^^

For distributed tracing to work, necessary tracing information must be passed
alongside a request as it flows through the system. When the request is handled
on the other side, the metadata is retrieved and the trace can continue.

To propagate the tracing information, HTTP headers are used to transmit the
required metadata to piece together the trace.

.. autoclass:: ddtrace.propagation.http.HTTPPropagator
    :members:

Custom
^^^^^^

You can manually propagate your tracing context over your RPC protocol. Here is
an example assuming that you have `rpc.call` function that call a `method` and
propagate a `rpc_metadata` dictionary over the wire::


    # Implement your own context propagator
    class MyRPCPropagator(object):
        def inject(self, span_context, rpc_metadata):
            rpc_metadata.update({
                'trace_id': span_context.trace_id,
                'span_id': span_context.span_id,
                'sampling_priority': span_context.sampling_priority,
            })

        def extract(self, rpc_metadata):
            return Context(
                trace_id=rpc_metadata['trace_id'],
                span_id=rpc_metadata['span_id'],
                sampling_priority=rpc_metadata['sampling_priority'],
            )

    # On the parent side
    def parent_rpc_call():
        with tracer.trace("parent_span") as span:
            rpc_metadata = {}
            propagator = MyRPCPropagator()
            propagator.inject(span.context, rpc_metadata)
            method = "<my rpc method>"
            rpc.call(method, metadata)

    # On the child side
    def child_rpc_call(method, rpc_metadata):
        propagator = MyRPCPropagator()
        context = propagator.extract(rpc_metadata)
        tracer.context_provider.activate(context)

        with tracer.trace("child_span") as span:
            span.set_meta('my_rpc_method', method)


Sampling
--------

.. _`Priority Sampling`:

Priority Sampling
^^^^^^^^^^^^^^^^^

To learn about what sampling is check out our documentation `here
<https://docs.datadoghq.com/tracing/getting_further/trace_sampling_and_storage/#priority-sampling-for-distributed-tracing>`_.

By default priorities are set on a trace by a sampler. The sampler can set the
priority to the following values:

- ``AUTO_REJECT``: the sampler automatically rejects the trace
- ``AUTO_KEEP``: the sampler automatically keeps the trace

Priority sampling is enabled by default.
When enabled, the sampler will automatically assign a priority to your traces,
depending on their service and volume.
This ensures that your sampled distributed traces will be complete.

You can also set this priority manually to either drop an uninteresting trace or
to keep an important one.
To do this, set the ``context.sampling_priority`` to one of the following:

- ``USER_REJECT``: the user asked to reject the trace
- ``USER_KEEP``: the user asked to keep the trace

When not using distributed tracing, you may change the priority at any time, as
long as the trace is not finished yet.
But it has to be done before any context propagation (fork, RPC calls) to be
effective in a distributed context.
Changing the priority after context has been propagated causes different parts
of a distributed trace to use different priorities. Some parts might be kept,
some parts might be rejected, and this can cause the trace to be partially
stored and remain incomplete.

If you change the priority, we recommend you do it as soon as possible, when the
root span has just been created::

    from ddtrace.ext.priority import USER_REJECT, USER_KEEP

    context = tracer.context_provider.active()

    # indicate to not keep the trace
    context.sampling_priority = USER_REJECT


Client Sampling
^^^^^^^^^^^^^^^

Client sampling enables the sampling of traces before they are sent to the
Agent. This can provide some performance benefit as the traces will be
dropped in the client.

The ``RateSampler`` randomly samples a percentage of traces::

    from ddtrace.sampler import RateSampler

    # Sample rate is between 0 (nothing sampled) to 1 (everything sampled).
    # Keep 20% of the traces.
    sample_rate = 0.2
    tracer.sampler = RateSampler(sample_rate)


Trace Search & Analytics
------------------------

Use `Trace Search & Analytics <https://docs.datadoghq.com/tracing/visualization/search/>`_ to filter application performance metrics and APM Events by user-defined tags. An APM event is generated every time a trace is generated.

Enabling APM events for all web frameworks can be accomplished by setting the environment variable ``DD_TRACE_ANALYTICS_ENABLED=true``:

* :ref:`aiohttp`
* :ref:`bottle`
* :ref:`django`
* :ref:`falcon`
* :ref:`flask`
* :ref:`molten`
* :ref:`pylons`
* :ref:`pyramid`
* :ref:`requests`
* :ref:`tornado`


For most libraries, APM events can be enabled with the environment variable ``DD_{INTEGRATION}_ANALYTICS_ENABLED=true``:

+----------------------+----------------------------------------+
|       Library        |          Environment Variable          |
+======================+========================================+
| :ref:`aiobotocore`   | ``DD_AIOBOTOCORE_ANALYTICS_ENABLED``   |
+----------------------+----------------------------------------+
| :ref:`aiopg`         | ``DD_AIOPG_ANALYTICS_ENABLED``         |
+----------------------+----------------------------------------+
| :ref:`boto`          | ``DD_BOTO_ANALYTICS_ENABLED``          |
+----------------------+----------------------------------------+
| :ref:`botocore`      | ``DD_BOTOCORE_ANALYTICS_ENABLED``      |
+----------------------+----------------------------------------+
| :ref:`bottle`        | ``DD_BOTTLE_ANALYTICS_ENABLED``        |
+----------------------+----------------------------------------+
| :ref:`cassandra`     | ``DD_CASSANDRA_ANALYTICS_ENABLED``     |
+----------------------+----------------------------------------+
| :ref:`elasticsearch` | ``DD_ELASTICSEARCH_ANALYTICS_ENABLED`` |
+----------------------+----------------------------------------+
| :ref:`falcon`        | ``DD_FALCON_ANALYTICS_ENABLED``        |
+----------------------+----------------------------------------+
| :ref:`flask`         | ``DD_FLASK_ANALYTICS_ENABLED``         |
+----------------------+----------------------------------------+
| :ref:`flask_cache`   | ``DD_FLASK_CACHE_ANALYTICS_ENABLED``   |
+----------------------+----------------------------------------+
| :ref:`grpc`          | ``DD_GRPC_ANALYTICS_ENABLED``          |
+----------------------+----------------------------------------+
| :ref:`httplib`       | ``DD_HTTPLIB_ANALYTICS_ENABLED``       |
+----------------------+----------------------------------------+
| :ref:`kombu`         | ``DD_KOMBU_ANALYTICS_ENABLED``         |
+----------------------+----------------------------------------+
| :ref:`molten`        | ``DD_MOLTEN_ANALYTICS_ENABLED``        |
+----------------------+----------------------------------------+
| :ref:`pylibmc`       | ``DD_PYLIBMC_ANALYTICS_ENABLED``       |
+----------------------+----------------------------------------+
| :ref:`pylons`        | ``DD_PYLONS_ANALYTICS_ENABLED``        |
+----------------------+----------------------------------------+
| :ref:`pymemcache`    | ``DD_PYMEMCACHE_ANALYTICS_ENABLED``    |
+----------------------+----------------------------------------+
| :ref:`pymongo`       | ``DD_PYMONGO_ANALYTICS_ENABLED``       |
+----------------------+----------------------------------------+
| :ref:`redis`         | ``DD_REDIS_ANALYTICS_ENABLED``         |
+----------------------+----------------------------------------+
| :ref:`rediscluster`  | ``DD_REDISCLUSTER_ANALYTICS_ENABLED``  |
+----------------------+----------------------------------------+
| :ref:`sqlalchemy`    | ``DD_SQLALCHEMY_ANALYTICS_ENABLED``    |
+----------------------+----------------------------------------+
| :ref:`vertica`       | ``DD_VERTICA_ANALYTICS_ENABLED``       |
+----------------------+----------------------------------------+

For datastore libraries that extend another, use the setting for the underlying library:

+------------------------+----------------------------------+
|        Library         |       Environment Variable       |
+========================+==================================+
| :ref:`mongoengine`     | ``DD_PYMONGO_ANALYTICS_ENABLED`` |
+------------------------+----------------------------------+
| :ref:`mysql-connector` | ``DD_DBAPI2_ANALYTICS_ENABLED``  |
+------------------------+----------------------------------+
| :ref:`mysqldb`         | ``DD_DBAPI2_ANALYTICS_ENABLED``  |
+------------------------+----------------------------------+
| :ref:`psycopg2`        | ``DD_DBAPI2_ANALYTICS_ENABLED``  |
+------------------------+----------------------------------+
| :ref:`pymysql`         | ``DD_DBAPI2_ANALYTICS_ENABLED``  |
+------------------------+----------------------------------+
| :ref:`sqllite`         | ``DD_DBAPI2_ANALYTICS_ENABLED``  |
+------------------------+----------------------------------+

Where environment variables are not used for configuring the tracer, the instructions for configuring trace analytics is provided in the library documentation:

* :ref:`aiohttp`
* :ref:`django`
* :ref:`pyramid`
* :ref:`requests`
* :ref:`tornado`

Resolving deprecation warnings
------------------------------
Before upgrading, it’s a good idea to resolve any deprecation warnings raised by your project.
These warnings must be fixed before upgrading, otherwise the ``ddtrace`` library
will not work as expected. Our deprecation messages include the version where
the behavior is altered or removed.

In Python, deprecation warnings are silenced by default. To enable them you may
add the following flag or environment variable::

    $ python -Wall app.py

    # or

    $ PYTHONWARNINGS=all python app.py


Trace Filtering
---------------

It is possible to filter or modify traces before they are sent to the Agent by
configuring the tracer with a filters list. For instance, to filter out
all traces of incoming requests to a specific url::

    Tracer.configure(settings={
        'FILTERS': [
            FilterRequestsOnUrl(r'http://test\.example\.com'),
        ],
    })

All the filters in the filters list will be evaluated sequentially
for each trace and the resulting trace will either be sent to the Agent or
discarded depending on the output.

**Use the standard filters**

The library comes with a ``FilterRequestsOnUrl`` filter that can be used to
filter out incoming requests to specific urls:

.. autoclass:: ddtrace.filters.FilterRequestsOnUrl
    :members:

**Write a custom filter**

Creating your own filters is as simple as implementing a class with a
``process_trace`` method and adding it to the filters parameter of
Tracer.configure. process_trace should either return a trace to be fed to the
next step of the pipeline or ``None`` if the trace should be discarded::

    class FilterExample(object):
        def process_trace(self, trace):
            # write here your logic to return the `trace` or None;
            # `trace` instance is owned by the thread and you can alter
            # each single span or the whole trace if needed

    # And then instantiate it with
    filters = [FilterExample()]
    Tracer.configure(settings={'FILTERS': filters})

(see filters.py for other example implementations)

.. _`Logs Injection`:

Logs Injection
--------------

.. automodule:: ddtrace.contrib.logging

HTTP layer
----------

Query String Tracing
^^^^^^^^^^^^^^^^^^^^

It is possible to store the query string of the URL — the part after the ``?``
in your URL — in the ``url.query.string`` tag.

Configuration can be provided both at the global level and at the integration level.

Examples::

    from ddtrace import config

    # Global config
    config.http.trace_query_string = True

    # Integration level config, e.g. 'falcon'
    config.falcon.http.trace_query_string = True

..  _http-headers-tracing:

Headers tracing
^^^^^^^^^^^^^^^


For a selected set of integrations, it is possible to store http headers from both requests and responses in tags.

Configuration can be provided both at the global level and at the integration level.

Examples::

    from ddtrace import config

    # Global config
    config.trace_headers([
        'user-agent',
        'transfer-encoding',
    ])

    # Integration level config, e.g. 'falcon'
    config.falcon.http.trace_headers([
        'user-agent',
        'some-other-header',
    ])

The following rules apply:
  - headers configuration is based on a whitelist. If a header does not appear in the whitelist, it won't be traced.
  - headers configuration is case-insensitive.
  - if you configure a specific integration, e.g. 'requests', then such configuration overrides the default global
    configuration, only for the specific integration.
  - if you do not configure a specific integration, then the default global configuration applies, if any.
  - if no configuration is provided (neither global nor integration-specific), then headers are not traced.

Once you configure your application for tracing, you will have the headers attached to the trace as tags, with a
structure like in the following example::

    http {
      method  GET
      request {
        headers {
          user_agent  my-app/0.0.1
        }
      }
      response {
        headers {
          transfer_encoding  chunked
        }
      }
      status_code  200
      url  https://api.github.com/events
    }


.. _adv_opentracing:

OpenTracing
-----------


The Datadog opentracer can be configured via the ``config`` dictionary
parameter to the tracer which accepts the following described fields. See below
for usage.

+---------------------+----------------------------------------+---------------+
|  Configuration Key  |              Description               | Default Value |
+=====================+========================================+===============+
| `enabled`           | enable or disable the tracer           | `True`        |
+---------------------+----------------------------------------+---------------+
| `debug`             | enable debug logging                   | `False`       |
+---------------------+----------------------------------------+---------------+
| `agent_hostname`    | hostname of the Datadog agent to use   | `localhost`   |
+---------------------+----------------------------------------+---------------+
| `agent_https`       | use https to connect to the agent      | `False`       |
+---------------------+----------------------------------------+---------------+
| `agent_port`        | port the Datadog agent is listening on | `8126`        |
+---------------------+----------------------------------------+---------------+
| `global_tags`       | tags that will be applied to each span | `{}`          |
+---------------------+----------------------------------------+---------------+
| `sampler`           | see `Sampling`_                        | `AllSampler`  |
+---------------------+----------------------------------------+---------------+
| `priority_sampling` | see `Priority Sampling`_               | `True`        |
+---------------------+----------------------------------------+---------------+
| `settings`          | see `Advanced Usage`_                  | `{}`          |
+---------------------+----------------------------------------+---------------+


Usage
^^^^^

**Manual tracing**

To explicitly trace::

  import time
  import opentracing
  from ddtrace.opentracer import Tracer, set_global_tracer

  def init_tracer(service_name):
      config = {
        'agent_hostname': 'localhost',
        'agent_port': 8126,
      }
      tracer = Tracer(service_name, config=config)
      set_global_tracer(tracer)
      return tracer

  def my_operation():
    span = opentracing.tracer.start_span('my_operation_name')
    span.set_tag('my_interesting_tag', 'my_interesting_value')
    time.sleep(0.05)
    span.finish()

  init_tracer('my_service_name')
  my_operation()

**Context Manager Tracing**

To trace a function using the span context manager::

  import time
  import opentracing
  from ddtrace.opentracer import Tracer, set_global_tracer

  def init_tracer(service_name):
      config = {
        'agent_hostname': 'localhost',
        'agent_port': 8126,
      }
      tracer = Tracer(service_name, config=config)
      set_global_tracer(tracer)
      return tracer

  def my_operation():
    with opentracing.tracer.start_span('my_operation_name') as span:
      span.set_tag('my_interesting_tag', 'my_interesting_value')
      time.sleep(0.05)

  init_tracer('my_service_name')
  my_operation()

See our tracing trace-examples_ repository for concrete, runnable examples of
the Datadog opentracer.

.. _trace-examples: https://github.com/DataDog/trace-examples/tree/master/python

See also the `Python OpenTracing`_ repository for usage of the tracer.

.. _Python OpenTracing: https://github.com/opentracing/opentracing-python


**Alongside Datadog tracer**

The Datadog OpenTracing tracer can be used alongside the Datadog tracer. This
provides the advantage of providing tracing information collected by
``ddtrace`` in addition to OpenTracing.  The simplest way to do this is to use
the :ref:`ddtrace-run<ddtracerun>` command to invoke your OpenTraced
application.


**Opentracer API**

.. autoclass:: ddtrace.opentracer.Tracer
    :members:
    :special-members: __init__


.. _ddtracerun:

``ddtrace-run``
---------------

``ddtrace-run`` will trace :ref:`supported<Supported Libraries>` web frameworks
and database modules without the need for changing your code::

  $ ddtrace-run -h

  Execute the given Python program, after configuring it
  to emit Datadog traces.

  Append command line arguments to your program as usual.

  Usage: [ENV_VARS] ddtrace-run <my_program>


The available environment variables for ``ddtrace-run`` are:

* ``DATADOG_TRACE_ENABLED=true|false`` (default: true): Enable web framework and
  library instrumentation. When false, your application code will not generate
  any traces.
* ``DATADOG_ENV`` (no default): Set an application's environment e.g. ``prod``,
  ``pre-prod``, ``stage``
* ``DATADOG_TRACE_DEBUG=true|false`` (default: false): Enable debug logging in
  the tracer
* ``DATADOG_SERVICE_NAME`` (no default): override the service name to be used
  for this program. This value is passed through when setting up middleware for
  web framework integrations (e.g. pylons, flask, django). For tracing without a
  web integration, prefer setting the service name in code.
* ``DATADOG_PATCH_MODULES=module:patch,module:patch...`` e.g.
  ``boto:true,redis:false``: override the modules patched for this execution of
  the program (default: none)
* ``DATADOG_TRACE_AGENT_HOSTNAME=localhost``: override the address of the trace
  agent host that the default tracer will attempt to submit to  (default:
  ``localhost``)
* ``DATADOG_TRACE_AGENT_PORT=8126``: override the port that the default tracer
  will submit to  (default: 8126)
* ``DATADOG_PRIORITY_SAMPLING`` (default: true): enables :ref:`Priority
  Sampling`
* ``DD_LOGS_INJECTION`` (default: false): enables :ref:`Logs Injection`

``ddtrace-run`` respects a variety of common entrypoints for web applications:

- ``ddtrace-run python my_app.py``
- ``ddtrace-run python manage.py runserver``
- ``ddtrace-run gunicorn myapp.wsgi:application``
- ``ddtrace-run uwsgi --http :9090 --wsgi-file my_app.py``


Pass along command-line arguments as your program would normally expect them::

$ ddtrace-run gunicorn myapp.wsgi:application --max-requests 1000 --statsd-host localhost:8125

If you're running in a Kubernetes cluster and still don't see your traces, make
sure your application has a route to the tracing Agent. An easy way to test
this is with a::

$ pip install ipython
$ DATADOG_TRACE_DEBUG=true ddtrace-run ipython

Because iPython uses SQLite, it will be automatically instrumented and your
traces should be sent off. If an error occurs, a message will be displayed in
the console, and changes can be made as needed.


API
---

``Tracer``
^^^^^^^^^^
.. autoclass:: ddtrace.Tracer
    :members:
    :special-members: __init__


``Span``
^^^^^^^^
.. autoclass:: ddtrace.Span
    :members:
    :special-members: __init__

``Pin``
^^^^^^^
.. autoclass:: ddtrace.Pin
    :members:
    :special-members: __init__

.. _patch_all:

``patch_all``
^^^^^^^^^^^^^

.. autofunction:: ddtrace.monkey.patch_all

``patch``
^^^^^^^^^
.. autofunction:: ddtrace.monkey.patch

.. toctree::
   :maxdepth: 2
