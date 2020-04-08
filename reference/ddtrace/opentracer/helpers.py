import opentracing
import ddtrace

"""
Helper routines for Datadog OpenTracing.
"""


def set_global_tracer(tracer):
    """Sets the global tracers to the given tracer."""

    # overwrite the opentracer reference
    opentracing.tracer = tracer

    # overwrite the Datadog tracer reference
    ddtrace.tracer = tracer._dd_tracer
