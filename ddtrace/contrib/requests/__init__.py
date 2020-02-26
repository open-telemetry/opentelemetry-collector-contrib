"""
The ``requests`` integration traces all HTTP calls to internal or external services.
Auto instrumentation is available using the ``patch`` function that **must be called
before** importing the ``requests`` library. The following is an example::

    from ddtrace import patch
    patch(requests=True)

    import requests
    requests.get("https://www.datadoghq.com")

If you would prefer finer grained control, use a ``TracedSession`` object as you would a
``requests.Session``::

    from ddtrace.contrib.requests import TracedSession

    session = TracedSession()
    session.get("https://www.datadoghq.com")

The library can be configured globally and per instance, using the Configuration API::

    from ddtrace import config

    # disable distributed tracing globally
    config.requests['distributed_tracing'] = False

    # enable trace analytics globally
    config.requests['analytics_enabled'] = True

    # change the service name/distributed tracing only for this session
    session = Session()
    cfg = config.get_from(session)
    cfg['service_name'] = 'auth-api'
    cfg['analytics_enabled'] = True

:ref:`Headers tracing <http-headers-tracing>` is supported for this integration.
"""
from ...utils.importlib import require_modules


required_modules = ['requests']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch, unpatch
        from .session import TracedSession

        __all__ = [
            'patch',
            'unpatch',
            'TracedSession',
        ]
