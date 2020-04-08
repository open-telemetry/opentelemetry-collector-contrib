import requests

from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

from .connection import _wrap_send


class TracedSession(requests.Session):
    """TracedSession is a requests' Session that is already traced.
    You can use it if you want a finer grained control for your
    HTTP clients.
    """
    pass


# always patch our `TracedSession` when imported
_w(TracedSession, 'send', _wrap_send)
