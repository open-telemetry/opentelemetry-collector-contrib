__all__ = ["get_extensions"]

from setuptools import Extension


def get_extensions():
    return [Extension("ddtrace.vendor.wrapt._wrappers", sources=["ddtrace/vendor/wrapt/_wrappers.c"],)]
