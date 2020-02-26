import gevent

from functools import wraps


_NOT_ERROR = gevent.hub.Hub.NOT_ERROR


def silence_errors(f):
    """
    Test decorator for gevent that silences all errors when
    a greenlet raises an exception.
    """
    @wraps(f)
    def wrapper(*args, **kwargs):
        gevent.hub.Hub.NOT_ERROR = (Exception,)
        f(*args, **kwargs)
        gevent.hub.Hub.NOT_ERROR = _NOT_ERROR
    return wrapper
