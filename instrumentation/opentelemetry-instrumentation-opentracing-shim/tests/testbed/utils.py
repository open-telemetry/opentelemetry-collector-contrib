from __future__ import print_function

import logging
import threading
import time


class RefCount:
    """Thread-safe counter"""

    def __init__(self, count=1):
        self._lock = threading.Lock()
        self._count = count

    def incr(self):
        with self._lock:
            self._count += 1
            return self._count

    def decr(self):
        with self._lock:
            self._count -= 1
            return self._count


def await_until(func, timeout=5.0):
    """Polls for func() to return True"""
    end_time = time.time() + timeout
    while time.time() < end_time and not func():
        time.sleep(0.01)


def stop_loop_when(loop, cond_func, timeout=5.0):
    """
    Registers a periodic callback that stops the loop when cond_func() == True.
    Compatible with both Tornado and asyncio.
    """
    if cond_func() or timeout <= 0.0:
        loop.stop()
        return

    timeout -= 0.1
    loop.call_later(0.1, stop_loop_when, loop, cond_func, timeout)


def get_logger(name):
    """Returns a logger with log level set to INFO"""
    logging.basicConfig(level=logging.INFO)
    return logging.getLogger(name)


def get_one_by_tag(spans, key, value):
    """Return a single Span with a tag value/key from a list,
    errors if more than one is found."""

    found = []
    for span in spans:
        if span.attributes.get(key) == value:
            found.append(span)

    if len(found) > 1:
        raise RuntimeError("Too many values")

    return found[0] if len(found) > 0 else None


def get_one_by_operation_name(spans, name):
    """Return a single Span with a name from a list,
    errors if more than one is found."""
    found = []
    for span in spans:
        if span.name == name:
            found.append(span)

    if len(found) > 1:
        raise RuntimeError("Too many values")

    return found[0] if len(found) > 0 else None
