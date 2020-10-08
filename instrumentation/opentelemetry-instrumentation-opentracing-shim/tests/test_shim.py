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

# TODO: make pylint use 3p opentracing module for type inference
# pylint:disable=no-member

import time
from unittest import TestCase
from unittest.mock import Mock

import opentracing

from opentelemetry import propagators, trace
from opentelemetry.instrumentation.opentracing_shim import (
    SpanContextShim,
    SpanShim,
    create_tracer,
    util,
)
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.test.mock_textmap import (
    MockTextMapPropagator,
    NOOPTextMapPropagator,
)


class TestShim(TestCase):
    # pylint: disable=too-many-public-methods

    def setUp(self):
        """Create an OpenTelemetry tracer and a shim before every test case."""
        trace.set_tracer_provider(TracerProvider())
        self.shim = create_tracer(trace.get_tracer_provider())

    @classmethod
    def setUpClass(cls):
        # Save current propagator to be restored on teardown.
        cls._previous_propagator = propagators.get_global_textmap()

        # Set mock propagator for testing.
        propagators.set_global_textmap(MockTextMapPropagator())

    @classmethod
    def tearDownClass(cls):
        # Restore previous propagator.
        propagators.set_global_textmap(cls._previous_propagator)

    def test_shim_type(self):
        # Verify shim is an OpenTracing tracer.
        self.assertIsInstance(self.shim, opentracing.Tracer)

    def test_start_active_span(self):
        """Test span creation and activation using `start_active_span()`."""

        with self.shim.start_active_span("TestSpan0") as scope:
            # Verify correct type of Scope and Span objects.
            self.assertIsInstance(scope, opentracing.Scope)
            self.assertIsInstance(scope.span, opentracing.Span)

            # Verify span is started.
            self.assertIsNotNone(scope.span.unwrap().start_time)

            # Verify span is active.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                scope.span.context.unwrap(),
            )
            # TODO: We can't check for equality of self.shim.active_span and
            # scope.span because the same OpenTelemetry span is returned inside
            # different SpanShim objects. A possible solution is described
            # here:
            # https://github.com/open-telemetry/opentelemetry-python/issues/161#issuecomment-534136274

        # Verify span has ended.
        self.assertIsNotNone(scope.span.unwrap().end_time)

        # Verify no span is active.
        self.assertIsNone(self.shim.active_span)

    def test_start_span(self):
        """Test span creation using `start_span()`."""

        with self.shim.start_span("TestSpan1") as span:
            # Verify correct type of Span object.
            self.assertIsInstance(span, opentracing.Span)

            # Verify span is started.
            self.assertIsNotNone(span.unwrap().start_time)

            # Verify `start_span()` does NOT make the span active.
            self.assertIsNone(self.shim.active_span)

        # Verify span has ended.
        self.assertIsNotNone(span.unwrap().end_time)

    def test_start_span_no_contextmanager(self):
        """Test `start_span()` without a `with` statement."""

        span = self.shim.start_span("TestSpan2")

        # Verify span is started.
        self.assertIsNotNone(span.unwrap().start_time)

        # Verify `start_span()` does NOT make the span active.
        self.assertIsNone(self.shim.active_span)

        span.finish()

    def test_explicit_span_finish(self):
        """Test `finish()` method on `Span` objects."""

        span = self.shim.start_span("TestSpan3")

        # Verify span hasn't ended.
        self.assertIsNone(span.unwrap().end_time)

        span.finish()

        # Verify span has ended.
        self.assertIsNotNone(span.unwrap().end_time)

    def test_explicit_start_time(self):
        """Test `start_time` argument."""

        now = time.time()
        with self.shim.start_active_span("TestSpan4", start_time=now) as scope:
            result = util.time_seconds_from_ns(scope.span.unwrap().start_time)
            # Tolerate inaccuracies of less than a microsecond. See Note:
            # https://open-telemetry.github.io/opentelemetry-python/opentelemetry.instrumentation.opentracing_shim.html
            # TODO: This seems to work consistently, but we should find out the
            # biggest possible loss of precision.
            self.assertAlmostEqual(result, now, places=6)

    def test_explicit_end_time(self):
        """Test `end_time` argument of `finish()` method."""

        span = self.shim.start_span("TestSpan5")
        now = time.time()
        span.finish(now)

        end_time = util.time_seconds_from_ns(span.unwrap().end_time)
        # Tolerate inaccuracies of less than a microsecond. See Note:
        # https://open-telemetry.github.io/opentelemetry-python/opentelemetry.instrumentation.opentracing_shim.html
        # TODO: This seems to work consistently, but we should find out the
        # biggest possible loss of precision.
        self.assertAlmostEqual(end_time, now, places=6)

    def test_explicit_span_activation(self):
        """Test manual activation and deactivation of a span."""

        span = self.shim.start_span("TestSpan6")

        # Verify no span is currently active.
        self.assertIsNone(self.shim.active_span)

        with self.shim.scope_manager.activate(
            span, finish_on_close=True
        ) as scope:
            # Verify span is active.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                scope.span.context.unwrap(),
            )

        # Verify no span is active.
        self.assertIsNone(self.shim.active_span)

    def test_start_active_span_finish_on_close(self):
        """Test `finish_on_close` argument of `start_active_span()`."""

        with self.shim.start_active_span(
            "TestSpan7", finish_on_close=True
        ) as scope:
            # Verify span hasn't ended.
            self.assertIsNone(scope.span.unwrap().end_time)

        # Verify span has ended.
        self.assertIsNotNone(scope.span.unwrap().end_time)

        with self.shim.start_active_span(
            "TestSpan8", finish_on_close=False
        ) as scope:
            # Verify span hasn't ended.
            self.assertIsNone(scope.span.unwrap().end_time)

        # Verify span hasn't ended after scope had been closed.
        self.assertIsNone(scope.span.unwrap().end_time)

        scope.span.finish()

    def test_activate_finish_on_close(self):
        """Test `finish_on_close` argument of `activate()`."""

        span = self.shim.start_span("TestSpan9")

        with self.shim.scope_manager.activate(
            span, finish_on_close=True
        ) as scope:
            # Verify span is active.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                scope.span.context.unwrap(),
            )

        # Verify span has ended.
        self.assertIsNotNone(span.unwrap().end_time)

        span = self.shim.start_span("TestSpan10")

        with self.shim.scope_manager.activate(
            span, finish_on_close=False
        ) as scope:
            # Verify span is active.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                scope.span.context.unwrap(),
            )

        # Verify span hasn't ended.
        self.assertIsNone(span.unwrap().end_time)

        span.finish()

    def test_explicit_scope_close(self):
        """Test `close()` method on `ScopeShim`."""

        with self.shim.start_active_span("ParentSpan") as parent:
            # Verify parent span is active.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                parent.span.context.unwrap(),
            )

            child = self.shim.start_active_span("ChildSpan")

            # Verify child span is active.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                child.span.context.unwrap(),
            )

            # Verify child span hasn't ended.
            self.assertIsNone(child.span.unwrap().end_time)

            child.close()

            # Verify child span has ended.
            self.assertIsNotNone(child.span.unwrap().end_time)

            # Verify parent span becomes active again.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                parent.span.context.unwrap(),
            )

    def test_parent_child_implicit(self):
        """Test parent-child relationship and activation/deactivation of spans
        without specifying the parent span upon creation.
        """

        with self.shim.start_active_span("ParentSpan") as parent:
            # Verify parent span is the active span.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                parent.span.context.unwrap(),
            )

            with self.shim.start_active_span("ChildSpan") as child:
                # Verify child span is the active span.
                self.assertEqual(
                    self.shim.active_span.context.unwrap(),
                    child.span.context.unwrap(),
                )

                # Verify parent-child relationship.
                parent_trace_id = (
                    parent.span.unwrap().get_span_context().trace_id
                )
                child_trace_id = (
                    child.span.unwrap().get_span_context().trace_id
                )

                self.assertEqual(parent_trace_id, child_trace_id)
                self.assertEqual(
                    child.span.unwrap().parent,
                    parent.span.unwrap().get_span_context(),
                )

            # Verify parent span becomes the active span again.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                parent.span.context.unwrap()
                # TODO: Check equality of the spans themselves rather than
                # their context once the SpanShim reconstruction problem has
                # been addressed (see previous TODO).
            )

        # Verify there is no active span.
        self.assertIsNone(self.shim.active_span)

    def test_parent_child_explicit_span(self):
        """Test parent-child relationship of spans when specifying a `Span`
        object as a parent upon creation.
        """

        with self.shim.start_span("ParentSpan") as parent:
            with self.shim.start_active_span(
                "ChildSpan", child_of=parent
            ) as child:
                parent_trace_id = parent.unwrap().get_span_context().trace_id
                child_trace_id = (
                    child.span.unwrap().get_span_context().trace_id
                )

                self.assertEqual(child_trace_id, parent_trace_id)
                self.assertEqual(
                    child.span.unwrap().parent,
                    parent.unwrap().get_span_context(),
                )

        with self.shim.start_span("ParentSpan") as parent:
            child = self.shim.start_span("ChildSpan", child_of=parent)

            parent_trace_id = parent.unwrap().get_span_context().trace_id
            child_trace_id = child.unwrap().get_span_context().trace_id

            self.assertEqual(child_trace_id, parent_trace_id)
            self.assertEqual(
                child.unwrap().parent, parent.unwrap().get_span_context()
            )

            child.finish()

    def test_parent_child_explicit_span_context(self):
        """Test parent-child relationship of spans when specifying a
        `SpanContext` object as a parent upon creation.
        """

        with self.shim.start_span("ParentSpan") as parent:
            with self.shim.start_active_span(
                "ChildSpan", child_of=parent.context
            ) as child:
                parent_trace_id = parent.unwrap().get_span_context().trace_id
                child_trace_id = (
                    child.span.unwrap().get_span_context().trace_id
                )

                self.assertEqual(child_trace_id, parent_trace_id)
                self.assertEqual(
                    child.span.unwrap().parent, parent.context.unwrap()
                )

        with self.shim.start_span("ParentSpan") as parent:
            with self.shim.start_span(
                "SpanWithContextParent", child_of=parent.context
            ) as child:
                parent_trace_id = parent.unwrap().get_span_context().trace_id
                child_trace_id = child.unwrap().get_span_context().trace_id

                self.assertEqual(child_trace_id, parent_trace_id)
                self.assertEqual(
                    child.unwrap().parent, parent.context.unwrap()
                )

    def test_references(self):
        """Test span creation using the `references` argument."""

        with self.shim.start_span("ParentSpan") as parent:
            ref = opentracing.child_of(parent.context)

            with self.shim.start_active_span(
                "ChildSpan", references=[ref]
            ) as child:
                self.assertEqual(
                    child.span.unwrap().links[0].context,
                    parent.context.unwrap(),
                )

    def test_set_operation_name(self):
        """Test `set_operation_name()` method."""

        with self.shim.start_active_span("TestName") as scope:
            self.assertEqual(scope.span.unwrap().name, "TestName")

            scope.span.set_operation_name("NewName")
            self.assertEqual(scope.span.unwrap().name, "NewName")

    def test_tags(self):
        """Test tags behavior using the `tags` argument and the `set_tags()`
        method.
        """

        tags = {"foo": "bar"}
        with self.shim.start_active_span("TestSetTag", tags=tags) as scope:
            scope.span.set_tag("baz", "qux")

            self.assertEqual(scope.span.unwrap().attributes["foo"], "bar")
            self.assertEqual(scope.span.unwrap().attributes["baz"], "qux")

    def test_span_tracer(self):
        """Test the `tracer` property on `Span` objects."""

        with self.shim.start_active_span("TestSpan11") as scope:
            self.assertEqual(scope.span.tracer, self.shim)

    def test_log_kv(self):
        """Test the `log_kv()` method on `Span` objects."""

        with self.shim.start_span("TestSpan12") as span:
            span.log_kv({"foo": "bar"})
            self.assertEqual(span.unwrap().events[0].attributes["foo"], "bar")
            # Verify timestamp was generated automatically.
            self.assertIsNotNone(span.unwrap().events[0].timestamp)

            # Test explicit timestamp.
            now = time.time()
            span.log_kv({"foo": "bar"}, now)
            result = util.time_seconds_from_ns(
                span.unwrap().events[1].timestamp
            )
            self.assertEqual(span.unwrap().events[1].attributes["foo"], "bar")
            # Tolerate inaccuracies of less than a microsecond. See Note:
            # https://open-telemetry.github.io/opentelemetry-python/instrumentation/opentracing_shim/opentracing_shim.html
            # TODO: This seems to work consistently, but we should find out the
            # biggest possible loss of precision.
            self.assertAlmostEqual(result, now, places=6)

    def test_log(self):
        """Test the deprecated `log` method on `Span` objects."""

        with self.shim.start_span("TestSpan13") as span:
            with self.assertWarns(DeprecationWarning):
                span.log(event="foo", payload="bar")

        self.assertEqual(span.unwrap().events[0].attributes["event"], "foo")
        self.assertEqual(span.unwrap().events[0].attributes["payload"], "bar")
        self.assertIsNotNone(span.unwrap().events[0].timestamp)

    def test_log_event(self):
        """Test the deprecated `log_event` method on `Span` objects."""

        with self.shim.start_span("TestSpan14") as span:
            with self.assertWarns(DeprecationWarning):
                span.log_event("foo", "bar")

        self.assertEqual(span.unwrap().events[0].attributes["event"], "foo")
        self.assertEqual(span.unwrap().events[0].attributes["payload"], "bar")
        self.assertIsNotNone(span.unwrap().events[0].timestamp)

    def test_span_context(self):
        """Test construction of `SpanContextShim` objects."""

        otel_context = trace.SpanContext(1234, 5678, is_remote=False)
        context = SpanContextShim(otel_context)

        self.assertIsInstance(context, opentracing.SpanContext)
        self.assertEqual(context.unwrap().trace_id, 1234)
        self.assertEqual(context.unwrap().span_id, 5678)

    def test_span_on_error(self):
        """Verify error tag and logs are created on span when an exception is
        raised.
        """

        # Raise an exception while a span is active.
        with self.assertRaises(Exception):
            with self.shim.start_active_span("TestName") as scope:
                raise Exception

        # Verify exception details have been added to span.
        self.assertEqual(scope.span.unwrap().attributes["error"], True)

    def test_inject_http_headers(self):
        """Test `inject()` method for Format.HTTP_HEADERS."""

        otel_context = trace.SpanContext(
            trace_id=1220, span_id=7478, is_remote=False
        )
        context = SpanContextShim(otel_context)

        headers = {}
        self.shim.inject(context, opentracing.Format.HTTP_HEADERS, headers)
        self.assertEqual(
            headers[MockTextMapPropagator.TRACE_ID_KEY], str(1220)
        )
        self.assertEqual(headers[MockTextMapPropagator.SPAN_ID_KEY], str(7478))

    def test_inject_text_map(self):
        """Test `inject()` method for Format.TEXT_MAP."""

        otel_context = trace.SpanContext(
            trace_id=1220, span_id=7478, is_remote=False
        )
        context = SpanContextShim(otel_context)

        # Verify Format.TEXT_MAP
        text_map = {}
        self.shim.inject(context, opentracing.Format.TEXT_MAP, text_map)
        self.assertEqual(
            text_map[MockTextMapPropagator.TRACE_ID_KEY], str(1220)
        )
        self.assertEqual(
            text_map[MockTextMapPropagator.SPAN_ID_KEY], str(7478)
        )

    def test_inject_binary(self):
        """Test `inject()` method for Format.BINARY."""

        otel_context = trace.SpanContext(
            trace_id=1220, span_id=7478, is_remote=False
        )
        context = SpanContextShim(otel_context)

        # Verify exception for non supported binary format.
        with self.assertRaises(opentracing.UnsupportedFormatException):
            self.shim.inject(context, opentracing.Format.BINARY, bytearray())

    def test_extract_http_headers(self):
        """Test `extract()` method for Format.HTTP_HEADERS."""

        carrier = {
            MockTextMapPropagator.TRACE_ID_KEY: 1220,
            MockTextMapPropagator.SPAN_ID_KEY: 7478,
        }

        ctx = self.shim.extract(opentracing.Format.HTTP_HEADERS, carrier)
        self.assertEqual(ctx.unwrap().trace_id, 1220)
        self.assertEqual(ctx.unwrap().span_id, 7478)

    def test_extract_empty_context_returns_invalid_context(self):
        """In the case where the propagator cannot extract a
        SpanContext, extract should return and invalid span context.
        """
        _old_propagator = propagators.get_global_textmap()
        propagators.set_global_textmap(NOOPTextMapPropagator())
        try:
            carrier = {}

            ctx = self.shim.extract(opentracing.Format.HTTP_HEADERS, carrier)
            self.assertEqual(ctx.unwrap(), trace.INVALID_SPAN_CONTEXT)
        finally:
            propagators.set_global_textmap(_old_propagator)

    def test_extract_text_map(self):
        """Test `extract()` method for Format.TEXT_MAP."""

        carrier = {
            MockTextMapPropagator.TRACE_ID_KEY: 1220,
            MockTextMapPropagator.SPAN_ID_KEY: 7478,
        }

        ctx = self.shim.extract(opentracing.Format.TEXT_MAP, carrier)
        self.assertEqual(ctx.unwrap().trace_id, 1220)
        self.assertEqual(ctx.unwrap().span_id, 7478)

    def test_extract_binary(self):
        """Test `extract()` method for Format.BINARY."""

        # Verify exception for non supported binary format.
        with self.assertRaises(opentracing.UnsupportedFormatException):
            self.shim.extract(opentracing.Format.BINARY, bytearray())

    def test_baggage(self):

        span_context_shim = SpanContextShim(
            trace.SpanContext(1234, 5678, is_remote=False)
        )

        baggage = span_context_shim.baggage

        with self.assertRaises(ValueError):
            baggage[1] = 3

        span_shim = SpanShim(Mock(), span_context_shim, Mock())

        span_shim.set_baggage_item(1, 2)

        self.assertTrue(span_shim.get_baggage_item(1), 2)

    def test_active(self):
        """Test that the active property and start_active_span return the same
        object"""

        # Verify no span is currently active.
        self.assertIsNone(self.shim.active_span)

        with self.shim.start_active_span("TestSpan15") as scope:
            # Verify span is active.
            self.assertEqual(
                self.shim.active_span.context.unwrap(),
                scope.span.context.unwrap(),
            )

            self.assertIs(self.shim.scope_manager.active, scope)

        # Verify no span is active.
        self.assertIsNone(self.shim.active_span)

    def test_mixed_mode(self):
        """Test that span parent-child relationship is kept between
        OpenTelemetry and the OpenTracing shim"""

        span_shim = self.shim.start_span("TestSpan16")

        with self.shim.scope_manager.activate(span_shim, finish_on_close=True):

            with (
                TracerProvider()
                .get_tracer(__name__)
                .start_as_current_span("abc")
            ) as opentelemetry_span:

                self.assertIs(
                    span_shim.unwrap().context, opentelemetry_span.parent,
                )

        with (
            TracerProvider().get_tracer(__name__).start_as_current_span("abc")
        ) as opentelemetry_span:

            with self.shim.start_active_span("TestSpan17") as scope:

                self.assertIs(
                    scope.span.unwrap().parent, opentelemetry_span.context,
                )
