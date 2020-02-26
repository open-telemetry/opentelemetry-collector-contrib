import os

import flask
import werkzeug
from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

from ddtrace import compat
from ddtrace import config, Pin

from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...ext import SpanTypes, http
from ...internal.logger import get_logger
from ...propagation.http import HTTPPropagator
from ...utils.wrappers import unwrap as _u
from .helpers import get_current_app, get_current_span, simple_tracer, with_instance_pin
from .wrappers import wrap_function, wrap_signal

log = get_logger(__name__)

FLASK_ENDPOINT = 'flask.endpoint'
FLASK_VIEW_ARGS = 'flask.view_args'
FLASK_URL_RULE = 'flask.url_rule'
FLASK_VERSION = 'flask.version'

# Configure default configuration
config._add('flask', dict(
    # Flask service configuration
    # DEV: Environment variable 'DATADOG_SERVICE_NAME' used for backwards compatibility
    service_name=os.environ.get('DATADOG_SERVICE_NAME') or 'flask',
    app='flask',

    collect_view_args=True,
    distributed_tracing_enabled=True,
    template_default_name='<memory>',
    trace_signals=True,

    # We mark 5xx responses as errors, these codes are additional status codes to mark as errors
    # DEV: This is so that if a user wants to see `401` or `403` as an error, they can configure that
    extra_error_codes=set(),
))


# Extract flask version into a tuple e.g. (0, 12, 1) or (1, 0, 2)
# DEV: This makes it so we can do `if flask_version >= (0, 12, 0):`
# DEV: Example tests:
#      (0, 10, 0) > (0, 10)
#      (0, 10, 0) >= (0, 10, 0)
#      (0, 10, 1) >= (0, 10)
#      (0, 11, 1) >= (0, 10)
#      (0, 11, 1) >= (0, 10, 2)
#      (1, 0, 0) >= (0, 10)
#      (0, 9) == (0, 9)
#      (0, 9, 0) != (0, 9)
#      (0, 8, 5) <= (0, 9)
flask_version_str = getattr(flask, '__version__', '0.0.0')
flask_version = tuple([int(i) for i in flask_version_str.split('.')])


def patch():
    """
    Patch `flask` module for tracing
    """
    # Check to see if we have patched Flask yet or not
    if getattr(flask, '_datadog_patch', False):
        return
    setattr(flask, '_datadog_patch', True)

    # Attach service pin to `flask.app.Flask`
    Pin(
        service=config.flask['service_name'],
        app=config.flask['app']
    ).onto(flask.Flask)

    # flask.app.Flask methods that have custom tracing (add metadata, wrap functions, etc)
    _w('flask', 'Flask.wsgi_app', traced_wsgi_app)
    _w('flask', 'Flask.dispatch_request', request_tracer('dispatch_request'))
    _w('flask', 'Flask.preprocess_request', request_tracer('preprocess_request'))
    _w('flask', 'Flask.add_url_rule', traced_add_url_rule)
    _w('flask', 'Flask.endpoint', traced_endpoint)
    _w('flask', 'Flask._register_error_handler', traced_register_error_handler)

    # flask.blueprints.Blueprint methods that have custom tracing (add metadata, wrap functions, etc)
    _w('flask', 'Blueprint.register', traced_blueprint_register)
    _w('flask', 'Blueprint.add_url_rule', traced_blueprint_add_url_rule)

    # flask.app.Flask traced hook decorators
    flask_hooks = [
        'before_request',
        'before_first_request',
        'after_request',
        'teardown_request',
        'teardown_appcontext',
    ]
    for hook in flask_hooks:
        _w('flask', 'Flask.{}'.format(hook), traced_flask_hook)
    _w('flask', 'after_this_request', traced_flask_hook)

    # flask.app.Flask traced methods
    flask_app_traces = [
        'process_response',
        'handle_exception',
        'handle_http_exception',
        'handle_user_exception',
        'try_trigger_before_first_request_functions',
        'do_teardown_request',
        'do_teardown_appcontext',
        'send_static_file',
    ]
    for name in flask_app_traces:
        _w('flask', 'Flask.{}'.format(name), simple_tracer('flask.{}'.format(name)))

    # flask static file helpers
    _w('flask', 'send_file', simple_tracer('flask.send_file'))

    # flask.json.jsonify
    _w('flask', 'jsonify', traced_jsonify)

    # flask.templating traced functions
    _w('flask.templating', '_render', traced_render)
    _w('flask', 'render_template', traced_render_template)
    _w('flask', 'render_template_string', traced_render_template_string)

    # flask.blueprints.Blueprint traced hook decorators
    bp_hooks = [
        'after_app_request',
        'after_request',
        'before_app_first_request',
        'before_app_request',
        'before_request',
        'teardown_request',
        'teardown_app_request',
    ]
    for hook in bp_hooks:
        _w('flask', 'Blueprint.{}'.format(hook), traced_flask_hook)

    # flask.signals signals
    if config.flask['trace_signals']:
        signals = [
            'template_rendered',
            'request_started',
            'request_finished',
            'request_tearing_down',
            'got_request_exception',
            'appcontext_tearing_down',
        ]
        # These were added in 0.11.0
        if flask_version >= (0, 11):
            signals.append('before_render_template')

        # These were added in 0.10.0
        if flask_version >= (0, 10):
            signals.append('appcontext_pushed')
            signals.append('appcontext_popped')
            signals.append('message_flashed')

        for signal in signals:
            module = 'flask'

            # v0.9 missed importing `appcontext_tearing_down` in `flask/__init__.py`
            #  https://github.com/pallets/flask/blob/0.9/flask/__init__.py#L35-L37
            #  https://github.com/pallets/flask/blob/0.9/flask/signals.py#L52
            # DEV: Version 0.9 doesn't have a patch version
            if flask_version <= (0, 9) and signal == 'appcontext_tearing_down':
                module = 'flask.signals'

            # DEV: Patch `receivers_for` instead of `connect` to ensure we don't mess with `disconnect`
            _w(module, '{}.receivers_for'.format(signal), traced_signal_receivers_for(signal))


