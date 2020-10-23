# Copyright 2018, OpenCensus Authors
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

import unittest
from unittest import mock

# pylint:disable=no-name-in-module
# pylint:disable=import-error
import opentelemetry.exporter.jaeger as jaeger_exporter
from opentelemetry import trace as trace_api
from opentelemetry.exporter.jaeger.gen.jaeger import ttypes as jaeger
from opentelemetry.sdk import trace
from opentelemetry.sdk.trace import Resource
from opentelemetry.sdk.util.instrumentation import InstrumentationInfo
from opentelemetry.trace.status import Status, StatusCanonicalCode


class TestJaegerSpanExporter(unittest.TestCase):
    def setUp(self):
        # create and save span to be used in tests
        context = trace_api.SpanContext(
            trace_id=0x000000000000000000000000DEADBEEF,
            span_id=0x00000000DEADBEF0,
            is_remote=False,
        )

        self._test_span = trace._Span("test_span", context=context)
        self._test_span.start()
        self._test_span.end()

    def test_constructor_default(self):
        """Test the default values assigned by constructor."""
        service_name = "my-service-name"
        host_name = "localhost"
        thrift_port = None
        agent_port = 6831
        collector_endpoint = "/api/traces?format=jaeger.thrift"
        collector_protocol = "http"
        exporter = jaeger_exporter.JaegerSpanExporter(service_name)

        self.assertEqual(exporter.service_name, service_name)
        self.assertEqual(exporter.collector_host_name, None)
        self.assertEqual(exporter.agent_host_name, host_name)
        self.assertEqual(exporter.agent_port, agent_port)
        self.assertEqual(exporter.collector_port, thrift_port)
        self.assertEqual(exporter.collector_protocol, collector_protocol)
        self.assertEqual(exporter.collector_endpoint, collector_endpoint)
        self.assertEqual(exporter.username, None)
        self.assertEqual(exporter.password, None)
        self.assertTrue(exporter.collector is None)
        self.assertTrue(exporter.agent_client is not None)

    def test_constructor_explicit(self):
        """Test the constructor passing all the options."""
        service = "my-opentelemetry-jaeger"
        collector_host_name = "opentelemetry.io"
        collector_port = 15875
        collector_endpoint = "/myapi/traces?format=jaeger.thrift"
        collector_protocol = "https"

        agent_port = 14268
        agent_host_name = "opentelemetry.io"

        username = "username"
        password = "password"
        auth = (username, password)

        exporter = jaeger_exporter.JaegerSpanExporter(
            service_name=service,
            collector_host_name=collector_host_name,
            collector_port=collector_port,
            collector_endpoint=collector_endpoint,
            collector_protocol="https",
            agent_host_name=agent_host_name,
            agent_port=agent_port,
            username=username,
            password=password,
        )
        self.assertEqual(exporter.service_name, service)
        self.assertEqual(exporter.agent_host_name, agent_host_name)
        self.assertEqual(exporter.agent_port, agent_port)
        self.assertEqual(exporter.collector_host_name, collector_host_name)
        self.assertEqual(exporter.collector_port, collector_port)
        self.assertEqual(exporter.collector_protocol, collector_protocol)
        self.assertTrue(exporter.collector is not None)
        self.assertEqual(exporter.collector.auth, auth)
        # property should not construct new object
        collector = exporter.collector
        self.assertEqual(exporter.collector, collector)
        # property should construct new object
        # pylint: disable=protected-access
        exporter._collector = None
        exporter.username = None
        exporter.password = None
        self.assertNotEqual(exporter.collector, collector)
        self.assertTrue(exporter.collector.auth is None)

    def test_nsec_to_usec_round(self):
        # pylint: disable=protected-access
        nsec_to_usec_round = jaeger_exporter._nsec_to_usec_round

        self.assertEqual(nsec_to_usec_round(5000), 5)
        self.assertEqual(nsec_to_usec_round(5499), 5)
        self.assertEqual(nsec_to_usec_round(5500), 6)

    # pylint: disable=too-many-locals
    def test_translate_to_jaeger(self):
        # pylint: disable=invalid-name
        self.maxDiff = None

        span_names = ("test1", "test2", "test3")
        trace_id = 0x6E0C63257DE34C926F9EFCD03927272E
        trace_id_high = 0x6E0C63257DE34C92
        trace_id_low = 0x6F9EFCD03927272E
        span_id = 0x34BF92DEEFC58C92
        parent_id = 0x1111111111111111
        other_id = 0x2222222222222222

        base_time = 683647322 * 10 ** 9  # in ns
        start_times = (
            base_time,
            base_time + 150 * 10 ** 6,
            base_time + 300 * 10 ** 6,
        )
        durations = (50 * 10 ** 6, 100 * 10 ** 6, 200 * 10 ** 6)
        end_times = (
            start_times[0] + durations[0],
            start_times[1] + durations[1],
            start_times[2] + durations[2],
        )

        span_context = trace_api.SpanContext(
            trace_id, span_id, is_remote=False
        )
        parent_span_context = trace_api.SpanContext(
            trace_id, parent_id, is_remote=False
        )
        other_context = trace_api.SpanContext(
            trace_id, other_id, is_remote=False
        )

        event_attributes = {
            "annotation_bool": True,
            "annotation_string": "annotation_test",
            "key_float": 0.3,
        }

        event_timestamp = base_time + 50 * 10 ** 6
        event = trace.Event(
            name="event0",
            timestamp=event_timestamp,
            attributes=event_attributes,
        )

        link_attributes = {"key_bool": True}

        link = trace_api.Link(
            context=other_context, attributes=link_attributes
        )

        default_tags = [
            jaeger.Tag(
                key="status.code",
                vType=jaeger.TagType.LONG,
                vLong=StatusCanonicalCode.OK.value,
            ),
            jaeger.Tag(
                key="status.message", vType=jaeger.TagType.STRING, vStr=None
            ),
            jaeger.Tag(
                key="span.kind",
                vType=jaeger.TagType.STRING,
                vStr=trace_api.SpanKind.INTERNAL.name,
            ),
        ]

        otel_spans = [
            trace._Span(
                name=span_names[0],
                context=span_context,
                parent=parent_span_context,
                events=(event,),
                links=(link,),
                kind=trace_api.SpanKind.CLIENT,
            ),
            trace._Span(
                name=span_names[1], context=parent_span_context, parent=None
            ),
            trace._Span(
                name=span_names[2], context=other_context, parent=None
            ),
        ]

        otel_spans[0].start(start_time=start_times[0])
        # added here to preserve order
        otel_spans[0].set_attribute("key_bool", False)
        otel_spans[0].set_attribute("key_string", "hello_world")
        otel_spans[0].set_attribute("key_float", 111.22)
        otel_spans[0].set_attribute("key_tuple", ("tuple_element",))
        otel_spans[0].resource = Resource(
            attributes={"key_resource": "some_resource"}
        )
        otel_spans[0].set_status(
            Status(StatusCanonicalCode.UNKNOWN, "Example description")
        )
        otel_spans[0].end(end_time=end_times[0])

        otel_spans[1].start(start_time=start_times[1])
        otel_spans[1].resource = Resource({})
        otel_spans[1].end(end_time=end_times[1])

        otel_spans[2].start(start_time=start_times[2])
        otel_spans[2].resource = Resource({})
        otel_spans[2].end(end_time=end_times[2])
        otel_spans[2].instrumentation_info = InstrumentationInfo(
            name="name", version="version"
        )

        # pylint: disable=protected-access
        spans = jaeger_exporter._translate_to_jaeger(otel_spans)

        expected_spans = [
            jaeger.Span(
                operationName=span_names[0],
                traceIdHigh=trace_id_high,
                traceIdLow=trace_id_low,
                spanId=span_id,
                parentSpanId=parent_id,
                startTime=start_times[0] // 10 ** 3,
                duration=durations[0] // 10 ** 3,
                flags=0,
                tags=[
                    jaeger.Tag(
                        key="key_bool", vType=jaeger.TagType.BOOL, vBool=False
                    ),
                    jaeger.Tag(
                        key="key_string",
                        vType=jaeger.TagType.STRING,
                        vStr="hello_world",
                    ),
                    jaeger.Tag(
                        key="key_float",
                        vType=jaeger.TagType.DOUBLE,
                        vDouble=111.22,
                    ),
                    jaeger.Tag(
                        key="key_tuple",
                        vType=jaeger.TagType.STRING,
                        vStr="('tuple_element',)",
                    ),
                    jaeger.Tag(
                        key="key_resource",
                        vType=jaeger.TagType.STRING,
                        vStr="some_resource",
                    ),
                    jaeger.Tag(
                        key="status.code",
                        vType=jaeger.TagType.LONG,
                        vLong=StatusCanonicalCode.UNKNOWN.value,
                    ),
                    jaeger.Tag(
                        key="status.message",
                        vType=jaeger.TagType.STRING,
                        vStr="Example description",
                    ),
                    jaeger.Tag(
                        key="span.kind",
                        vType=jaeger.TagType.STRING,
                        vStr=trace_api.SpanKind.CLIENT.name,
                    ),
                    jaeger.Tag(
                        key="error", vType=jaeger.TagType.BOOL, vBool=True
                    ),
                ],
                references=[
                    jaeger.SpanRef(
                        refType=jaeger.SpanRefType.FOLLOWS_FROM,
                        traceIdHigh=trace_id_high,
                        traceIdLow=trace_id_low,
                        spanId=other_id,
                    )
                ],
                logs=[
                    jaeger.Log(
                        timestamp=event_timestamp // 10 ** 3,
                        fields=[
                            jaeger.Tag(
                                key="annotation_bool",
                                vType=jaeger.TagType.BOOL,
                                vBool=True,
                            ),
                            jaeger.Tag(
                                key="annotation_string",
                                vType=jaeger.TagType.STRING,
                                vStr="annotation_test",
                            ),
                            jaeger.Tag(
                                key="key_float",
                                vType=jaeger.TagType.DOUBLE,
                                vDouble=0.3,
                            ),
                            jaeger.Tag(
                                key="message",
                                vType=jaeger.TagType.STRING,
                                vStr="event0",
                            ),
                        ],
                    )
                ],
            ),
            jaeger.Span(
                operationName=span_names[1],
                traceIdHigh=trace_id_high,
                traceIdLow=trace_id_low,
                spanId=parent_id,
                parentSpanId=0,
                startTime=start_times[1] // 10 ** 3,
                duration=durations[1] // 10 ** 3,
                flags=0,
                tags=default_tags,
            ),
            jaeger.Span(
                operationName=span_names[2],
                traceIdHigh=trace_id_high,
                traceIdLow=trace_id_low,
                spanId=other_id,
                parentSpanId=0,
                startTime=start_times[2] // 10 ** 3,
                duration=durations[2] // 10 ** 3,
                flags=0,
                tags=[
                    jaeger.Tag(
                        key="status.code",
                        vType=jaeger.TagType.LONG,
                        vLong=StatusCanonicalCode.OK.value,
                    ),
                    jaeger.Tag(
                        key="status.message",
                        vType=jaeger.TagType.STRING,
                        vStr=None,
                    ),
                    jaeger.Tag(
                        key="span.kind",
                        vType=jaeger.TagType.STRING,
                        vStr=trace_api.SpanKind.INTERNAL.name,
                    ),
                    jaeger.Tag(
                        key="otel.instrumentation_library.name",
                        vType=jaeger.TagType.STRING,
                        vStr="name",
                    ),
                    jaeger.Tag(
                        key="otel.instrumentation_library.version",
                        vType=jaeger.TagType.STRING,
                        vStr="version",
                    ),
                ],
            ),
        ]

        # events are complicated to compare because order of fields
        # (attributes) in otel is not important but in jeager it is
        self.assertCountEqual(
            spans[0].logs[0].fields, expected_spans[0].logs[0].fields
        )
        # get rid of fields to be able to compare the whole spans
        spans[0].logs[0].fields = None
        expected_spans[0].logs[0].fields = None

        self.assertEqual(spans, expected_spans)

    def test_export(self):
        """Test that agent and/or collector are invoked"""
        exporter = jaeger_exporter.JaegerSpanExporter(
            "test_export", agent_host_name="localhost", agent_port=6318
        )

        # just agent is configured now
        agent_client_mock = mock.Mock(spec=jaeger_exporter.AgentClientUDP)
        # pylint: disable=protected-access
        exporter._agent_client = agent_client_mock

        exporter.export((self._test_span,))
        self.assertEqual(agent_client_mock.emit.call_count, 1)

        # add also a collector and test that both are called
        collector_mock = mock.Mock(spec=jaeger_exporter.Collector)
        # pylint: disable=protected-access
        exporter._collector = collector_mock

        exporter.export((self._test_span,))
        self.assertEqual(agent_client_mock.emit.call_count, 1)
        self.assertEqual(collector_mock.submit.call_count, 1)

    def test_agent_client(self):
        agent_client = jaeger_exporter.AgentClientUDP(
            host_name="localhost", port=6354
        )

        batch = jaeger.Batch(
            # pylint: disable=protected-access
            spans=jaeger_exporter._translate_to_jaeger((self._test_span,)),
            process=jaeger.Process(serviceName="xxx"),
        )

        agent_client.emit(batch)
