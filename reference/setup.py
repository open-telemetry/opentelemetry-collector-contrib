import copy
import os
import sys

from distutils.command.build_ext import build_ext
from distutils.errors import CCompilerError, DistutilsExecError, DistutilsPlatformError
from setuptools import setup, find_packages
from setuptools.command.test import test as TestCommand


HERE = os.path.dirname(os.path.abspath(__file__))


def load_module_from_project_file(mod_name, fname):
    """
    Helper used to load a module from a file in this project

    DEV: Loading this way will by-pass loading all parent modules
         e.g. importing `ddtrace.vendor.psutil.setup` will load `ddtrace/__init__.py`
         which has side effects like loading the tracer
    """
    fpath = os.path.join(HERE, fname)

    if sys.version_info >= (3, 5):
        import importlib.util

        spec = importlib.util.spec_from_file_location(mod_name, fpath)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        return mod
    elif sys.version_info >= (3, 3):
        from importlib.machinery import SourceFileLoader

        return SourceFileLoader(mod_name, fpath).load_module()
    else:
        import imp

        return imp.load_source(mod_name, fpath)


class Tox(TestCommand):

    user_options = [("tox-args=", "a", "Arguments to pass to tox")]

    def initialize_options(self):
        TestCommand.initialize_options(self)
        self.tox_args = None

    def finalize_options(self):
        TestCommand.finalize_options(self)
        self.test_args = []
        self.test_suite = True

    def run_tests(self):
        # import here, cause outside the eggs aren't loaded
        import tox
        import shlex

        args = self.tox_args
        if args:
            args = shlex.split(self.tox_args)
        errno = tox.cmdline(args=args)
        sys.exit(errno)


long_description = """
# dd-trace-py

`ddtrace` is Datadog's tracing library for Python.  It is used to trace requests
as they flow across web servers, databases and microservices so that developers
have great visiblity into bottlenecks and troublesome requests.

## Getting Started

For a basic product overview, installation and quick start, check out our
[setup documentation][setup docs].

For more advanced usage and configuration, check out our [API
documentation][pypi docs].

For descriptions of terminology used in APM, take a look at the [official
documentation][visualization docs].

[setup docs]: https://docs.datadoghq.com/tracing/setup/python/
[pypi docs]: http://pypi.datadoghq.com/trace/docs/
[visualization docs]: https://docs.datadoghq.com/tracing/visualization/
"""

# enum34 is an enum backport for earlier versions of python
# funcsigs backport required for vendored debtcollector
install_requires = ["psutil>=5.0.0", "enum34; python_version<'3.4'"]

# Base `setup()` kwargs without any C-extension registering
setup_kwargs = dict(
    name="ddtrace",
    description="Datadog tracing code",
    url="https://github.com/DataDog/dd-trace-py",
    author="Datadog, Inc.",
    author_email="dev@datadoghq.com",
    long_description=long_description,
    long_description_content_type="text/markdown",
    license="BSD",
    packages=find_packages(exclude=["tests*"]),
    install_requires=install_requires,
    extras_require={
        # users can include opentracing by having:
        # install_requires=['ddtrace[opentracing]', ...]
        "opentracing": ["opentracing>=2.0.0"],
    },
    # plugin tox
    tests_require=["tox", "flake8"],
    cmdclass={"test": Tox},
    entry_points={"console_scripts": ["ddtrace-run = ddtrace.commands.ddtrace_run:main"]},
    classifiers=[
        "Programming Language :: Python",
        "Programming Language :: Python :: 3.4",
        "Programming Language :: Python :: 3.5",
        "Programming Language :: Python :: 3.6",
        "Programming Language :: Python :: 3.7",
    ],
    use_scm_version=True,
    setup_requires=["setuptools_scm"],
)


if sys.platform == "win32":
    build_ext_errors = (CCompilerError, DistutilsExecError, DistutilsPlatformError, IOError, OSError)
else:
    build_ext_errors = (CCompilerError, DistutilsExecError, DistutilsPlatformError)


class BuildExtFailed(Exception):
    pass


# Attempt to build a C-extension, catch exceptions so failed building skips the extension
# DEV: This is basically what `distutils`'s' `Extension(optional=True)` does
class optional_build_ext(build_ext):
    def run(self):
        try:
            build_ext.run(self)
        except DistutilsPlatformError as e:
            extensions = [ext.name for ext in self.extensions]
            print("WARNING: Failed to build extensions %r, skipping: %s" % (extensions, e))

    def build_extension(self, ext):
        try:
            build_ext.build_extension(self, ext)
        except build_ext_errors as e:
            print("WARNING: Failed to build extension %s, skipping: %s" % (ext.name, e))


def get_exts_for(name):
    try:
        mod = load_module_from_project_file(
            "ddtrace.vendor.{}.setup".format(name), "ddtrace/vendor/{}/setup.py".format(name)
        )
        return mod.get_extensions()
    except Exception as e:
        print("WARNING: Failed to load %s extensions, skipping: %s" % (name, e))
        return []


# Try to build with C extensions first, fallback to only pure-Python if building fails
try:
    all_exts = []
    for extname in ("msgpack", "wrapt"):
        exts = get_exts_for(extname)
        if exts:
            all_exts.extend(exts)

    kwargs = copy.deepcopy(setup_kwargs)
    kwargs["ext_modules"] = all_exts
    # DEV: Make sure `cmdclass` exists
    kwargs.setdefault("cmdclass", dict())
    kwargs["cmdclass"]["build_ext"] = optional_build_ext
    setup(**kwargs)
except Exception as e:
    # Set `DDTRACE_BUILD_TRACE=TRUE` in CI to raise any build errors
    if os.environ.get("DDTRACE_BUILD_RAISE") == "TRUE":
        raise

    print("WARNING: Failed to install with ddtrace C-extensions, falling back to pure-Python only extensions: %s" % e)
    setup(**setup_kwargs)
