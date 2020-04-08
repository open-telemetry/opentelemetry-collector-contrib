"""
The Flask__ integration will add tracing to all requests to your Flask application.

This integration will track the entire Flask lifecycle including user-defined endpoints, hooks,
signals, and templating rendering.

To configure tracing manually::

    from ddtrace import patch_all
    patch_all()

    from flask import Flask

    app = Flask(__name__)


    @app.route('/')
    def index():
        return 'hello world'


    if __name__ == '__main__':
        app.run()


You may also enable Flask tracing automatically via ddtrace-run::

    ddtrace-run python app.py


Configuration
~~~~~~~~~~~~~

.. py:data:: ddtrace.config.flask['distributed_tracing_enabled']

   Whether to parse distributed tracing headers from requests received by your Flask app.

   Default: ``True``

.. py:data:: ddtrace.config.flask['analytics_enabled']

   Whether to generate APM events for Flask in Trace Search & Analytics.

   Can also be enabled with the ``DD_FLASK_ANALYTICS_ENABLED`` environment variable.

   Default: ``None``

.. py:data:: ddtrace.config.flask['service_name']

   The service name reported for your Flask app.

   Can also be configured via the ``DATADOG_SERVICE_NAME`` environment variable.

   Default: ``'flask'``

.. py:data:: ddtrace.config.flask['collect_view_args']

   Whether to add request tags for view function argument values.

   Default: ``True``

.. py:data:: ddtrace.config.flask['template_default_name']

   The default template name to use when one does not exist.

   Default: ``<memory>``

.. py:data:: ddtrace.config.flask['trace_signals']

   Whether to trace Flask signals (``before_request``, ``after_request``, etc).

   Default: ``True``

.. py:data:: ddtrace.config.flask['extra_error_codes']

   A list of response codes that should get marked as errors.

   *5xx codes are always considered an error.*

   Default: ``[]``


Example::

    from ddtrace import config

    # Enable distributed tracing
    config.flask['distributed_tracing_enabled'] = True

    # Override service name
    config.flask['service_name'] = 'custom-service-name'

    # Report 401, and 403 responses as errors
    config.flask['extra_error_codes'] = [401, 403]

.. __: http://flask.pocoo.org/
"""

from ...utils.importlib import require_modules


required_modules = ['flask']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        # DEV: We do this so we can `@mock.patch('ddtrace.contrib.flask._patch.<func>')` in tests
        from . import patch as _patch
        from .middleware import TraceMiddleware

        patch = _patch.patch
        unpatch = _patch.unpatch

        __all__ = ['TraceMiddleware', 'patch', 'unpatch']
