import ddtrace


def _wrap_submit(func, instance, args, kwargs):
    """
    Wrap `Executor` method used to submit a work executed in another
    thread. This wrapper ensures that a new `Context` is created and
    properly propagated using an intermediate function.
    """
    # If there isn't a currently active context, then do not create one
    # DEV: Calling `.active()` when there isn't an active context will create a new context
    # DEV: We need to do this in case they are either:
    #        - Starting nested futures
    #        - Starting futures from outside of an existing context
    #
    #      In either of these cases we essentially will propagate the wrong context between futures
    #
    #      The resolution is to not create/propagate a new context if one does not exist, but let the
    #      future's thread create the context instead.
    current_ctx = None
    if ddtrace.tracer.context_provider._has_active_context():
        current_ctx = ddtrace.tracer.context_provider.active()

        # If we have a context then make sure we clone it
        # DEV: We don't know if the future will finish executing before the parent span finishes
        #      so we clone to ensure we properly collect/report the future's spans
        current_ctx = current_ctx.clone()

    # extract the target function that must be executed in
    # a new thread and the `target` arguments
    fn = args[0]
    fn_args = args[1:]
    return func(_wrap_execution, current_ctx, fn, fn_args, kwargs)


def _wrap_execution(ctx, fn, args, kwargs):
    """
    Intermediate target function that is executed in a new thread;
    it receives the original function with arguments and keyword
    arguments, including our tracing `Context`. The current context
    provider sets the Active context in a thread local storage
    variable because it's outside the asynchronous loop.
    """
    if ctx is not None:
        ddtrace.tracer.context_provider.activate(ctx)
    return fn(*args, **kwargs)
