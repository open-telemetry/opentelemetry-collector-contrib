import time

import opentracing
from opentracing import (
    child_of,
    Format,
    InvalidCarrierException,
    UnsupportedFormatException,
    SpanContextCorruptedException,
)

import ddtrace
from ddtrace.ext.priority import AUTO_KEEP
from ddtrace.opentracer import Tracer, set_global_tracer
from ddtrace.opentracer.span_context import SpanContext
from ddtrace.propagation.http import HTTP_HEADER_TRACE_ID
from ddtrace.settings import ConfigException

import mock
import pytest


class TestTracerConfig(object):
    def test_config(self):
        """Test the configuration of the tracer"""
        config = {"enabled": True}
        tracer = Tracer(service_name="myservice", config=config)

        assert tracer._service_name == "myservice"
        assert tracer._enabled is True

    def test_no_service_name(self):
        """A service_name should be generated if one is not provided."""
        tracer = Tracer()
        assert tracer._service_name == "pytest"

    def test_multiple_tracer_configs(self):
        """Ensure that a tracer config is a copy of the passed config."""
        config = {"enabled": True}

        tracer1 = Tracer(service_name="serv1", config=config)
        assert tracer1._service_name == "serv1"

        config["enabled"] = False
        tracer2 = Tracer(service_name="serv2", config=config)

        # Ensure tracer1's config was not mutated
        assert tracer1._service_name == "serv1"
        assert tracer1._enabled is True

        assert tracer2._service_name == "serv2"
        assert tracer2._enabled is False

    def test_invalid_config_key(self):
        """A config with an invalid key should raise a ConfigException."""

        config = {"enabeld": False}

        # No debug flag should not raise an error
        tracer = Tracer(service_name="mysvc", config=config)

        # With debug flag should raise an error
        config["debug"] = True
        with pytest.raises(ConfigException) as ce_info:
            tracer = Tracer(config=config)
            assert "enabeld" in str(ce_info)
            assert tracer is not None

        # Test with multiple incorrect keys
        config["setttings"] = {}
        with pytest.raises(ConfigException) as ce_info:
            tracer = Tracer(service_name="mysvc", config=config)
            assert ["enabeld", "setttings"] in str(ce_info)
            assert tracer is not None

    def test_global_tags(self):
        """Global tags should be passed from the opentracer to the tracer."""
        config = {
            "global_tags": {"tag1": "value1", "tag2": 2,},
        }

        tracer = Tracer(service_name="mysvc", config=config)
        with tracer.start_span("myop") as span:
            # global tags should be attached to generated all datadog spans
            assert span._dd_span.get_tag("tag1") == "value1"
            assert span._dd_span.get_metric("tag2") == 2

            with tracer.start_span("myop2") as span2:
                assert span2._dd_span.get_tag("tag1") == "value1"
                assert span2._dd_span.get_metric("tag2") == 2


