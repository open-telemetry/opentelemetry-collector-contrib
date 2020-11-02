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

from unittest import mock

from opentelemetry import trace as trace_api
from opentelemetry.instrumentation.pymongo import (
    CommandTracer,
    PymongoInstrumentor,
)
from opentelemetry.test.test_base import TestBase


class TestPymongo(TestBase):
    def setUp(self):
        super().setUp()
        self.tracer = self.tracer_provider.get_tracer(__name__)

    def test_pymongo_instrumentor(self):
        mock_register = mock.Mock()
        patch = mock.patch(
            "pymongo.monitoring.register", side_effect=mock_register
        )
        with patch:
            PymongoInstrumentor().instrument()

        self.assertTrue(mock_register.called)

    def test_started(self):
        command_attrs = {
            "filter": "filter",
            "sort": "sort",
            "limit": "limit",
            "pipeline": "pipeline",
            "command_name": "find",
        }
        command_tracer = CommandTracer(self.tracer)
        mock_event = MockEvent(
            command_attrs, ("test.com", "1234"), "test_request_id"
        )
        command_tracer.started(event=mock_event)
        # the memory exporter can't be used here because the span isn't ended
        # yet
        # pylint: disable=protected-access
        span = command_tracer._pop_span(mock_event)
        self.assertIs(span.kind, trace_api.SpanKind.CLIENT)
        self.assertEqual(span.name, "mongodb.command_name.find")
        self.assertEqual(span.attributes["component"], "mongodb")
        self.assertEqual(span.attributes["db.type"], "mongodb")
        self.assertEqual(span.attributes["db.instance"], "database_name")
        self.assertEqual(span.attributes["db.statement"], "command_name find")
        self.assertEqual(span.attributes["net.peer.name"], "test.com")
        self.assertEqual(span.attributes["net.peer.port"], "1234")
        self.assertEqual(
            span.attributes["db.mongo.operation_id"], "operation_id"
        )
        self.assertEqual(
            span.attributes["db.mongo.request_id"], "test_request_id"
        )

        self.assertEqual(span.attributes["db.mongo.filter"], "filter")
        self.assertEqual(span.attributes["db.mongo.sort"], "sort")
        self.assertEqual(span.attributes["db.mongo.limit"], "limit")
        self.assertEqual(span.attributes["db.mongo.pipeline"], "pipeline")

    def test_succeeded(self):
        mock_event = MockEvent({})
        command_tracer = CommandTracer(self.tracer)
        command_tracer.started(event=mock_event)
        command_tracer.succeeded(event=mock_event)
        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]
        self.assertEqual(
            span.attributes["db.mongo.duration_micros"], "duration_micros"
        )
        self.assertIs(
            span.status.status_code, trace_api.status.StatusCode.UNSET
        )
        self.assertIsNotNone(span.end_time)

    def test_not_recording(self):
        mock_tracer = mock.Mock()
        mock_span = mock.Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        mock_tracer.use_span.return_value.__enter__ = mock_span
        mock_tracer.use_span.return_value.__exit__ = True
        mock_event = MockEvent({})
        command_tracer = CommandTracer(mock_tracer)
        command_tracer.started(event=mock_event)
        command_tracer.succeeded(event=mock_event)
        self.assertFalse(mock_span.is_recording())
        self.assertTrue(mock_span.is_recording.called)
        self.assertFalse(mock_span.set_attribute.called)
        self.assertFalse(mock_span.set_status.called)

    def test_failed(self):
        mock_event = MockEvent({})
        command_tracer = CommandTracer(self.tracer)
        command_tracer.started(event=mock_event)
        command_tracer.failed(event=mock_event)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertEqual(
            span.attributes["db.mongo.duration_micros"], "duration_micros"
        )
        self.assertIs(
            span.status.status_code, trace_api.status.StatusCode.ERROR,
        )
        self.assertEqual(span.status.description, "failure")
        self.assertIsNotNone(span.end_time)

    def test_multiple_commands(self):
        first_mock_event = MockEvent({}, ("firstUrl", "123"), "first")
        second_mock_event = MockEvent({}, ("secondUrl", "456"), "second")
        command_tracer = CommandTracer(self.tracer)
        command_tracer.started(event=first_mock_event)
        command_tracer.started(event=second_mock_event)
        command_tracer.succeeded(event=first_mock_event)
        command_tracer.failed(event=second_mock_event)

        spans_list = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans_list), 2)
        first_span = spans_list[0]
        second_span = spans_list[1]

        self.assertEqual(first_span.attributes["db.mongo.request_id"], "first")
        self.assertIs(
            first_span.status.status_code, trace_api.status.StatusCode.UNSET,
        )
        self.assertEqual(
            second_span.attributes["db.mongo.request_id"], "second"
        )
        self.assertIs(
            second_span.status.status_code, trace_api.status.StatusCode.ERROR,
        )

    def test_int_command(self):
        command_attrs = {
            "command_name": 123,
        }
        mock_event = MockEvent(command_attrs)

        command_tracer = CommandTracer(self.tracer)
        command_tracer.started(event=mock_event)
        command_tracer.succeeded(event=mock_event)

        spans_list = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        self.assertEqual(span.name, "mongodb.command_name.123")


class MockCommand:
    def __init__(self, command_attrs):
        self.command_attrs = command_attrs

    def get(self, key, default=""):
        return self.command_attrs.get(key, default)


class MockEvent:
    def __init__(self, command_attrs, connection_id=None, request_id=""):
        self.command = MockCommand(command_attrs)
        self.connection_id = connection_id
        self.request_id = request_id

    def __getattr__(self, item):
        return item
