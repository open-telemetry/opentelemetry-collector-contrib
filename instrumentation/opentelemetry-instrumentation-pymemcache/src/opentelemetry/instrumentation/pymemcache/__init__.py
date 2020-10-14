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

"""

Usage
-----

The OpenTelemetry ``pymemcache`` integration traces pymemcache client operations

Usage
-----

.. code-block:: python

    from opentelemetry import trace
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.instrumentation.pymemcache import PymemcacheInstrumentor
    trace.set_tracer_provider(TracerProvider())
    PymemcacheInstrumentor().instrument()
    from pymemcache.client.base import Client
    client = Client(('localhost', 11211))
    client.set('some_key', 'some_value')

API
---
"""
# pylint: disable=no-value-for-parameter

import logging

import pymemcache
from wrapt import ObjectProxy
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.pymemcache.version import __version__
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.trace import SpanKind, get_tracer

logger = logging.getLogger(__name__)

# Network attribute semantic convention here:
# https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/span-general.md#general-network-connection-attributes
_HOST = "net.peer.name"
_PORT = "net.peer.port"
# Database semantic conventions here:
# https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/database.md
_DB = "db.type"
_URL = "db.url"

_DEFAULT_SERVICE = "memcached"
_RAWCMD = "db.statement"
_CMD = "memcached.command"
COMMANDS = [
    "set",
    "set_many",
    "add",
    "replace",
    "append",
    "prepend",
    "cas",
    "get",
    "get_many",
    "gets",
    "gets_many",
    "delete",
    "delete_many",
    "incr",
    "decr",
    "touch",
    "stats",
    "version",
    "flush_all",
    "quit",
    "set_multi",
    "get_multi",
]


def _set_connection_attributes(span, instance):
    if not span.is_recording():
        return
    for key, value in _get_address_attributes(instance).items():
        span.set_attribute(key, value)


def _with_tracer_wrapper(func):
    """Helper for providing tracer for wrapper functions."""

    def _with_tracer(tracer, cmd):
        def wrapper(wrapped, instance, args, kwargs):
            # prevent double wrapping
            if hasattr(wrapped, "__wrapped__"):
                return wrapped(*args, **kwargs)

            return func(tracer, cmd, wrapped, instance, args, kwargs)

        return wrapper

    return _with_tracer


@_with_tracer_wrapper
def _wrap_cmd(tracer, cmd, wrapped, instance, args, kwargs):
    with tracer.start_as_current_span(
        _CMD, kind=SpanKind.INTERNAL, attributes={}
    ) as span:
        try:
            if span.is_recording():
                if not args:
                    vals = ""
                else:
                    vals = _get_query_string(args[0])

                query = "{}{}{}".format(cmd, " " if vals else "", vals)
                span.set_attribute(_RAWCMD, query)

                _set_connection_attributes(span, instance)
        except Exception as ex:  # pylint: disable=broad-except
            logger.warning(
                "Failed to set attributes for pymemcache span %s", str(ex)
            )

        return wrapped(*args, **kwargs)


def _get_query_string(arg):

    """Return the query values given the first argument to a pymemcache command.

    If there are multiple query values, they are joined together
    space-separated.
    """
    keys = ""

    if isinstance(arg, dict):
        arg = list(arg)

    if isinstance(arg, str):
        keys = arg
    elif isinstance(arg, bytes):
        keys = arg.decode()
    elif isinstance(arg, list) and len(arg) >= 1:
        if isinstance(arg[0], str):
            keys = " ".join(arg)
        elif isinstance(arg[0], bytes):
            keys = b" ".join(arg).decode()

    return keys


def _get_address_attributes(instance):
    """Attempt to get host and port from Client instance."""
    address_attributes = {}
    address_attributes[_DB] = "memcached"

    # client.base.Client contains server attribute which is either a host/port tuple, or unix socket path string
    # https://github.com/pinterest/pymemcache/blob/f02ddf73a28c09256589b8afbb3ee50f1171cac7/pymemcache/client/base.py#L228
    if hasattr(instance, "server"):
        if isinstance(instance.server, tuple):
            host, port = instance.server
            address_attributes[_HOST] = host
            address_attributes[_PORT] = port
            address_attributes[_URL] = "memcached://{}:{}".format(host, port)
        elif isinstance(instance.server, str):
            address_attributes[_URL] = "memcached://{}".format(instance.server)

    return address_attributes


class PymemcacheInstrumentor(BaseInstrumentor):
    """An instrumentor for pymemcache See `BaseInstrumentor`"""

    def _instrument(self, **kwargs):
        tracer_provider = kwargs.get("tracer_provider")
        tracer = get_tracer(__name__, __version__, tracer_provider)

        for cmd in COMMANDS:
            _wrap(
                "pymemcache.client.base",
                "Client.{}".format(cmd),
                _wrap_cmd(tracer, cmd),
            )

    def _uninstrument(self, **kwargs):
        for command in COMMANDS:
            unwrap(pymemcache.client.base.Client, "{}".format(command))
