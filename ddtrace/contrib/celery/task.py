from .app import patch_app

from ...utils.deprecation import deprecation


def patch_task(task, pin=None):
    """Deprecated API. The new API uses signals that can be activated via
    patch(celery=True) or through `ddtrace-run` script. Using this API
    enables instrumentation on all tasks.
    """
    deprecation(
        name='ddtrace.contrib.celery.patch_task',
        message='Use `patch(celery=True)` or `ddtrace-run` script instead',
        version='1.0.0',
    )

    # Enable instrumentation everywhere
    patch_app(task.app)
    return task


def unpatch_task(task):
    """Deprecated API. The new API uses signals that can be deactivated
    via unpatch() API. This API is now a no-op implementation so it doesn't
    affect instrumented tasks.
    """
    deprecation(
        name='ddtrace.contrib.celery.patch_task',
        message='Use `unpatch()` instead',
        version='1.0.0',
    )
    return task
