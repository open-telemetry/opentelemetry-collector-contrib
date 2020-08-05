
Subtask Span propagation example.
=================================

This example shows an active ``Span`` being simply propagated to the subtasks -either threads or coroutines-, and finished **by** the parent task. In real-life scenarios instrumentation libraries may help with ``Span`` propagation **if** not offered by default (see implementation details below), but we show here the case without such help.

Implementation details:

* For ``threading``, the ``Span`` is manually passed down the call chain, activating it in each corotuine/task.
* For ``asyncio``, the active ``Span`` is not passed nor activated down the chain as the ``Context`` automatically propagates it.

``threading`` implementation:

.. code-block:: python

       def parent_task(self, message):
           with self.tracer.start_active_span("parent") as scope:
               f = self.executor.submit(self.child_task, message, scope.span)
               res = f.result()

           return res

       def child_task(self, message, span):
           with self.tracer.scope_manager.activate(span, False):
               with self.tracer.start_active_span("child"):
                   return "%s::response" % message

``asyncio`` implementation:

.. code-block:: python

       async def parent_task(self, message):  # noqa
           with self.tracer.start_active_span("parent"):
               res = await self.child_task(message)

           return res

       async def child_task(self, message):
           # No need to pass/activate the parent Span, as it stays in the context.
           with self.tracer.start_active_span("child"):
               return "%s::response" % message

