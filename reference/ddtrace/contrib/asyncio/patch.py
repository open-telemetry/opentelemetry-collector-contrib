import asyncio

from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

from ...internal.context_manager import CONTEXTVARS_IS_AVAILABLE
from .wrappers import wrapped_create_task, wrapped_create_task_contextvars
from ...utils.wrappers import unwrap as _u


def patch():
    """Patches current loop `create_task()` method to enable spawned tasks to
    parent to the base task context.
    """
    if getattr(asyncio, '_datadog_patch', False):
        return
    setattr(asyncio, '_datadog_patch', True)

    loop = asyncio.get_event_loop()
    if CONTEXTVARS_IS_AVAILABLE:
        _w(loop, 'create_task', wrapped_create_task_contextvars)
    else:
        _w(loop, 'create_task', wrapped_create_task)


def unpatch():
    """Remove tracing from patched modules."""

    if getattr(asyncio, '_datadog_patch', False):
        setattr(asyncio, '_datadog_patch', False)

    loop = asyncio.get_event_loop()
    _u(loop, 'create_task')