def unpatch():
    if not getattr(flask, '_datadog_patch', False):
        return
    setattr(flask, '_datadog_patch', False)

    props = [
        # Flask
        'Flask.wsgi_app',
        'Flask.dispatch_request',
        'Flask.add_url_rule',
        'Flask.endpoint',
        'Flask._register_error_handler',

        'Flask.preprocess_request',
        'Flask.process_response',
        'Flask.handle_exception',
        'Flask.handle_http_exception',
        'Flask.handle_user_exception',
        'Flask.try_trigger_before_first_request_functions',
        'Flask.do_teardown_request',
        'Flask.do_teardown_appcontext',
        'Flask.send_static_file',

        # Flask Hooks
        'Flask.before_request',
        'Flask.before_first_request',
        'Flask.after_request',
        'Flask.teardown_request',
        'Flask.teardown_appcontext',

        # Blueprint
        'Blueprint.register',
        'Blueprint.add_url_rule',

        # Blueprint Hooks
        'Blueprint.after_app_request',
        'Blueprint.after_request',
        'Blueprint.before_app_first_request',
        'Blueprint.before_app_request',
        'Blueprint.before_request',
        'Blueprint.teardown_request',
        'Blueprint.teardown_app_request',

        # Signals
        'template_rendered.receivers_for',
        'request_started.receivers_for',
        'request_finished.receivers_for',
        'request_tearing_down.receivers_for',
        'got_request_exception.receivers_for',
        'appcontext_tearing_down.receivers_for',

        # Top level props
        'after_this_request',
        'send_file',
        'jsonify',
        'render_template',
        'render_template_string',
        'templating._render',
    ]

    # These were added in 0.11.0
    if flask_version >= (0, 11):
        props.append('before_render_template.receivers_for')

    # These were added in 0.10.0
    if flask_version >= (0, 10):
        props.append('appcontext_pushed.receivers_for')
        props.append('appcontext_popped.receivers_for')
        props.append('message_flashed.receivers_for')

    for prop in props:
        # Handle 'flask.request_started.receivers_for'
        obj = flask

        # v0.9.0 missed importing `appcontext_tearing_down` in `flask/__init__.py`
        #  https://github.com/pallets/flask/blob/0.9/flask/__init__.py#L35-L37
        #  https://github.com/pallets/flask/blob/0.9/flask/signals.py#L52
        # DEV: Version 0.9 doesn't have a patch version
        if flask_version <= (0, 9) and prop == 'appcontext_tearing_down.receivers_for':
            obj = flask.signals

        if '.' in prop:
            attr, _, prop = prop.partition('.')
            obj = getattr(obj, attr, object())
        _u(obj, prop)


