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
The integration with MongoDB supports the `pymongo`_ library, it can be
enabled using the ``PymongoInstrumentor``.

.. _pymongo: https://pypi.org/project/pymongo

Usage
-----

.. code:: python

    from pymongo import MongoClient
    from opentelemetry import trace
    from opentelemetry.trace import TracerProvider
    from opentelemetry.instrumentation.pymongo import PymongoInstrumentor

    trace.set_tracer_provider(TracerProvider())

    PymongoInstrumentor().instrument()
    client = MongoClient()
    db = client["MongoDB_Database"]
    collection = db["MongoDB_Collection"]
    collection.find_one()

API
---
"""

from pymongo import monitoring

from opentelemetry import trace
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.pymongo.version import __version__
from opentelemetry.trace import SpanKind, get_tracer
from opentelemetry.trace.status import Status, StatusCode

DATABASE_TYPE = "mongodb"
COMMAND_ATTRIBUTES = ["filter", "sort", "skip", "limit", "pipeline"]


class CommandTracer(monitoring.CommandListener):
    def __init__(self, tracer):
        self._tracer = tracer
        self._span_dict = {}
        self.is_enabled = True

    def started(self, event: monitoring.CommandStartedEvent):
        """ Method to handle a pymongo CommandStartedEvent """
        if not self.is_enabled:
            return
        command = event.command.get(event.command_name, "")
        name = DATABASE_TYPE + "." + event.command_name
        statement = event.command_name
        if command:
            name += "." + str(command)
            statement += " " + str(command)

        try:
            span = self._tracer.start_span(name, kind=SpanKind.CLIENT)
            if span.is_recording():
                span.set_attribute("component", DATABASE_TYPE)
                span.set_attribute("db.type", DATABASE_TYPE)
                span.set_attribute("db.instance", event.database_name)
                span.set_attribute("db.statement", statement)
                if event.connection_id is not None:
                    span.set_attribute("net.peer.name", event.connection_id[0])
                    span.set_attribute("net.peer.port", event.connection_id[1])

                # pymongo specific, not specified by spec
                span.set_attribute("db.mongo.operation_id", event.operation_id)
                span.set_attribute("db.mongo.request_id", event.request_id)

                for attr in COMMAND_ATTRIBUTES:
                    _attr = event.command.get(attr)
                    if _attr is not None:
                        span.set_attribute("db.mongo." + attr, str(_attr))

            # Add Span to dictionary
            self._span_dict[_get_span_dict_key(event)] = span
        except Exception as ex:  # noqa pylint: disable=broad-except
            if span is not None and span.is_recording():
                span.set_status(Status(StatusCode.ERROR, str(ex)))
                span.end()
                self._pop_span(event)

    def succeeded(self, event: monitoring.CommandSucceededEvent):
        """ Method to handle a pymongo CommandSucceededEvent """
        if not self.is_enabled:
            return
        span = self._pop_span(event)
        if span is None:
            return
        if span.is_recording():
            span.set_attribute(
                "db.mongo.duration_micros", event.duration_micros
            )
        span.end()

    def failed(self, event: monitoring.CommandFailedEvent):
        """ Method to handle a pymongo CommandFailedEvent """
        if not self.is_enabled:
            return
        span = self._pop_span(event)
        if span is None:
            return
        if span.is_recording():
            span.set_attribute(
                "db.mongo.duration_micros", event.duration_micros
            )
            span.set_status(Status(StatusCode.ERROR, event.failure))
        span.end()

    def _pop_span(self, event):
        return self._span_dict.pop(_get_span_dict_key(event), None)


def _get_span_dict_key(event):
    if event.connection_id is not None:
        return (event.request_id, event.connection_id)
    return event.request_id


class PymongoInstrumentor(BaseInstrumentor):
    _commandtracer_instance = None  # type CommandTracer
    # The instrumentation for PyMongo is based on the event listener interface
    # https://api.mongodb.com/python/current/api/pymongo/monitoring.html.
    # This interface only allows to register listeners and does not provide
    # an unregister API. In order to provide a mechanishm to disable
    # instrumentation an enabled flag is implemented in CommandTracer,
    # it's checked in the different listeners.

    def _instrument(self, **kwargs):
        """Integrate with pymongo to trace it using event listener.
        https://api.mongodb.com/python/current/api/pymongo/monitoring.html

        Args:
            tracer_provider: The `TracerProvider` to use. If none is passed the
                current configured one is used.
        """

        tracer_provider = kwargs.get("tracer_provider")

        # Create and register a CommandTracer only the first time
        if self._commandtracer_instance is None:
            tracer = get_tracer(__name__, __version__, tracer_provider)

            self._commandtracer_instance = CommandTracer(tracer)
            monitoring.register(self._commandtracer_instance)

        # If already created, just enable it
        self._commandtracer_instance.is_enabled = True

    def _uninstrument(self, **kwargs):
        if self._commandtracer_instance is not None:
            self._commandtracer_instance.is_enabled = False
