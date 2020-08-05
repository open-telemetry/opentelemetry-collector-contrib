
Active Span replacement example.
================================

This example shows a ``Span`` being created and then passed to an asynchronous task, which will temporary activate it to finish its processing, and further restore the previously active ``Span``.

``threading`` implementation:

.. code-block:: python

   # Create a new Span for this task
   with self.tracer.start_active_span("task"):

       with self.tracer.scope_manager.activate(span, True):
              # Simulate work strictly related to the initial Span
              pass

       # Use the task span as parent of a new subtask
       with self.tracer.start_active_span("subtask"):
              pass