class TestTracer(object):
    def test_start_span(self, ot_tracer, writer):
        """Start and finish a span."""
        with ot_tracer.start_span("myop") as span:
            pass

        # span should be finished when the context manager exits
        assert span.finished

        spans = writer.pop()
        assert len(spans) == 1

    def test_start_span_references(self, ot_tracer, writer):
        """Start a span using references."""

        with ot_tracer.start_span("one", references=[child_of()]):
            pass

        spans = writer.pop()
        assert spans[0].parent_id is None

        root = ot_tracer.start_active_span("root")
        # create a child using a parent reference that is not the context parent
        with ot_tracer.start_active_span("one"):
            with ot_tracer.start_active_span("two", references=[child_of(root.span)]):
                pass
        root.close()

        spans = writer.pop()
        assert spans[2].parent_id is spans[0].span_id

    def test_start_span_custom_start_time(self, ot_tracer):
        """Start a span with a custom start time."""
        t = 100
        with mock.patch("ddtrace.span.time_ns") as time:
            time.return_value = 102 * 1e9
            with ot_tracer.start_span("myop", start_time=t) as span:
                pass

        assert span._dd_span.start == t
        assert span._dd_span.duration == 2

    def test_start_span_with_spancontext(self, ot_tracer, writer):
        """Start and finish a span using a span context as the child_of
        reference.
        """
        with ot_tracer.start_span("myop") as span:
            with ot_tracer.start_span("myop", child_of=span.context) as span2:
                pass

        # span should be finished when the context manager exits
        assert span.finished
        assert span2.finished

        spans = writer.pop()
        assert len(spans) == 2

        # ensure proper parenting
        assert spans[1].parent_id is spans[0].span_id

    def test_start_span_with_tags(self, ot_tracer):
        """Create a span with initial tags."""
        tags = {"key": "value", "key2": "value2"}
        with ot_tracer.start_span("myop", tags=tags) as span:
            pass

        assert span._dd_span.get_tag("key") == "value"
        assert span._dd_span.get_tag("key2") == "value2"

    def test_start_span_with_resource_name_tag(self, ot_tracer):
        """Create a span with the tag to set the resource name"""
        tags = {"resource.name": "value", "key2": "value2"}
        with ot_tracer.start_span("myop", tags=tags) as span:
            pass

        # Span resource name should be set to tag value, and should not get set as
        # a tag on the underlying span.
        assert span._dd_span.resource == "value"
        assert span._dd_span.get_tag("resource.name") is None

        # Other tags are set as normal
        assert span._dd_span.get_tag("key2") == "value2"

    def test_start_active_span_multi_child(self, ot_tracer, writer):
        """Start and finish multiple child spans.
        This should ensure that child spans can be created 2 levels deep.
        """
        with ot_tracer.start_active_span("myfirstop") as scope1:
            time.sleep(0.009)
            with ot_tracer.start_active_span("mysecondop") as scope2:
                time.sleep(0.007)
                with ot_tracer.start_active_span("mythirdop") as scope3:
                    time.sleep(0.005)

        # spans should be finished when the context manager exits
        assert scope1.span.finished
        assert scope2.span.finished
        assert scope3.span.finished

        spans = writer.pop()

        # check spans are captured in the trace
        assert scope1.span._dd_span is spans[0]
        assert scope2.span._dd_span is spans[1]
        assert scope3.span._dd_span is spans[2]

        # ensure proper parenting
        assert spans[1].parent_id is spans[0].span_id
        assert spans[2].parent_id is spans[1].span_id

        # sanity check a lower bound on the durations
        assert spans[0].duration >= 0.009 + 0.007 + 0.005
        assert spans[1].duration >= 0.007 + 0.005
        assert spans[2].duration >= 0.005

    def test_start_active_span_multi_child_siblings(self, ot_tracer, writer):
        """Start and finish multiple span at the same level.
        This should test to ensure a parent can have multiple child spans at the
        same level.
        """
        with ot_tracer.start_active_span("myfirstop") as scope1:
            time.sleep(0.009)
            with ot_tracer.start_active_span("mysecondop") as scope2:
                time.sleep(0.007)
            with ot_tracer.start_active_span("mythirdop") as scope3:
                time.sleep(0.005)

        # spans should be finished when the context manager exits
        assert scope1.span.finished
        assert scope2.span.finished
        assert scope3.span.finished

        spans = writer.pop()

        # check spans are captured in the trace
        assert scope1.span._dd_span is spans[0]
        assert scope2.span._dd_span is spans[1]
        assert scope3.span._dd_span is spans[2]

        # ensure proper parenting
        assert spans[1].parent_id is spans[0].span_id
        assert spans[2].parent_id is spans[0].span_id

        # sanity check a lower bound on the durations
        assert spans[0].duration >= 0.009 + 0.007 + 0.005
        assert spans[1].duration >= 0.007
        assert spans[2].duration >= 0.005

    def test_start_span_manual_child_of(self, ot_tracer, writer):
        """Start spans without using a scope manager.
        Spans should be created without parents since there will be no call
        for the active span.
        """
        root = ot_tracer.start_span("zero")

        with ot_tracer.start_span("one", child_of=root):
            with ot_tracer.start_span("two", child_of=root):
                with ot_tracer.start_span("three", child_of=root):
                    pass
        root.finish()

        spans = writer.pop()

        assert spans[0].parent_id is None
        # ensure each child span is a child of root
        assert spans[1].parent_id is root._dd_span.span_id
        assert spans[2].parent_id is root._dd_span.span_id
        assert spans[3].parent_id is root._dd_span.span_id
        assert spans[0].trace_id == spans[1].trace_id and spans[1].trace_id == spans[2].trace_id

    def test_start_span_no_active_span(self, ot_tracer, writer):
        """Start spans without using a scope manager.
        Spans should be created without parents since there will be no call
        for the active span.
        """
        with ot_tracer.start_span("one", ignore_active_span=True):
            with ot_tracer.start_span("two", ignore_active_span=True):
                pass
            with ot_tracer.start_span("three", ignore_active_span=True):
                pass

        spans = writer.pop()

        # ensure each span does not have a parent
        assert spans[0].parent_id is None
        assert spans[1].parent_id is None
        assert spans[2].parent_id is None
        # and that each span is a new trace
        assert (
            spans[0].trace_id != spans[1].trace_id
            and spans[1].trace_id != spans[2].trace_id
            and spans[0].trace_id != spans[2].trace_id
        )

    def test_start_active_span_child_finish_after_parent(self, ot_tracer, writer):
        """Start a child span and finish it after its parent."""
        span1 = ot_tracer.start_active_span("one").span
        span2 = ot_tracer.start_active_span("two").span
        span1.finish()
        time.sleep(0.005)
        span2.finish()

        spans = writer.pop()
        assert len(spans) == 2
        assert spans[0].parent_id is None
        assert spans[1].parent_id is span1._dd_span.span_id
        assert spans[1].duration > spans[0].duration

    def test_start_span_multi_intertwined(self, ot_tracer, writer):
        """Start multiple spans at the top level intertwined.
        Alternate calling between two traces.
        """
        import threading

        # synchronize threads with a threading event object
        event = threading.Event()

        def trace_one():
            id = 11  # noqa: A001
            with ot_tracer.start_active_span(str(id)):
                id += 1
                with ot_tracer.start_active_span(str(id)):
                    id += 1
                    with ot_tracer.start_active_span(str(id)):
                        event.set()

        def trace_two():
            id = 21  # noqa: A001
            event.wait()
            with ot_tracer.start_active_span(str(id)):
                id += 1
                with ot_tracer.start_active_span(str(id)):
                    id += 1
                with ot_tracer.start_active_span(str(id)):
                    pass

        # the ordering should be
        # t1.span1/t2.span1, t2.span2, t1.span2, t1.span3, t2.span3
        t1 = threading.Thread(target=trace_one)
        t2 = threading.Thread(target=trace_two)

        t1.start()
        t2.start()
        # wait for threads to finish
        t1.join()
        t2.join()

        spans = writer.pop()

        # trace_one will finish before trace_two so its spans should be written
        # before the spans from trace_two, let's confirm this
        assert spans[0].name == "11"
        assert spans[1].name == "12"
        assert spans[2].name == "13"
        assert spans[3].name == "21"
        assert spans[4].name == "22"
        assert spans[5].name == "23"

        # next let's ensure that each span has the correct parent:
        # trace_one
        assert spans[0].parent_id is None
        assert spans[1].parent_id is spans[0].span_id
        assert spans[2].parent_id is spans[1].span_id
        # trace_two
        assert spans[3].parent_id is None
        assert spans[4].parent_id is spans[3].span_id
        assert spans[5].parent_id is spans[3].span_id

        # finally we should ensure that the trace_ids are reasonable
        # trace_one
        assert spans[0].trace_id == spans[1].trace_id and spans[1].trace_id == spans[2].trace_id
        # traces should be independent
        assert spans[2].trace_id != spans[3].trace_id
        # trace_two
        assert spans[3].trace_id == spans[4].trace_id and spans[4].trace_id == spans[5].trace_id

    def test_start_active_span(self, ot_tracer, writer):
        with ot_tracer.start_active_span("one") as scope:
            pass

        assert scope.span._dd_span.name == "one"
        assert scope.span.finished
        spans = writer.pop()
        assert spans

    def test_start_active_span_finish_on_close(self, ot_tracer, writer):
        with ot_tracer.start_active_span("one", finish_on_close=False) as scope:
            pass

        assert scope.span._dd_span.name == "one"
        assert not scope.span.finished
        spans = writer.pop()
        assert not spans

    def test_start_active_span_nested(self, ot_tracer):
        """Test the active span of multiple nested calls of start_active_span."""
        with ot_tracer.start_active_span("one") as outer_scope:
            assert ot_tracer.active_span == outer_scope.span
            with ot_tracer.start_active_span("two") as inner_scope:
                assert ot_tracer.active_span == inner_scope.span
                with ot_tracer.start_active_span("three") as innest_scope:  # why isn't it innest? innermost so verbose
                    assert ot_tracer.active_span == innest_scope.span
            with ot_tracer.start_active_span("two") as inner_scope:
                assert ot_tracer.active_span == inner_scope.span
            assert ot_tracer.active_span == outer_scope.span
        assert ot_tracer.active_span is None

    def test_start_active_span_trace(self, ot_tracer, writer):
        """Test the active span of multiple nested calls of start_active_span."""
        with ot_tracer.start_active_span("one") as outer_scope:
            outer_scope.span.set_tag("outer", 2)
            with ot_tracer.start_active_span("two") as inner_scope:
                inner_scope.span.set_tag("inner", 3)
            with ot_tracer.start_active_span("two") as inner_scope:
                inner_scope.span.set_tag("inner", 3)
                with ot_tracer.start_active_span("three") as innest_scope:
                    innest_scope.span.set_tag("innerest", 4)

        spans = writer.pop()

        assert spans[0].parent_id is None
        assert spans[1].parent_id is spans[0].span_id
        assert spans[2].parent_id is spans[0].span_id
        assert spans[3].parent_id is spans[2].span_id


