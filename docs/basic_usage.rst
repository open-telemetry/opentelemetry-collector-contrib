.. _`basic usage`:

Basic Usage
===========

With ``ddtrace`` installed, the application can be instrumented.


Auto Instrumentation
--------------------

``ddtrace-run``
^^^^^^^^^^^^^^^

Python applications can easily be instrumented with ``ddtrace`` by using the
included ``ddtrace-run`` command. Simply prefix your Python execution command
with ``ddtrace-run`` in order to auto-instrument the libraries in your
application.

For example, if the command to run your application is::

$ python app.py

then to auto-instrument using Datadog, the corresponding command is::

$ ddtrace-run python app.py

For more advanced usage of ``ddtrace-run`` refer to the documentation :ref:`here<ddtracerun>`.

``patch_all``
^^^^^^^^^^^^^

To manually invoke the automatic instrumentation use ``patch_all``::

  from ddtrace import patch_all
  patch_all()

To toggle instrumentation for a particular module::

  from ddtrace import patch_all
  patch_all(redis=False, cassandra=False)

By default all supported libraries will be patched when
``patch_all`` is invoked.

**Note:** To ensure that the supported libraries are instrumented properly in
the application, they must be patched *prior* to being imported. So make sure
to call ``patch_all`` *before* importing libraries that are to be instrumented.

More information about ``patch_all`` is available in our :ref:`patch_all` API
documentation.


Manual Instrumentation
----------------------

If you would like to extend the functionality of the ``ddtrace`` library or gain
finer control over instrumenting your application, several techniques are
provided by the library.

Decorator
^^^^^^^^^

``ddtrace`` provides a decorator that can be used to trace a particular method
in your application::

  @tracer.wrap()
  def business_logic():
    """A method that would be of interest to trace."""
    # ...
    # ...

API details of the decorator can be found here :py:meth:`ddtrace.Tracer.wrap`.

Context Manager
^^^^^^^^^^^^^^^

To trace an arbitrary block of code, you can use :py:meth:`ddtrace.Tracer.trace` 
that returns a :py:mod:`ddtrace.Span` which can be used as a context manager::

  # trace some interesting operation
  with tracer.trace('interesting.operations'):
    # do some interesting operation(s)
    # ...
    # ...

Further API details can be found here :py:meth:`ddtrace.Tracer`.

Using the API
^^^^^^^^^^^^^

If the above methods are still not enough to satisfy your tracing needs, a
manual API is provided which will allow you to start and finish spans however
you may require::

  span = tracer.trace('operations.of.interest')

  # do some operation(s) of interest in between

  # NOTE: make sure to call span.finish() or the entire trace will not be sent
  # to Datadog
  span.finish()

API details of the decorator can be found here:

- :py:meth:`ddtrace.Tracer.trace`
- :py:meth:`ddtrace.Span.finish`.
