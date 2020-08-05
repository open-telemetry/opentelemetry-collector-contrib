
Late Span finish example.
=========================

This example shows a ``Span`` for a top-level operation, with independent, unknown lifetime, acting as parent of a few asynchronous subtasks (which must re-activate it but not finish it).

.. code-block:: python

       # Fire away a few subtasks, passing a parent Span whose lifetime
       # is not tied at all to the children.
       def submit_subtasks(self, parent_span):
           def task(name, interval):
               with self.tracer.scope_manager.activate(parent_span, False):
                   with self.tracer.start_active_span(name):
                       time.sleep(interval)

           self.executor.submit(task, "task1", 0.1)
           self.executor.submit(task, "task2", 0.3)