@pytest.fixture
def nop_span_ctx():

    return SpanContext(sampling_priority=AUTO_KEEP)


class TestTracerSpanContextPropagation(object):
    """Test the injection and extration of a span context from a tracer."""

    def test_invalid_format(self, ot_tracer, nop_span_ctx):
        """An invalid format should raise an UnsupportedFormatException."""
        # test inject
        with pytest.raises(UnsupportedFormatException):
            ot_tracer.inject(nop_span_ctx, None, {})

        # test extract
        with pytest.raises(UnsupportedFormatException):
            ot_tracer.extract(None, {})

    def test_inject_invalid_carrier(self, ot_tracer, nop_span_ctx):
        """Only dicts should be supported as a carrier."""
        with pytest.raises(InvalidCarrierException):
            ot_tracer.inject(nop_span_ctx, Format.HTTP_HEADERS, None)

    def test_extract_invalid_carrier(self, ot_tracer):
        """Only dicts should be supported as a carrier."""
        with pytest.raises(InvalidCarrierException):
            ot_tracer.extract(Format.HTTP_HEADERS, None)

    def test_http_headers_base(self, ot_tracer):
        """extract should undo inject for http headers."""

        span_ctx = SpanContext(trace_id=123, span_id=456)
        carrier = {}

        ot_tracer.inject(span_ctx, Format.HTTP_HEADERS, carrier)
        assert len(carrier.keys()) > 0

        ext_span_ctx = ot_tracer.extract(Format.HTTP_HEADERS, carrier)
        assert ext_span_ctx._dd_context.trace_id == 123
        assert ext_span_ctx._dd_context.span_id == 456

    def test_http_headers_baggage(self, ot_tracer):
        """extract should undo inject for http headers."""
        span_ctx = SpanContext(trace_id=123, span_id=456, baggage={"test": 4, "test2": "string"})
        carrier = {}

        ot_tracer.inject(span_ctx, Format.HTTP_HEADERS, carrier)
        assert len(carrier.keys()) > 0

        ext_span_ctx = ot_tracer.extract(Format.HTTP_HEADERS, carrier)
        assert ext_span_ctx._dd_context.trace_id == 123
        assert ext_span_ctx._dd_context.span_id == 456
        assert ext_span_ctx.baggage == span_ctx.baggage

    def test_empty_propagated_context(self, ot_tracer):
        """An empty propagated context should raise a
        SpanContextCorruptedException when extracted.
        """
        carrier = {}
        with pytest.raises(SpanContextCorruptedException):
            ot_tracer.extract(Format.HTTP_HEADERS, carrier)

    def test_text(self, ot_tracer):
        """extract should undo inject for http headers"""
        span_ctx = SpanContext(trace_id=123, span_id=456, baggage={"test": 4, "test2": "string"})
        carrier = {}

        ot_tracer.inject(span_ctx, Format.TEXT_MAP, carrier)
        assert len(carrier.keys()) > 0

        ext_span_ctx = ot_tracer.extract(Format.TEXT_MAP, carrier)
        assert ext_span_ctx._dd_context.trace_id == 123
        assert ext_span_ctx._dd_context.span_id == 456
        assert ext_span_ctx.baggage == span_ctx.baggage

    def test_corrupted_propagated_context(self, ot_tracer):
        """Corrupted context should raise a SpanContextCorruptedException."""
        span_ctx = SpanContext(trace_id=123, span_id=456, baggage={"test": 4, "test2": "string"})
        carrier = {}

        ot_tracer.inject(span_ctx, Format.TEXT_MAP, carrier)
        assert len(carrier.keys()) > 0

        # manually alter a key in the carrier baggage
        del carrier[HTTP_HEADER_TRACE_ID]
        corrupted_key = HTTP_HEADER_TRACE_ID[2:]
        carrier[corrupted_key] = 123

        with pytest.raises(SpanContextCorruptedException):
            ot_tracer.extract(Format.TEXT_MAP, carrier)

    def test_immutable_span_context(self, ot_tracer):
        """Span contexts should be immutable."""
        with ot_tracer.start_span("root") as root:
            ctx_before = root.context
            root.set_baggage_item("test", 2)
            assert ctx_before is not root.context
            with ot_tracer.start_span("child") as level1:
                with ot_tracer.start_span("child") as level2:
                    pass
        assert root.context is not level1.context
        assert level2.context is not level1.context
        assert level2.context is not root.context

    def test_inherited_baggage(self, ot_tracer):
        """Baggage should be inherited by child spans."""
        with ot_tracer.start_active_span("root") as root:
            # this should be passed down to the child
            root.span.set_baggage_item("root", 1)
            root.span.set_baggage_item("root2", 1)
            with ot_tracer.start_active_span("child") as level1:
                level1.span.set_baggage_item("level1", 1)
                with ot_tracer.start_active_span("child") as level2:
                    level2.span.set_baggage_item("level2", 1)
        # ensure immutability
        assert level1.span.context is not root.span.context
        assert level2.span.context is not level1.span.context

        # level1 should have inherited the baggage of root
        assert level1.span.get_baggage_item("root")
        assert level1.span.get_baggage_item("root2")

        # level2 should have inherited the baggage of both level1 and level2
        assert level2.span.get_baggage_item("root")
        assert level2.span.get_baggage_item("root2")
        assert level2.span.get_baggage_item("level1")
        assert level2.span.get_baggage_item("level2")


class TestTracerCompatibility(object):
    """Ensure that our opentracer produces results in the underlying datadog tracer."""

    def test_required_dd_fields(self):
        """Ensure required fields needed for successful tracing are possessed
        by the underlying datadog tracer.
        """
        # a service name is required
        tracer = Tracer("service")
        with tracer.start_span("my_span") as span:
            assert span._dd_span.service


def test_set_global_tracer():
    """Sanity check for set_global_tracer"""
    my_tracer = Tracer("service")
    set_global_tracer(my_tracer)

    assert opentracing.tracer is my_tracer
    assert ddtrace.tracer is my_tracer._dd_tracer
