
Nested callbacks example.
=========================

This example shows a ``Span`` for a top-level operation, and how it can be passed down on a list of nested callbacks (always one at a time), have it as the active one for each of them, and finished **only** when the last one executes. For Python, we have decided to do it in a **fire-and-forget** fashion.

Implementation details:


* For ``threading``, the ``Span`` is manually activatted it in each corotuine/task.
* For ``asyncio``, the active ``Span`` is not activated down the chain as the ``Context`` automatically propagates it.

``threading`` implementation:

.. code-block:: python

       def submit(self):
           span = self.tracer.scope_manager.active.span

           def task1():
               with self.tracer.scope_manager.activate(span, False):
                   span.set_tag("key1", "1")

                   def task2():
                       with self.tracer.scope_manager.activate(span, False):
                           span.set_tag("key2", "2")
                           ...

``asyncio`` implementation:

.. code-block:: python

        async def task1():
            span.set_tag("key1", "1")

            async def task2():
                span.set_tag("key2", "2")

                async def task3():
                    span.set_tag("key3", "3")
                    span.finish()

                self.loop.create_task(task3())

            self.loop.create_task(task2())

        self.loop.create_task(task1())
