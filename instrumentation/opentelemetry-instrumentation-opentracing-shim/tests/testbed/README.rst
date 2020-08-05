
Testbed suite for the OpenTelemetry-OpenTracing Bridge
======================================================

Testbed suite designed to test the API changes.

Build and test.
---------------

.. code-block:: sh

   tox -e py37-test-opentracing-shim

Alternatively, due to the organization of the suite, it's possible to run directly the tests using ``py.test``\ :

.. code-block:: sh

       py.test -s testbed/test_multiple_callbacks/test_threads.py

Tested frameworks
-----------------

Currently the examples cover ``threading`` and ``asyncio``.

List of patterns
----------------


* `Active Span replacement <test_active_span_replacement>`_ - Start an isolated task and query for its results in another task/thread.
* `Client-Server <test_client_server>`_ - Typical client-server example.
* `Common Request Handler <test_common_request_handler>`_ - One request handler for all requests.
* `Late Span finish <test_late_span_finish>`_ - Late parent ``Span`` finish.
* `Multiple callbacks <test_multiple_callbacks>`_ - Multiple callbacks spawned at the same time.
* `Nested callbacks <test_nested_callbacks>`_ - One callback at a time, defined in a pipeline fashion.
* `Subtask Span propagation <test_subtask_span_propagation>`_ - ``Span`` propagation for subtasks/coroutines.

Adding new patterns
-------------------

A new pattern is composed of a directory under *testbed* with the *test_* prefix, and containing the files for each platform, also with the *test_* prefix:

.. code-block::

   testbed/
     test_new_pattern/
       test_threads.py
       test_asyncio.py
