
Listener Response example.
==========================

This example shows a ``Span`` created upon a message being sent to a ``Client``, and its handling along a related, **not shared** ``ResponseListener`` object with a ``on_response(self, response)`` method to finish it.

.. code-block:: python

       def _task(self, message, listener):
           res = "%s::response" % message
           listener.on_response(res)
           return res

       def send_sync(self, message):
           span = self.tracer.start_span("send")
           span.set_tag(tags.SPAN_KIND, tags.SPAN_KIND_RPC_CLIENT)

           listener = ResponseListener(span)
           return self.executor.submit(self._task, message, listener).result()
