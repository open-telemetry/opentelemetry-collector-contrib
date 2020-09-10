OpenTelemetry ASGI Instrumentation
==================================

|pypi|

.. |pypi| image:: https://badge.fury.io/py/opentelemetry-instrumentation-asgi.svg
   :target: https://pypi.org/project/opentelemetry-instrumentation-asgi/


This library provides a ASGI middleware that can be used on any ASGI framework
(such as Django, Starlette, FastAPI or Quart) to track requests timing through OpenTelemetry.

Installation
------------

::

    pip install opentelemetry-instrumentation-asgi


Usage (Quart)
-------------

.. code-block:: python

    from quart import Quart
    from opentelemetry.instrumentation.asgi import OpenTelemetryMiddleware

    app = Quart(__name__)
    app.asgi_app = OpenTelemetryMiddleware(app.asgi_app)

    @app.route("/")
    async def hello():
        return "Hello!"

    if __name__ == "__main__":
        app.run(debug=True)


Usage (Django 3.0)
------------------

Modify the application's ``asgi.py`` file as shown below.

.. code-block:: python

    import os
    from django.core.asgi import get_asgi_application
    from opentelemetry.instrumentation.asgi import OpenTelemetryMiddleware

    os.environ.setdefault('DJANGO_SETTINGS_MODULE', 'asgi_example.settings')

    application = get_asgi_application()
    application = OpenTelemetryMiddleware(application)


Usage (Raw ASGI)
----------------

.. code-block:: python

    from opentelemetry.instrumentation.asgi import OpenTelemetryMiddleware

    app = ...  # An ASGI application.
    app = OpenTelemetryMiddleware(app)


References
----------

* `OpenTelemetry Project <https://opentelemetry.io/>`_
