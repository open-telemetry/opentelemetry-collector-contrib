import ddtrace
from contextlib import contextmanager


def assert_dict_issuperset(a, b):
    assert set(a.items()).issuperset(set(b.items())), \
        '{a} is not a superset of {b}'.format(a=a, b=b)


@contextmanager
def override_global_tracer(tracer):
    """Helper functions that overrides the global tracer available in the
    `ddtrace` package. This is required because in some `httplib` tests we
    can't get easily the PIN object attached to the `HTTPConnection` to
    replace the used tracer with a dummy tracer.
    """
    original_tracer = ddtrace.tracer
    ddtrace.tracer = tracer
    yield
    ddtrace.tracer = original_tracer
