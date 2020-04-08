from celery import signals

from ddtrace import Pin, config
from ddtrace.pin import _DD_PIN_NAME

from .constants import APP
from .signals import (
    trace_prerun,
    trace_postrun,
    trace_before_publish,
    trace_after_publish,
    trace_failure,
    trace_retry,
)


def patch_app(app, pin=None):
    """Attach the Pin class to the application and connect
    our handlers to Celery signals.
    """
    if getattr(app, '__datadog_patch', False):
        return
    setattr(app, '__datadog_patch', True)

    # attach the PIN object
    pin = pin or Pin(
        service=config.celery['worker_service_name'],
        app=APP,
        _config=config.celery,
    )
    pin.onto(app)
    # connect to the Signal framework

    signals.task_prerun.connect(trace_prerun, weak=False)
    signals.task_postrun.connect(trace_postrun, weak=False)
    signals.before_task_publish.connect(trace_before_publish, weak=False)
    signals.after_task_publish.connect(trace_after_publish, weak=False)
    signals.task_failure.connect(trace_failure, weak=False)
    signals.task_retry.connect(trace_retry, weak=False)
    return app


def unpatch_app(app):
    """Remove the Pin instance from the application and disconnect
    our handlers from Celery signal framework.
    """
    if not getattr(app, '__datadog_patch', False):
        return
    setattr(app, '__datadog_patch', False)

    pin = Pin.get_from(app)
    if pin is not None:
        delattr(app, _DD_PIN_NAME)

    signals.task_prerun.disconnect(trace_prerun)
    signals.task_postrun.disconnect(trace_postrun)
    signals.before_task_publish.disconnect(trace_before_publish)
    signals.after_task_publish.disconnect(trace_after_publish)
    signals.task_failure.disconnect(trace_failure)
    signals.task_retry.disconnect(trace_retry)
