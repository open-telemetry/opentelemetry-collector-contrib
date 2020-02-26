Web Frameworks
--------------

``ddtrace`` provides tracing support for many Python web frameworks. For each
framework ``ddtrace`` supports:

- tracing of requests [*]_: trace requests through middleware and back
- distributed tracing [*]_: trace requests across application boundaries
- automatic error tagging [*]_: spans will be marked with any errors that occur

.. [*] https://docs.datadoghq.com/tracing/
.. [*] https://docs.datadoghq.com/tracing/faq/distributed-tracing/
.. [*] "erroneous HTTP return codes" are defined as being greater than 500

.. _aiohttp:

aiohttp
^^^^^^^

.. automodule:: ddtrace.contrib.aiohttp


.. _bottle:

Bottle
^^^^^^

.. automodule:: ddtrace.contrib.bottle

.. _djangorestframework:
.. _django:

Django
^^^^^^

.. automodule:: ddtrace.contrib.django


.. _falcon:

Falcon
^^^^^^

.. automodule:: ddtrace.contrib.falcon


.. _flask:

Flask
^^^^^


.. automodule:: ddtrace.contrib.flask

.. _molten:

Molten
^^^^^^

.. automodule:: ddtrace.contrib.molten

.. _pylons:

Pylons
^^^^^^

.. automodule:: ddtrace.contrib.pylons


.. _pyramid:

Pyramid
^^^^^^^

.. automodule:: ddtrace.contrib.pyramid


.. _tornado:

Tornado
^^^^^^^

.. automodule:: ddtrace.contrib.tornado

