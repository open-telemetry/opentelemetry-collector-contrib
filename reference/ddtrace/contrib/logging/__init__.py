"""
Datadog APM traces can be integrated with Logs by first having the tracing
library patch the standard library ``logging`` module and updating the log
formatter used by an application. This feature enables you to inject the current
trace information into a log entry.

Before the trace information can be injected into logs, the formatter has to be
updated to include ``dd.trace_id`` and ``dd.span_id`` attributes from the log
record. The integration with Logs occurs as long as the log entry includes
``dd.trace_id=%(dd.trace_id)s`` and ``dd.span_id=%(dd.span_id)s``.

ddtrace-run
-----------

When using ``ddtrace-run``, enable patching by setting the environment variable
``DD_LOGS_INJECTION=true``. The logger by default will have a format that
includes trace information::

    import logging
    from ddtrace import tracer

    log = logging.getLogger()
    log.level = logging.INFO


    @tracer.wrap()
    def hello():
        log.info('Hello, World!')

    hello()

Manual Instrumentation
----------------------

If you prefer to instrument manually, patch the logging library then update the
log formatter as in the following example::

    from ddtrace import patch_all; patch_all(logging=True)
    import logging
    from ddtrace import tracer

    FORMAT = ('%(asctime)s %(levelname)s [%(name)s] [%(filename)s:%(lineno)d] '
              '[dd.trace_id=%(dd.trace_id)s dd.span_id=%(dd.span_id)s] '
              '- %(message)s')
    logging.basicConfig(format=FORMAT)
    log = logging.getLogger()
    log.level = logging.INFO


    @tracer.wrap()
    def hello():
        log.info('Hello, World!')

    hello()
"""

from ...utils.importlib import require_modules


required_modules = ['logging']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .patch import patch, unpatch

        __all__ = ['patch', 'unpatch']
