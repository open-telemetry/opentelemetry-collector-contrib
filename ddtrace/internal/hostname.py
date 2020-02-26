import functools
import socket

_hostname = None


def _cached(func):
    @functools.wraps(func)
    def wrapper():
        global _hostname
        if not _hostname:
            _hostname = func()

        return _hostname

    return wrapper


@_cached
def get_hostname():
    return socket.gethostname()
