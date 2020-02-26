# flake8: noqa
"""
Imports for compatibility with Python 2, Python 3 and Google App Engine.
"""
from functools import wraps
import logging
import socket
import sys


def _is_py_version_higher_than(major, minor=0):
    """
    Assert that the Python version is higher than `$maj.$min`.
    """
    return sys.version_info >= (major, minor)


def is_p3k():
    """
    Assert that Python is version 3 or higher.
    """
    return _is_py_version_higher_than(3)


def is_higher_py35():
    """
    Assert that Python is version 3.5 or higher.
    """
    return _is_py_version_higher_than(3, 5)


get_input = input

# Python 3.x
if is_p3k():
    from io import StringIO
    import builtins
    import configparser
    import urllib.request as url_lib, urllib.error, urllib.parse

    imap = map
    text = str

    def iteritems(d):
        return iter(d.items())

    def iternext(iter):
        return next(iter)


# Python 2.x
else:
    import __builtin__ as builtins
    from cStringIO import StringIO
    from itertools import imap
    import ConfigParser as configparser
    import urllib2 as url_lib

    get_input = raw_input
    text = unicode

    def iteritems(d):
        return d.iteritems()

    def iternext(iter):
        return iter.next()


# Python > 3.5
if is_higher_py35():
    from asyncio import iscoroutinefunction

# Others
else:
    def iscoroutinefunction(*args, **kwargs):
        return False

# Optional requirements
try:
    from UserDict import IterableUserDict
except ImportError:
    from collections import UserDict as IterableUserDict

try:
    from configparser import ConfigParser
except ImportError:
    from ConfigParser import ConfigParser

try:
    from urllib.parse import urlparse
except ImportError:
    from urlparse import urlparse

try:
    import pkg_resources as pkg
except ImportError:
    pkg = None

#Python 2.6.x
try:
    from logging import NullHandler
except ImportError:
    from logging import Handler

    class NullHandler(Handler):
        def emit(self, record):
            pass
