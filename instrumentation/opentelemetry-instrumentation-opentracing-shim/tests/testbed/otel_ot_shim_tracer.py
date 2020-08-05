import opentelemetry.instrumentation.opentracing_shim as opentracingshim
from opentelemetry.sdk import trace
from opentelemetry.sdk.trace.export import SimpleExportSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import (
    InMemorySpanExporter,
)


class MockTracer(opentracingshim.TracerShim):
    """Wrapper of `opentracingshim.TracerShim`.

    MockTracer extends `opentracingshim.TracerShim` by adding a in memory
    span exporter that can be used to get the list of finished spans."""

    def __init__(self):
        tracer_provider = trace.TracerProvider()
        oteltracer = tracer_provider.get_tracer(__name__)
        super(MockTracer, self).__init__(oteltracer)
        exporter = InMemorySpanExporter()
        span_processor = SimpleExportSpanProcessor(exporter)
        tracer_provider.add_span_processor(span_processor)

        self.exporter = exporter

    def finished_spans(self):
        return self.exporter.get_finished_spans()
