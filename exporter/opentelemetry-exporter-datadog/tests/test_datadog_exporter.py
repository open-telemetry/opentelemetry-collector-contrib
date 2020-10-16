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

import itertools
import logging
import time
import unittest
from unittest import mock

from ddtrace.internal.writer import AgentWriter

from opentelemetry import trace as trace_api
from opentelemetry.context import Context
from opentelemetry.exporter import datadog
from opentelemetry.sdk import trace
from opentelemetry.sdk.trace import Resource, sampling
from opentelemetry.sdk.util.instrumentation import InstrumentationInfo


class MockDatadogSpanExporter(datadog.DatadogSpanExporter):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        agent_writer_mock = mock.Mock(spec=AgentWriter)
        agent_writer_mock.started = True
        agent_writer_mock.exit_timeout = 1
        self._agent_writer = agent_writer_mock


def get_spans(tracer, exporter, shutdown=True):
    if shutdown:
        tracer.source.shutdown()

    spans = [
        call_args[-1]["spans"]
        for call_args in exporter.agent_writer.write.call_args_list
    ]

    return [span.to_dict() for span in itertools.chain.from_iterable(spans)]


class TestDatadogSpanExporter(unittest.TestCase):
    def setUp(self):
        self.exporter = MockDatadogSpanExporter()
        self.span_processor = datadog.DatadogExportSpanProcessor(self.exporter)
        tracer_provider = trace.TracerProvider()
        tracer_provider.add_span_processor(self.span_processor)
        self.tracer_provider = tracer_provider
        self.tracer = tracer_provider.get_tracer(__name__)

    def tearDown(self):
        self.tracer_provider.shutdown()

    def test_constructor_default(self):
        """Test the default values assigned by constructor."""
        exporter = datadog.DatadogSpanExporter()

        self.assertEqual(exporter.agent_url, "http://localhost:8126")
        self.assertIsNone(exporter.service)
        self.assertIsNotNone(exporter.agent_writer)

    def test_constructor_explicit(self):
        """Test the constructor passing all the options."""
        agent_url = "http://localhost:8126"
        exporter = datadog.DatadogSpanExporter(
            agent_url=agent_url, service="explicit",
        )

        self.assertEqual(exporter.agent_url, agent_url)
        self.assertEqual(exporter.service, "explicit")
        self.assertIsNone(exporter.env)
        self.assertIsNone(exporter.version)
        self.assertEqual(exporter.tags, {})

        exporter = datadog.DatadogSpanExporter(
            agent_url=agent_url,
            service="explicit",
            env="test",
            version="0.0.1",
            tags="",
        )

        self.assertEqual(exporter.agent_url, agent_url)
        self.assertEqual(exporter.service, "explicit")
        self.assertEqual(exporter.env, "test")
        self.assertEqual(exporter.version, "0.0.1")
        self.assertEqual(exporter.tags, {})

        exporter = datadog.DatadogSpanExporter(
            agent_url=agent_url,
            service="explicit",
            env="test",
            version="0.0.1",
            tags="team:testers,layer:app",
        )

        self.assertEqual(exporter.agent_url, agent_url)
        self.assertEqual(exporter.service, "explicit")
        self.assertEqual(exporter.env, "test")
        self.assertEqual(exporter.version, "0.0.1")
        self.assertEqual(exporter.tags, {"team": "testers", "layer": "app"})

    @mock.patch.dict(
        "os.environ",
        {
            "DD_TRACE_AGENT_URL": "http://agent:8126",
            "DD_SERVICE": "test-service",
            "DD_ENV": "test",
            "DD_VERSION": "0.0.1",
            "DD_TAGS": "team:testers",
        },
    )
    def test_constructor_environ(self):
        exporter = datadog.DatadogSpanExporter()

        self.assertEqual(exporter.agent_url, "http://agent:8126")
        self.assertEqual(exporter.service, "test-service")
        self.assertEqual(exporter.env, "test")
        self.assertEqual(exporter.version, "0.0.1")
        self.assertEqual(exporter.tags, {"team": "testers"})
        self.assertIsNotNone(exporter.agent_writer)

    # pylint: disable=too-many-locals
    @mock.patch.dict(
        "os.environ",
        {
            "DD_SERVICE": "test-service",
            "DD_ENV": "test",
            "DD_VERSION": "0.0.1",
            "DD_TAGS": "team:testers",
        },
    )
    def test_translate_to_datadog(self):
        # pylint: disable=invalid-name
        self.maxDiff = None

        resource = Resource(
            attributes={
                "key_resource": "some_resource",
                "service.name": "resource_service_name",
            }
        )

        resource_without_service = Resource(
            attributes={"conflicting_key": "conflicting_value"}
        )

        span_names = ("test1", "test2", "test3")
        trace_id = 0x6E0C63257DE34C926F9EFCD03927272E
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

        instrumentation_info = InstrumentationInfo(__name__, "0")

        otel_spans = [
            trace._Span(
                name=span_names[0],
                context=span_context,
                parent=parent_span_context,
                kind=trace_api.SpanKind.CLIENT,
                instrumentation_info=instrumentation_info,
                resource=Resource({}),
            ),
            trace._Span(
                name=span_names[1],
                context=parent_span_context,
                parent=None,
                instrumentation_info=instrumentation_info,
                resource=resource_without_service,
            ),
            trace._Span(
                name=span_names[2],
                context=other_context,
                parent=None,
                resource=resource,
            ),
        ]

        otel_spans[1].set_attribute("conflicting_key", "original_value")

        otel_spans[0].start(start_time=start_times[0])
        otel_spans[0].end(end_time=end_times[0])

        otel_spans[1].start(start_time=start_times[1])
        otel_spans[1].end(end_time=end_times[1])

        otel_spans[2].start(start_time=start_times[2])
        otel_spans[2].end(end_time=end_times[2])

        # pylint: disable=protected-access
        exporter = datadog.DatadogSpanExporter()
        datadog_spans = [
            span.to_dict()
            for span in exporter._translate_to_datadog(otel_spans)
        ]

        expected_spans = [
            dict(
                trace_id=trace_id_low,
                parent_id=parent_id,
                span_id=span_id,
                name="tests.test_datadog_exporter.CLIENT",
                resource=span_names[0],
                start=start_times[0],
                duration=durations[0],
                error=0,
                service="test-service",
                meta={"env": "test", "team": "testers"},
            ),
            dict(
                trace_id=trace_id_low,
                parent_id=0,
                span_id=parent_id,
                name="tests.test_datadog_exporter.INTERNAL",
                resource=span_names[1],
                start=start_times[1],
                duration=durations[1],
                error=0,
                service="test-service",
                meta={
                    "env": "test",
                    "team": "testers",
                    "version": "0.0.1",
                    "conflicting_key": "original_value",
                },
            ),
            dict(
                trace_id=trace_id_low,
                parent_id=0,
                span_id=other_id,
                name=span_names[2],
                resource=span_names[2],
                start=start_times[2],
                duration=durations[2],
                error=0,
                service="resource_service_name",
                meta={
                    "env": "test",
                    "team": "testers",
                    "version": "0.0.1",
                    "key_resource": "some_resource",
                },
            ),
        ]

        self.assertEqual(datadog_spans, expected_spans)

    def test_export(self):
        """Test that agent and/or collector are invoked"""
        # create and save span to be used in tests
        context = trace_api.SpanContext(
            trace_id=0x000000000000000000000000DEADBEEF,
            span_id=0x00000000DEADBEF0,
            is_remote=False,
        )

        test_span = trace._Span("test_span", context=context)
        test_span.start()
        test_span.end()

        self.exporter.export((test_span,))

        self.assertEqual(self.exporter.agent_writer.write.call_count, 1)

    def test_resources(self):
        test_attributes = [
            {},
            {"http.method": "GET", "http.route": "/foo/<int:id>"},
            {"http.method": "GET", "http.target": "/foo/200"},
        ]

        for index, test in enumerate(test_attributes):
            with self.tracer.start_span(str(index), attributes=test):
                pass

        datadog_spans = get_spans(self.tracer, self.exporter)

        self.assertEqual(len(datadog_spans), 3)

        actual = [span["resource"] for span in datadog_spans]
        expected = ["0", "GET /foo/<int:id>", "GET"]

        self.assertEqual(actual, expected)

    def test_span_types(self):
        test_instrumentations = [
            "opentelemetry.instrumentation.aiohttp-client",
            "opentelemetry.instrumentation.dbapi",
            "opentelemetry.instrumentation.django",
            "opentelemetry.instrumentation.flask",
            "opentelemetry.instrumentation.grpc",
            "opentelemetry.instrumentation.jinja2",
            "opentelemetry.instrumentation.mysql",
            "opentelemetry.instrumentation.psycopg2",
            "opentelemetry.instrumentation.pymongo",
            "opentelemetry.instrumentation.pymysql",
            "opentelemetry.instrumentation.redis",
            "opentelemetry.instrumentation.requests",
            "opentelemetry.instrumentation.sqlalchemy",
            "opentelemetry.instrumentation.wsgi",
        ]

        for index, instrumentation in enumerate(test_instrumentations):
            # change tracer's instrumentation info before starting span
            self.tracer.instrumentation_info = InstrumentationInfo(
                instrumentation, "0"
            )
            with self.tracer.start_span(str(index)):
                pass

        datadog_spans = get_spans(self.tracer, self.exporter)

        self.assertEqual(len(datadog_spans), 14)

        actual = [span.get("type") for span in datadog_spans]
        expected = [
            "http",
            "sql",
            "web",
            "web",
            "grpc",
            "template",
            "sql",
            "sql",
            "mongodb",
            "sql",
            "redis",
            "http",
            "sql",
            "web",
        ]
        self.assertEqual(actual, expected)

    def test_errors(self):
        with self.assertRaises(ValueError):
            with self.tracer.start_span("foo"):
                raise ValueError("bar")

        datadog_spans = get_spans(self.tracer, self.exporter)

        self.assertEqual(len(datadog_spans), 1)

        span = datadog_spans[0]
        self.assertEqual(span["error"], 1)
        self.assertEqual(span["meta"]["error.msg"], "bar")
        self.assertEqual(span["meta"]["error.type"], "ValueError")

    def test_shutdown(self):
        span_names = ["xxx", "bar", "foo"]

        for name in span_names:
            with self.tracer.start_span(name):
                pass

        self.span_processor.shutdown()

        # check that spans are exported without an explicitly call to
        # force_flush()
        datadog_spans = get_spans(self.tracer, self.exporter)
        actual = [span.get("resource") for span in datadog_spans]
        self.assertListEqual(span_names, actual)

    def test_flush(self):
        span_names0 = ["xxx", "bar", "foo"]
        span_names1 = ["yyy", "baz", "fox"]

        for name in span_names0:
            with self.tracer.start_span(name):
                pass

        self.assertTrue(self.span_processor.force_flush())
        datadog_spans = get_spans(self.tracer, self.exporter, shutdown=False)
        actual0 = [span.get("resource") for span in datadog_spans]
        self.assertListEqual(span_names0, actual0)

        # create some more spans to check that span processor still works
        for name in span_names1:
            with self.tracer.start_span(name):
                pass

        self.assertTrue(self.span_processor.force_flush())
        datadog_spans = get_spans(self.tracer, self.exporter)
        actual1 = [span.get("resource") for span in datadog_spans]
        self.assertListEqual(span_names0 + span_names1, actual1)

    def test_span_processor_lossless(self):
        """Test that no spans are lost when sending max_trace_size spans"""
        span_processor = datadog.DatadogExportSpanProcessor(
            self.exporter, max_trace_size=128
        )
        tracer_provider = trace.TracerProvider()
        tracer_provider.add_span_processor(span_processor)
        tracer = tracer_provider.get_tracer(__name__)

        with tracer.start_as_current_span("root"):
            for _ in range(127):
                with tracer.start_span("foo"):
                    pass

        self.assertTrue(span_processor.force_flush())
        datadog_spans = get_spans(tracer, self.exporter)
        self.assertEqual(len(datadog_spans), 128)
        tracer_provider.shutdown()

    def test_span_processor_dropped_spans(self):
        """Test that spans are lost when exceeding max_trace_size spans"""
        span_processor = datadog.DatadogExportSpanProcessor(
            self.exporter, max_trace_size=128
        )
        tracer_provider = trace.TracerProvider()
        tracer_provider.add_span_processor(span_processor)
        tracer = tracer_provider.get_tracer(__name__)

        with tracer.start_as_current_span("root"):
            for _ in range(127):
                with tracer.start_span("foo"):
                    pass
            with self.assertLogs(level=logging.WARNING):
                with tracer.start_span("one-too-many"):
                    pass

        self.assertTrue(span_processor.force_flush())
        datadog_spans = get_spans(tracer, self.exporter)
        self.assertEqual(len(datadog_spans), 128)
        tracer_provider.shutdown()

    def test_span_processor_scheduled_delay(self):
        """Test that spans are exported each schedule_delay_millis"""
        delay = 300
        span_processor = datadog.DatadogExportSpanProcessor(
            self.exporter, schedule_delay_millis=delay
        )
        tracer_provider = trace.TracerProvider()
        tracer_provider.add_span_processor(span_processor)
        tracer = tracer_provider.get_tracer(__name__)

        with tracer.start_span("foo"):
            pass

        time.sleep(delay / (1e3 * 2))
        datadog_spans = get_spans(tracer, self.exporter, shutdown=False)
        self.assertEqual(len(datadog_spans), 0)

        time.sleep(delay / (1e3 * 2) + 0.01)
        datadog_spans = get_spans(tracer, self.exporter, shutdown=False)
        self.assertEqual(len(datadog_spans), 1)

        tracer_provider.shutdown()

    def test_span_processor_accepts_parent_context(self):
        span_processor = mock.Mock(
            wraps=datadog.DatadogExportSpanProcessor(self.exporter)
        )
        tracer_provider = trace.TracerProvider()
        tracer_provider.add_span_processor(span_processor)
        tracer = tracer_provider.get_tracer(__name__)

        context = Context()
        span = tracer.start_span("foo", context=context)

        span_processor.on_start.assert_called_once_with(
            span, parent_context=context
        )

    def test_origin(self):
        context = trace_api.SpanContext(
            trace_id=0x000000000000000000000000DEADBEEF,
            span_id=trace_api.INVALID_SPAN,
            is_remote=True,
            trace_state=trace_api.TraceState(
                {datadog.constants.DD_ORIGIN: "origin-service"}
            ),
        )

        root_span = trace._Span(name="root", context=context, parent=None)
        child_span = trace._Span(
            name="child", context=context, parent=root_span
        )
        root_span.start()
        child_span.start()
        child_span.end()
        root_span.end()

        # pylint: disable=protected-access
        exporter = datadog.DatadogSpanExporter()
        datadog_spans = [
            span.to_dict()
            for span in exporter._translate_to_datadog([root_span, child_span])
        ]

        self.assertEqual(len(datadog_spans), 2)

        actual = [
            span["meta"].get(datadog.constants.DD_ORIGIN)
            if "meta" in span
            else None
            for span in datadog_spans
        ]
        expected = ["origin-service", None]
        self.assertListEqual(actual, expected)

    def test_sampling_rate(self):
        context = trace_api.SpanContext(
            trace_id=0x000000000000000000000000DEADBEEF,
            span_id=0x34BF92DEEFC58C92,
            is_remote=False,
            trace_flags=trace_api.TraceFlags(trace_api.TraceFlags.SAMPLED),
        )
        sampler = sampling.TraceIdRatioBased(0.5)

        span = trace._Span(
            name="sampled", context=context, parent=None, sampler=sampler
        )
        span.start()
        span.end()

        # pylint: disable=protected-access
        exporter = datadog.DatadogSpanExporter()
        datadog_spans = [
            span.to_dict() for span in exporter._translate_to_datadog([span])
        ]

        self.assertEqual(len(datadog_spans), 1)

        actual = [
            span["metrics"].get(datadog.constants.SAMPLE_RATE_METRIC_KEY)
            if "metrics" in span
            else None
            for span in datadog_spans
        ]
        expected = [0.5]
        self.assertListEqual(actual, expected)
