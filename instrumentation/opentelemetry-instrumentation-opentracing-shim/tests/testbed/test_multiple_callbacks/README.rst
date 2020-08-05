
Multiple callbacks example.
===========================

This example shows a ``Span`` created for a top-level operation, covering a set of asynchronous operations (representing callbacks), and have this ``Span`` finished when **all** of them have been executed.

``Client.send()`` is used to create a new asynchronous operation (callback), and in turn every operation both restores the active ``Span``, and creates a child ``Span`` (useful for measuring the performance of each callback).

Implementation details:


* For ``threading``, a thread-safe counter is put in each ``Span`` to keep track of the pending callbacks, and call ``Span.finish()`` when the count becomes 0.
* For ``asyncio`` the children corotuines representing the subtasks are simply yielded over, so no counter is needed.

``threading`` implementation:

.. code-block:: python

       def task(self, interval, parent_span):
           logger.info("Starting task")

           try:
               scope = self.tracer.scope_manager.activate(parent_span, False)
               with self.tracer.start_active_span("task"):
                   time.sleep(interval)
           finally:
               scope.close()
               if parent_span._ref_count.decr() == 0:
                   parent_span.finish()

``asyncio`` implementation:

.. code-block:: python

       async def task(self, interval, parent_span):
           logger.info("Starting task")

           with self.tracer.start_active_span("task"):
               await asyncio.sleep(interval)

       # Invoke and yield over the corotuines.
       with self.tracer.start_active_span("parent"):
       tasks = self.submit_callbacks()
       await asyncio.gather(*tasks)
