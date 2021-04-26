#!/usr/bin/env python3

# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import argparse
import pkgutil
import subprocess
import sys
from logging import getLogger

from opentelemetry.instrumentation.version import __version__ as version

logger = getLogger(__file__)


# A mapping of "target library" to "desired instrumentor path/versioned package
# name". Used as part of the `opentelemetry-bootstrap` command which looks at
# libraries used by the application that is to be instrumented, and handles
# automatically installing the appropriate instrumentations for that app.
# This helps for those who prefer to turn on as much instrumentation as
# possible, and don't want to go through the manual process of combing through
# the libraries their application uses to figure which one can be
# instrumented.
# NOTE: system-metrics is not to be included.
def all_instrumentations():
    pkg_instrumentation_map = {
        "aiohttp-client": "opentelemetry-instrumentation-aiohttp-client",
        "aiopg": "opentelemetry-instrumentation-aiopg",
        "asyncpg": "opentelemetry-instrumentation-asyncpg",
        "boto": "opentelemetry-instrumentation-boto",
        "botocore": "opentelemetry-instrumentation-botocore",
        "celery": "opentelemetry-instrumentation-celery",
        "dbapi": "opentelemetry-instrumentation-dbapi",
        "django": "opentelemetry-instrumentation-django",
        "elasticsearch": "opentelemetry-instrumentation-elasticsearch",
        "falcon": "opentelemetry-instrumentation-falcon",
        "fastapi": "opentelemetry-instrumentation-fastapi",
        "flask": "opentelemetry-instrumentation-flask",
        "grpc": "opentelemetry-instrumentation-grpc",
        "jinja2": "opentelemetry-instrumentation-jinja2",
        "mysql": "opentelemetry-instrumentation-mysql",
        "psycopg2": "opentelemetry-instrumentation-psycopg2",
        "pymemcache": "opentelemetry-instrumentation-pymemcache",
        "pymongo": "opentelemetry-instrumentation-pymongo",
        "pymysql": "opentelemetry-instrumentation-pymysql",
        "pyramid": "opentelemetry-instrumentation-pyramid",
        "redis": "opentelemetry-instrumentation-redis",
        "requests": "opentelemetry-instrumentation-requests",
        "sklearn": "opentelemetry-instrumentation-sklearn",
        "sqlalchemy": "opentelemetry-instrumentation-sqlalchemy",
        "sqlite3": "opentelemetry-instrumentation-sqlite3",
        "starlette": "opentelemetry-instrumentation-starlette",
        "tornado": "opentelemetry-instrumentation-tornado",
        "urllib": "opentelemetry-instrumentation-urllib",
    }
    for pkg, instrumentation in pkg_instrumentation_map.items():
        pkg_instrumentation_map[pkg] = "{0}=={1}".format(
            instrumentation, version
        )
    return pkg_instrumentation_map


instrumentations = all_instrumentations()

# relevant instrumentors and tracers to uninstall and check for conflicts for target libraries
libraries = {
    "aiohttp-client": ("opentelemetry-instrumentation-aiohttp-client",),
    "aiopg": ("opentelemetry-instrumentation-aiopg",),
    "asyncpg": ("opentelemetry-instrumentation-asyncpg",),
    "boto": ("opentelemetry-instrumentation-boto",),
    "botocore": ("opentelemetry-instrumentation-botocore",),
    "celery": ("opentelemetry-instrumentation-celery",),
    "dbapi": ("opentelemetry-instrumentation-dbapi",),
    "django": ("opentelemetry-instrumentation-django",),
    "elasticsearch": ("opentelemetry-instrumentation-elasticsearch",),
    "falcon": ("opentelemetry-instrumentation-falcon",),
    "fastapi": ("opentelemetry-instrumentation-fastapi",),
    "flask": ("opentelemetry-instrumentation-flask",),
    "grpc": ("opentelemetry-instrumentation-grpc",),
    "jinja2": ("opentelemetry-instrumentation-jinja2",),
    "mysql": ("opentelemetry-instrumentation-mysql",),
    "psycopg2": ("opentelemetry-instrumentation-psycopg2",),
    "pymemcache": ("opentelemetry-instrumentation-pymemcache",),
    "pymongo": ("opentelemetry-instrumentation-pymongo",),
    "pymysql": ("opentelemetry-instrumentation-pymysql",),
    "pyramid": ("opentelemetry-instrumentation-pyramid",),
    "redis": ("opentelemetry-instrumentation-redis",),
    "requests": ("opentelemetry-instrumentation-requests",),
    "sklearn": ("opentelemetry-instrumentation-sklearn",),
    "sqlalchemy": ("opentelemetry-instrumentation-sqlalchemy",),
    "sqlite3": ("opentelemetry-instrumentation-sqlite3",),
    "starlette": ("opentelemetry-instrumentation-starlette",),
    "tornado": ("opentelemetry-instrumentation-tornado",),
    "urllib": ("opentelemetry-instrumentation-urllib",),
}


