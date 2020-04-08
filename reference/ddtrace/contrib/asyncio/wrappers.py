import ddtrace

from .compat import asyncio_current_task
from .provider import CONTEXT_ATTR
from ...context import Context


def wrapped_create_task(wrapped, instance, args, kwargs):
    """Wrapper for ``create_task(coro)`` that propagates the current active
    ``Context`` to the new ``Task``. This function is useful to connect traces
    of detached executions.

    Note: we can't just link the task contexts due to the following scenario:
        * begin task A
        * task A starts task B1..B10
        * finish task B1-B9 (B10 still on trace stack)
        * task A starts task C
        * now task C gets parented to task B10 since it's still on the stack,
          however was not actually triggered by B10
    """
    new_task = wrapped(*args, **kwargs)
    current_task = asyncio_current_task()

    ctx = getattr(current_task, CONTEXT_ATTR, None)
    if ctx:
        # current task has a context, so parent a new context to the base context
        new_ctx = Context(
            trace_id=ctx.trace_id,
            span_id=ctx.span_id,
            sampling_priority=ctx.sampling_priority,
        )
        setattr(new_task, CONTEXT_ATTR, new_ctx)

    return new_task


def wrapped_create_task_contextvars(wrapped, instance, args, kwargs):
    """Wrapper for ``create_task(coro)`` that propagates the current active
    ``Context`` to the new ``Task``. This function is useful to connect traces
    of detached executions. Uses contextvars for task-local storage.
    """
    current_task_ctx = ddtrace.tracer.get_call_context()

    if not current_task_ctx:
        # no current context exists so nothing special to be done in handling
        # context for new task
        return wrapped(*args, **kwargs)

    # clone and activate current task's context for new task to support
    # detached executions
    new_task_ctx = current_task_ctx.clone()
    ddtrace.tracer.context_provider.activate(new_task_ctx)
    try:
        # activated context will now be copied to new task
        return wrapped(*args, **kwargs)
    finally:
        # reactivate current task context
        ddtrace.tracer.context_provider.activate(current_task_ctx)
