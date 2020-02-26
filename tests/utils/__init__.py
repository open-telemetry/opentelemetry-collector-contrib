import contextlib
import os

from ddtrace.ext import http


def assert_span_http_status_code(span, code):
    """Assert on the span's 'http.status_code' tag"""
    tag = span.get_tag(http.STATUS_CODE)
    code = str(code)
    assert tag == code, "%r != %r" % (tag, code)


@contextlib.contextmanager
def override_env(env):
    """
    Temporarily override ``os.environ`` with provided values::

        >>> with self.override_env(dict(DATADOG_TRACE_DEBUG=True)):
            # Your test
    """
    # Copy the full original environment
    original = dict(os.environ)

    # Update based on the passed in arguments
    os.environ.update(env)
    try:
        yield
    finally:
        # Full clear the environment out and reset back to the original
        os.environ.clear()
        os.environ.update(original)
