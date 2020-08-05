
Common Request Handler example.
===============================

This example shows a ``Span`` used with ``RequestHandler``, which is used as a middleware (as in web frameworks) to manage a new ``Span`` per operation through its ``before_request()`` / ``after_response()`` methods.

Implementation details:


* For ``threading``, no active ``Span`` is consumed as the tasks may be run concurrently on different threads, and an explicit ``SpanContext`` has to be saved to be used as parent.

RequestHandler implementation:

.. code-block:: python

       def before_request(self, request, request_context):

           # If we should ignore the active Span, use any passed SpanContext
           # as the parent. Else, use the active one.
           span = self.tracer.start_span("send",
                                         child_of=self.context,
                                         ignore_active_span=True)

