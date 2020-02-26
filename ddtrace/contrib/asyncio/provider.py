import asyncio

from ...context import Context
from ...provider import DefaultContextProvider

# Task attribute used to set/get the Context instance
CONTEXT_ATTR = '__datadog_context'


class AsyncioContextProvider(DefaultContextProvider):
    """
    Context provider that retrieves all contexts for the current asyncio
    execution. It must be used in asynchronous programming that relies
    in the built-in ``asyncio`` library. Framework instrumentation that
    is built on top of the ``asyncio`` library, can use this provider.

    This Context Provider inherits from ``DefaultContextProvider`` because
    it uses a thread-local storage when the ``Context`` is propagated to
    a different thread, than the one that is running the async loop.
    """
    def activate(self, context, loop=None):
        """Sets the scoped ``Context`` for the current running ``Task``.
        """
        loop = self._get_loop(loop)
        if not loop:
            self._local.set(context)
            return context

        # the current unit of work (if tasks are used)
        task = asyncio.Task.current_task(loop=loop)
        setattr(task, CONTEXT_ATTR, context)
        return context

    def _get_loop(self, loop=None):
        """Helper to try and resolve the current loop"""
        try:
            return loop or asyncio.get_event_loop()
        except RuntimeError:
            # Detects if a loop is available in the current thread;
            # DEV: This happens when a new thread is created from the out that is running the async loop
            # DEV: It's possible that a different Executor is handling a different Thread that
            #      works with blocking code. In that case, we fallback to a thread-local Context.
            pass
        return None

    def _has_active_context(self, loop=None):
        """Helper to determine if we have a currently active context"""
        loop = self._get_loop(loop=loop)
        if loop is None:
            return self._local._has_active_context()

        # the current unit of work (if tasks are used)
        task = asyncio.Task.current_task(loop=loop)
        if task is None:
            return False

        ctx = getattr(task, CONTEXT_ATTR, None)
        return ctx is not None

    def active(self, loop=None):
        """
        Returns the scoped Context for this execution flow. The ``Context`` uses
        the current task as a carrier so if a single task is used for the entire application,
        the context must be handled separately.
        """
        loop = self._get_loop(loop=loop)
        if not loop:
            return self._local.get()

        # the current unit of work (if tasks are used)
        task = asyncio.Task.current_task(loop=loop)
        if task is None:
            # providing a detached Context from the current Task, may lead to
            # wrong traces. This defensive behavior grants that a trace can
            # still be built without raising exceptions
            return Context()

        ctx = getattr(task, CONTEXT_ATTR, None)
        if ctx is not None:
            # return the active Context for this task (if any)
            return ctx

        # create a new Context using the Task as a Context carrier
        ctx = Context()
        setattr(task, CONTEXT_ATTR, ctx)
        return ctx
