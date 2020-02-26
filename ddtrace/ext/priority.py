"""
Priority is a hint given to the backend so that it knows which traces to reject or kept.
In a distributed context, it should be set before any context propagation (fork, RPC calls) to be effective.

For example:

from ddtrace.ext.priority import USER_REJECT, USER_KEEP

context = tracer.context_provider.active()
# Indicate to not keep the trace
context.sampling_priority = USER_REJECT

# Indicate to keep the trace
span.context.sampling_priority = USER_KEEP
"""

# Use this to explicitly inform the backend that a trace should be rejected and not stored.
USER_REJECT = -1
# Used by the builtin sampler to inform the backend that a trace should be rejected and not stored.
AUTO_REJECT = 0
# Used by the builtin sampler to inform the backend that a trace should be kept and stored.
AUTO_KEEP = 1
# Use this to explicitly inform the backend that a trace should be kept and stored.
USER_KEEP = 2
