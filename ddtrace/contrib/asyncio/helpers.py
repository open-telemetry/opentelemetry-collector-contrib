"""
This module includes a list of convenience methods that
can be used to simplify some operations while handling
Context and Spans in instrumented ``asyncio`` code.
"""
import asyncio
import ddtrace

from .provider import CONTEXT_ATTR
from .wrappers import wrapped_create_task
from ...context import Context


def set_call_context(task, ctx):
    """
    Updates the ``Context`` for the given Task. Useful when you need to
    pass the context among different tasks.

    This method is available for backward-compatibility. Use the
    ``AsyncioContextProvider`` API to set the current active ``Context``.
    """
    setattr(task, CONTEXT_ATTR, ctx)


def ensure_future(coro_or_future, *, loop=None, tracer=None):
    """Wrapper that sets a context to the newly created Task.

    If the current task already has a Context, it will be attached to the new Task so the Trace list will be preserved.
    """
    tracer = tracer or ddtrace.tracer
    current_ctx = tracer.get_call_context()
    task = asyncio.ensure_future(coro_or_future, loop=loop)
    set_call_context(task, current_ctx)
    return task


def run_in_executor(loop, executor, func, *args, tracer=None):
    """Wrapper function that sets a context to the newly created Thread.

    If the current task has a Context, it will be attached as an empty Context with the current_span activated to
    inherit the ``trace_id`` and the ``parent_id``.

    Because the Executor can run the Thread immediately or after the
    coroutine is executed, we may have two different scenarios:
    * the Context is copied in the new Thread and the trace is sent twice
    * the coroutine flushes the Context and when the Thread copies the
    Context it is already empty (so it will be a root Span)

    To support both situations, we create a new Context that knows only what was
    the latest active Span when the new thread was created. In this new thread,
    we fallback to the thread-local ``Context`` storage.

    """
    tracer = tracer or ddtrace.tracer
    ctx = Context()
    current_ctx = tracer.get_call_context()
    ctx._current_span = current_ctx._current_span

    # prepare the future using an executor wrapper
    future = loop.run_in_executor(executor, _wrap_executor, func, args, tracer, ctx)
    return future


def _wrap_executor(fn, args, tracer, ctx):
    """
    This function is executed in the newly created Thread so the right
    ``Context`` can be set in the thread-local storage. This operation
    is safe because the ``Context`` class is thread-safe and can be
    updated concurrently.
    """
    # the AsyncioContextProvider knows that this is a new thread
    # so it is legit to pass the Context in the thread-local storage;
    # fn() will be executed outside the asyncio loop as a synchronous code
    tracer.context_provider.activate(ctx)
    return fn(*args)


def create_task(*args, **kwargs):
    """This function spawns a task with a Context that inherits the
    `trace_id` and the `parent_id` from the current active one if available.
    """
    loop = asyncio.get_event_loop()
    return wrapped_create_task(loop.create_task, None, args, kwargs)
