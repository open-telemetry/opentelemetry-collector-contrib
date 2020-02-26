"""
Decorator `timed` for coroutine methods.

Warning: requires Python 3.5 or higher.
"""
# stdlib
from functools import wraps
from time import time


def _get_wrapped_co(self, func):
    """
    `timed` wrapper for coroutine methods.
    """
    @wraps(func)
    async def wrapped_co(*args, **kwargs):
        start = time()
        try:
            result = await func(*args, **kwargs)
            return result
        finally:
            self._send(start)
    return wrapped_co