@with_instance_pin
def traced_wsgi_app(pin, wrapped, instance, args, kwargs):
    """
    Wrapper for flask.app.Flask.wsgi_app

    This wrapper is the starting point for all requests.
    """
    # DEV: This is safe before this is the args for a WSGI handler
    #   https://www.python.org/dev/peps/pep-3333/
    environ, start_response = args

    # Create a werkzeug request from the `environ` to make interacting with it easier
    # DEV: This executes before a request context is created
    request = werkzeug.Request(environ)

    # Configure distributed tracing
    if config.flask.get('distributed_tracing_enabled', False):
        propagator = HTTPPropagator()
        context = propagator.extract(request.headers)
        # Only need to activate the new context if something was propagated
        if context.trace_id:
            pin.tracer.context_provider.activate(context)

    # Default resource is method and path:
    #   GET /
    #   POST /save
    # We will override this below in `traced_dispatch_request` when we have a `RequestContext` and possibly a url rule
    resource = u'{} {}'.format(request.method, request.path)
    with pin.tracer.trace('flask.request', service=pin.service, resource=resource, span_type=SpanTypes.WEB) as s:
        # set analytics sample rate with global config enabled
        sample_rate = config.flask.get_analytics_sample_rate(use_global_config=True)
        if sample_rate is not None:
            s.set_tag(ANALYTICS_SAMPLE_RATE_KEY, sample_rate)

        s.set_tag(FLASK_VERSION, flask_version_str)

        # Wrap the `start_response` handler to extract response code
        # DEV: We tried using `Flask.finalize_request`, which seemed to work, but gave us hell during tests
        # DEV: The downside to using `start_response` is we do not have a `Flask.Response` object here,
        #   only `status_code`, and `headers` to work with
        #   On the bright side, this works in all versions of Flask (or any WSGI app actually)
        def _wrap_start_response(func):
            def traced_start_response(status_code, headers):
                code, _, _ = status_code.partition(' ')
                try:
                    code = int(code)
                except ValueError:
                    pass

                # Override root span resource name to be `<method> 404` for 404 requests
                # DEV: We do this because we want to make it easier to see all unknown requests together
                #      Also, we do this to reduce the cardinality on unknown urls
                # DEV: If we have an endpoint or url rule tag, then we don't need to do this,
                #      we still want `GET /product/<int:product_id>` grouped together,
                #      even if it is a 404
                if not s.get_tag(FLASK_ENDPOINT) and not s.get_tag(FLASK_URL_RULE):
                    s.resource = u'{} {}'.format(request.method, code)

                s.set_tag(http.STATUS_CODE, code)
                if 500 <= code < 600:
                    s.error = 1
                elif code in config.flask.get('extra_error_codes', set()):
                    s.error = 1
                return func(status_code, headers)
            return traced_start_response
        start_response = _wrap_start_response(start_response)

        # DEV: We set response status code in `_wrap_start_response`
        # DEV: Use `request.base_url` and not `request.url` to keep from leaking any query string parameters
        s.set_tag(http.URL, request.base_url)
        s.set_tag(http.METHOD, request.method)
        if config.flask.trace_query_string:
            s.set_tag(http.QUERY_STRING, compat.to_unicode(request.query_string))

        return wrapped(environ, start_response)


def traced_blueprint_register(wrapped, instance, args, kwargs):
    """
    Wrapper for flask.blueprints.Blueprint.register

    This wrapper just ensures the blueprint has a pin, either set manually on
    itself from the user or inherited from the application
    """
    app = kwargs.get('app', args[0])
    # Check if this Blueprint has a pin, otherwise clone the one from the app onto it
    pin = Pin.get_from(instance)
    if not pin:
        pin = Pin.get_from(app)
        if pin:
            pin.clone().onto(instance)
    return wrapped(*args, **kwargs)


def traced_blueprint_add_url_rule(wrapped, instance, args, kwargs):
    pin = Pin._find(wrapped, instance)
    if not pin:
        return wrapped(*args, **kwargs)

    def _wrap(rule, endpoint=None, view_func=None, **kwargs):
        if view_func:
            pin.clone().onto(view_func)
        return wrapped(rule, endpoint=endpoint, view_func=view_func, **kwargs)

    return _wrap(*args, **kwargs)


def traced_add_url_rule(wrapped, instance, args, kwargs):
    """Wrapper for flask.app.Flask.add_url_rule to wrap all views attached to this app"""
    def _wrap(rule, endpoint=None, view_func=None, **kwargs):
        if view_func:
            # TODO: `if hasattr(view_func, 'view_class')` then this was generated from a `flask.views.View`
            #   should we do something special with these views? Change the name/resource? Add tags?
            view_func = wrap_function(instance, view_func, name=endpoint, resource=rule)

        return wrapped(rule, endpoint=endpoint, view_func=view_func, **kwargs)

    return _wrap(*args, **kwargs)


