import os

from .trace import trace_pyramid, DD_TWEEN_NAME
from .constants import (
    SETTINGS_SERVICE, SETTINGS_DISTRIBUTED_TRACING,
    SETTINGS_ANALYTICS_ENABLED, SETTINGS_ANALYTICS_SAMPLE_RATE,
)
from ...utils.formats import asbool, get_env

import pyramid.config
from pyramid.path import caller_package

from ddtrace.vendor import wrapt

DD_PATCH = '_datadog_patch'


def patch():
    """
    Patch pyramid.config.Configurator
    """
    if getattr(pyramid.config, DD_PATCH, False):
        return

    setattr(pyramid.config, DD_PATCH, True)
    _w = wrapt.wrap_function_wrapper
    _w('pyramid.config', 'Configurator.__init__', traced_init)


def traced_init(wrapped, instance, args, kwargs):
    settings = kwargs.pop('settings', {})
    service = os.environ.get('DATADOG_SERVICE_NAME') or 'pyramid'
    distributed_tracing = asbool(get_env('pyramid', 'distributed_tracing', True))
    # DEV: integration-specific analytics flag can be not set but still enabled
    # globally for web frameworks
    analytics_enabled = get_env('pyramid', 'analytics_enabled')
    if analytics_enabled is not None:
        analytics_enabled = asbool(analytics_enabled)
    analytics_sample_rate = get_env('pyramid', 'analytics_sample_rate', True)
    trace_settings = {
        SETTINGS_SERVICE: service,
        SETTINGS_DISTRIBUTED_TRACING: distributed_tracing,
        SETTINGS_ANALYTICS_ENABLED: analytics_enabled,
        SETTINGS_ANALYTICS_SAMPLE_RATE: analytics_sample_rate,
    }
    # Update over top of the defaults
    # DEV: If we did `settings.update(trace_settings)` then we would only ever
    #      have the default values.
    trace_settings.update(settings)
    # If the tweens are explicitly set with 'pyramid.tweens', we need to
    # explicitly set our tween too since `add_tween` will be ignored.
    insert_tween_if_needed(trace_settings)
    kwargs['settings'] = trace_settings

    # `caller_package` works by walking a fixed amount of frames up the stack
    # to find the calling package. So if we let the original `__init__`
    # function call it, our wrapper will mess things up.
    if not kwargs.get('package', None):
        # Get the packge for the third frame up from this one.
        #   - ddtrace.contrib.pyramid.path
        #   - ddtrace.vendor.wrapt
        #   - (this is the frame we want)
        # DEV: Default is `level=2` which will give us the package from `wrapt`
        kwargs['package'] = caller_package(level=3)

    wrapped(*args, **kwargs)
    trace_pyramid(instance)


def insert_tween_if_needed(settings):
    tweens = settings.get('pyramid.tweens')
    # If the list is empty, pyramid does not consider the tweens have been
    # set explicitly.
    # And if our tween is already there, nothing to do
    if not tweens or not tweens.strip() or DD_TWEEN_NAME in tweens:
        return
    # pyramid.tweens.EXCVIEW is the name of built-in exception view provided by
    # pyramid.  We need our tween to be before it, otherwise unhandled
    # exceptions will be caught before they reach our tween.
    idx = tweens.find(pyramid.tweens.EXCVIEW)
    if idx == -1:
        settings['pyramid.tweens'] = tweens + '\n' + DD_TWEEN_NAME
    else:
        settings['pyramid.tweens'] = tweens[:idx] + DD_TWEEN_NAME + '\n' + tweens[idx:]
