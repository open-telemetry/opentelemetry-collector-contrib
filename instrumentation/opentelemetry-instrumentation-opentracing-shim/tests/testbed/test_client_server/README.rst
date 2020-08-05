
Client-Server example.
======================

This example shows a ``Span`` created by a ``Client``, which will send a ``Message`` / ``SpanContext`` to a ``Server``, which will in turn extract such context and use it as parent of a new (server-side) ``Span``.

``Client.send()`` is used to send messages and inject the ``SpanContext`` using the ``TEXT_MAP`` format, and ``Server.process()`` will process received messages and will extract the context used as parent.

.. code-block:: python

   def send(self):
       with self.tracer.start_active_span("send") as scope:
           scope.span.set_tag(tags.SPAN_KIND, tags.SPAN_KIND_RPC_CLIENT)

           message = {}
           self.tracer.inject(scope.span.context,
                      opentracing.Format.TEXT_MAP,
                      message)
           self.queue.put(message)
