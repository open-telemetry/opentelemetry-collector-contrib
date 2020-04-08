from ddtrace.opentracer import Tracer


def init_tracer(service_name, dd_tracer, scope_manager=None):
    """A method that emulates what a user of OpenTracing would call to
    initialize a Datadog opentracer.

    It accepts a Datadog tracer that should be the same one used for testing.
    """
    writer = dd_tracer.writer
    ot_tracer = Tracer(service_name, dd_tracer=dd_tracer, scope_manager=scope_manager)
    dd_tracer.writer = writer
    ot_tracer._dd_tracer = dd_tracer
    return ot_tracer
