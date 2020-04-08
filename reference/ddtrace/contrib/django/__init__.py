"""
The Django integration will trace users requests, template renderers, database and cache
calls.

**Note:** by default the tracer is **disabled** (will not send spans) when
the Django setting ``DEBUG`` is ``True``. This can be overridden by explicitly enabling
the tracer with ``DATADOG_TRACE['ENABLED'] = True``, as described below.

To enable the Django integration, add the application to your installed
apps, as follows::

    INSTALLED_APPS = [
        # your Django apps...

        # the order is not important
        'ddtrace.contrib.django',
    ]

The configuration for this integration is namespaced under the ``DATADOG_TRACE``
Django setting. For example, your ``settings.py`` may contain::

    DATADOG_TRACE = {
        'DEFAULT_SERVICE': 'my-django-app',
        'TAGS': {'env': 'production'},
    }

If you need to access to Datadog settings, you can::

    from ddtrace.contrib.django.conf import settings

    tracer = settings.TRACER
    tracer.trace("something")
    # your code ...

To have Django capture the tracer logs, ensure the ``LOGGING`` variable in
``settings.py`` looks similar to::

    LOGGING = {
        'loggers': {
            'ddtrace': {
                'handlers': ['console'],
                'level': 'WARNING',
            },
        },
    }


The available settings are:

* ``DEFAULT_SERVICE`` (default: ``'django'``): set the service name used by the
  tracer. Usually this configuration must be updated with a meaningful name.
* ``DEFAULT_DATABASE_PREFIX`` (default: ``''``): set a prefix value to database services,
  so that your service is listed such as `prefix-defaultdb`.
* ``DEFAULT_CACHE_SERVICE`` (default: ``''``): set the django cache service name used
  by the tracer. Change this name if you want to see django cache spans as a cache application.
* ``TAGS`` (default: ``{}``): set global tags that should be applied to all
  spans.
* ``TRACER`` (default: ``ddtrace.tracer``): set the default tracer
  instance that is used to trace Django internals. By default the ``ddtrace``
  tracer is used.
* ``ENABLED`` (default: ``not django_settings.DEBUG``): defines if the tracer is
  enabled or not. If set to false, the code is still instrumented but no spans
  are sent to the trace agent. This setting cannot be changed at runtime
  and a restart is required. By default the tracer is disabled when in ``DEBUG``
  mode, enabled otherwise.
* ``DISTRIBUTED_TRACING`` (default: ``True``): defines if the tracer should
  use incoming X-DATADOG-* HTTP headers to extend a trace created remotely. It is
  required for distributed tracing if this application is called remotely from another
  instrumented application.
  We suggest to enable it only for internal services where headers are under your control.
* ``ANALYTICS_ENABLED`` (default: ``None``): enables APM events in Trace Search & Analytics.
* ``AGENT_HOSTNAME`` (default: ``localhost``): define the hostname of the trace agent.
* ``AGENT_PORT`` (default: ``8126``): define the port of the trace agent.
* ``AUTO_INSTRUMENT`` (default: ``True``): if set to false the code will not be
  instrumented (even if ``INSTRUMENT_DATABASE``, ``INSTRUMENT_CACHE`` or
  ``INSTRUMENT_TEMPLATE`` are set to ``True``), while the tracer may be active
  for your internal usage. This could be useful if you want to use the Django
  integration, but you want to trace only particular functions or views. If set
  to False, the request middleware will be disabled even if present.
* ``INSTRUMENT_DATABASE`` (default: ``True``): if set to ``False`` database will not
  be instrumented. Only configurable when ``AUTO_INSTRUMENT`` is set to ``True``.
* ``INSTRUMENT_CACHE`` (default: ``True``): if set to ``False`` cache will not
  be instrumented. Only configurable when ``AUTO_INSTRUMENT`` is set to ``True``.
* ``INSTRUMENT_TEMPLATE`` (default: ``True``): if set to ``False`` template
  rendering will not be instrumented. Only configurable when ``AUTO_INSTRUMENT``
  is set to ``True``.
"""
from ...utils.importlib import require_modules


required_modules = ['django']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .middleware import TraceMiddleware, TraceExceptionMiddleware
        from .patch import patch
        __all__ = ['TraceMiddleware', 'TraceExceptionMiddleware', 'patch']


# define the Django app configuration
default_app_config = 'ddtrace.contrib.django.apps.TracerConfig'