def _install_package(library, instrumentation):
    """
    Ensures that desired version is installed w/o upgrading its dependencies
    by uninstalling where necessary (if `target` is not provided).


    OpenTelemetry auto-instrumentation packages often have traced libraries
    as instrumentation dependency (e.g. flask for
    opentelemetry-instrumentation-flask), so using -I on library could cause
    likely undesired Flask upgrade.Using --no-dependencies alone would leave
    potential for nonfunctional installations.
    """
    pip_list = _sys_pip_freeze()
    for package in libraries[library]:
        if "{}==".format(package).lower() in pip_list:
            logger.info(
                "Existing %s installation detected.  Uninstalling.", package
            )
            _sys_pip_uninstall(package)
    _sys_pip_install(instrumentation)


def _syscall(func):
    def wrapper(package=None):
        try:
            if package:
                return func(package)
            return func()
        except subprocess.SubprocessError as exp:
            cmd = getattr(exp, "cmd", None)
            if cmd:
                msg = 'Error calling system command "{0}"'.format(
                    " ".join(cmd)
                )
            if package:
                msg = '{0} for package "{1}"'.format(msg, package)
            raise RuntimeError(msg)

    return wrapper


@_syscall
def _sys_pip_freeze():
    return (
        subprocess.check_output([sys.executable, "-m", "pip", "freeze"])
        .decode()
        .lower()
    )


@_syscall
def _sys_pip_install(package):
    # explicit upgrade strategy to override potential pip config
    subprocess.check_call(
        [
            sys.executable,
            "-m",
            "pip",
            "install",
            "-U",
            "--upgrade-strategy",
            "only-if-needed",
            package,
        ]
    )


@_syscall
def _sys_pip_uninstall(package):
    subprocess.check_call(
        [sys.executable, "-m", "pip", "uninstall", "-y", package]
    )


def _pip_check():
    """Ensures none of the instrumentations have dependency conflicts.
    Clean check reported as:
    'No broken requirements found.'
    Dependency conflicts are reported as:
    'opentelemetry-instrumentation-flask 1.0.1 has requirement opentelemetry-sdk<2.0,>=1.0, but you have opentelemetry-sdk 0.5.'
    To not be too restrictive, we'll only check for relevant packages.
    """
    # pylint: disable=consider-using-with
    check_pipe = subprocess.Popen(
        [sys.executable, "-m", "pip", "check"], stdout=subprocess.PIPE
    )
    pip_check = check_pipe.communicate()[0].decode()
    pip_check_lower = pip_check.lower()
    for package_tup in libraries.values():
        for package in package_tup:
            if package.lower() in pip_check_lower:
                raise RuntimeError(
                    "Dependency conflict found: {}".format(pip_check)
                )


def _is_installed(library):
    return library in sys.modules or pkgutil.find_loader(library) is not None


def _find_installed_libraries():
    return {k: v for k, v in instrumentations.items() if _is_installed(k)}


def _run_requirements(packages):
    print("\n".join(packages.values()), end="")


def _run_install(packages):
    for pkg, inst in packages.items():
        _install_package(pkg, inst)

    _pip_check()


def run() -> None:
    action_install = "install"
    action_requirements = "requirements"

    parser = argparse.ArgumentParser(
        description="""
        opentelemetry-bootstrap detects installed libraries and automatically
        installs the relevant instrumentation packages for them.
        """
    )
    parser.add_argument(
        "-a",
        "--action",
        choices=[action_install, action_requirements],
        default=action_requirements,
        help="""
        install - uses pip to install the new requirements using to the
                  currently active site-package.
        requirements - prints out the new requirements to stdout. Action can
                       be piped and appended to a requirements.txt file.
        """,
    )
    args = parser.parse_args()

    cmd = {
        action_install: _run_install,
        action_requirements: _run_requirements,
    }[args.action]
    cmd(_find_installed_libraries())
