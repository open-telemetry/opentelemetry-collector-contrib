__all__ = ["get_extensions"]

from setuptools import Extension
import sys


def get_extensions():
    libraries = []
    if sys.platform == "win32":
        libraries.append("ws2_32")

    macros = []
    if sys.byteorder == "big":
        macros = [("__BIG_ENDIAN__", "1")]
    else:
        macros = [("__LITTLE_ENDIAN__", "1")]

    ext = Extension(
        "ddtrace.vendor.msgpack._cmsgpack",
        sources=["ddtrace/vendor/msgpack/_cmsgpack.cpp"],
        libraries=libraries,
        include_dirs=["ddtrace/vendor/"],
        define_macros=macros,
    )

    return [ext]
