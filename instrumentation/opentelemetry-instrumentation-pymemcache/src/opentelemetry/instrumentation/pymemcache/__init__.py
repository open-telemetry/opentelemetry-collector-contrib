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

    from opentelemetry.instrumentation.pymemcache import PymemcacheInstrumentor

    PymemcacheInstrumentor().instrument()

    from pymemcache.client.base import Client
    client = Client(('localhost', 11211))
    client.set('some_key', 'some_value')

API
---
"""
# pylint: disable=no-value-for-parameter

import logging
from typing import Collection

import pymemcache
from wrapt import ObjectProxy
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.pymemcache.package import _instruments
from opentelemetry.instrumentation.pymemcache.version import __version__
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.semconv.trace import (
    DbSystemValues,
    NetTransportValues,
    SpanAttributes,
)
from opentelemetry.trace import SpanKind, get_tracer

logger = logging.getLogger(__name__)


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
        cmd, kind=SpanKind.CLIENT, attributes={}
    ) as span:
        try:
            if span.is_recording():
                if not args:
                    vals = ""
                else:
                    vals = _get_query_string(args[0])

                query = "{}{}{}".format(cmd, " " if vals else "", vals)
                span.set_attribute(SpanAttributes.DB_STATEMENT, query)

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
    address_attributes[SpanAttributes.DB_SYSTEM] = "memcached"

    # client.base.Client contains server attribute which is either a host/port tuple, or unix socket path string
    # https://github.com/pinterest/pymemcache/blob/f02ddf73a28c09256589b8afbb3ee50f1171cac7/pymemcache/client/base.py#L228
    if hasattr(instance, "server"):
        if isinstance(instance.server, tuple):
            host, port = instance.server
            address_attributes[SpanAttributes.NET_PEER_NAME] = host
            address_attributes[SpanAttributes.NET_PEER_PORT] = port
            address_attributes[
                SpanAttributes.NET_TRANSPORT
            ] = NetTransportValues.IP_TCP.value
        elif isinstance(instance.server, str):
            address_attributes[SpanAttributes.NET_PEER_NAME] = instance.server
            address_attributes[
                SpanAttributes.NET_TRANSPORT
            ] = NetTransportValues.UNIX.value

    return address_attributes


class PymemcacheInstrumentor(BaseInstrumentor):
    """An instrumentor for pymemcache See `BaseInstrumentor`"""

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

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