def traced_endpoint(wrapped, instance, args, kwargs):
    """Wrapper for flask.app.Flask.endpoint to ensure all endpoints are wrapped"""
    endpoint = kwargs.get('endpoint', args[0])

    def _wrapper(func):
        # DEV: `wrap_function` will call `func_name(func)` for us
        return wrapped(endpoint)(wrap_function(instance, func, resource=endpoint))
    return _wrapper


def traced_flask_hook(wrapped, instance, args, kwargs):
    """Wrapper for hook functions (before_request, after_request, etc) are properly traced"""
    func = kwargs.get('f', args[0])
    return wrapped(wrap_function(instance, func))


def traced_render_template(wrapped, instance, args, kwargs):
    """Wrapper for flask.templating.render_template"""
    pin = Pin._find(wrapped, instance, get_current_app())
    if not pin or not pin.enabled():
        return wrapped(*args, **kwargs)

    with pin.tracer.trace('flask.render_template', span_type=SpanTypes.TEMPLATE):
        return wrapped(*args, **kwargs)


def traced_render_template_string(wrapped, instance, args, kwargs):
    """Wrapper for flask.templating.render_template_string"""
    pin = Pin._find(wrapped, instance, get_current_app())
    if not pin or not pin.enabled():
        return wrapped(*args, **kwargs)

    with pin.tracer.trace('flask.render_template_string', span_type=SpanTypes.TEMPLATE):
        return wrapped(*args, **kwargs)


def traced_render(wrapped, instance, args, kwargs):
    """
    Wrapper for flask.templating._render

    This wrapper is used for setting template tags on the span.

    This method is called for render_template or render_template_string
    """
    pin = Pin._find(wrapped, instance, get_current_app())
    # DEV: `get_current_span` will verify `pin` is valid and enabled first
    span = get_current_span(pin)
    if not span:
        return wrapped(*args, **kwargs)

    def _wrap(template, context, app):
        name = getattr(template, 'name', None) or config.flask.get('template_default_name')
        span.resource = name
        span.set_tag('flask.template_name', name)
        return wrapped(*args, **kwargs)
    return _wrap(*args, **kwargs)


def traced_register_error_handler(wrapped, instance, args, kwargs):
    """Wrapper to trace all functions registered with flask.app.register_error_handler"""
    def _wrap(key, code_or_exception, f):
        return wrapped(key, code_or_exception, wrap_function(instance, f))
    return _wrap(*args, **kwargs)


def request_tracer(name):
    @with_instance_pin
    def _traced_request(pin, wrapped, instance, args, kwargs):
        """
        Wrapper to trace a Flask function while trying to extract endpoint information
          (endpoint, url_rule, view_args, etc)

        This wrapper will add identifier tags to the current span from `flask.app.Flask.wsgi_app`.
        """
        span = get_current_span(pin)
        if not span:
            return wrapped(*args, **kwargs)

        try:
            request = flask._request_ctx_stack.top.request

            # DEV: This name will include the blueprint name as well (e.g. `bp.index`)
            if not span.get_tag(FLASK_ENDPOINT) and request.endpoint:
                span.resource = u'{} {}'.format(request.method, request.endpoint)
                span.set_tag(FLASK_ENDPOINT, request.endpoint)

            if not span.get_tag(FLASK_URL_RULE) and request.url_rule and request.url_rule.rule:
                span.resource = u'{} {}'.format(request.method, request.url_rule.rule)
                span.set_tag(FLASK_URL_RULE, request.url_rule.rule)

            if not span.get_tag(FLASK_VIEW_ARGS) and request.view_args and config.flask.get('collect_view_args'):
                for k, v in request.view_args.items():
                    span.set_tag(u'{}.{}'.format(FLASK_VIEW_ARGS, k), v)
        except Exception:
            log.debug('failed to set tags for "flask.request" span', exc_info=True)

        with pin.tracer.trace('flask.{}'.format(name), service=pin.service):
            return wrapped(*args, **kwargs)
    return _traced_request


def traced_signal_receivers_for(signal):
    """Wrapper for flask.signals.{signal}.receivers_for to ensure all signal receivers are traced"""
    def outer(wrapped, instance, args, kwargs):
        sender = kwargs.get('sender', args[0])
        # See if they gave us the flask.app.Flask as the sender
        app = None
        if isinstance(sender, flask.Flask):
            app = sender
        for receiver in wrapped(*args, **kwargs):
            yield wrap_signal(app, signal, receiver)
    return outer


def traced_jsonify(wrapped, instance, args, kwargs):
    pin = Pin._find(wrapped, instance, get_current_app())
    if not pin or not pin.enabled():
        return wrapped(*args, **kwargs)

    with pin.tracer.trace('flask.jsonify'):
        return wrapped(*args, **kwargs)
