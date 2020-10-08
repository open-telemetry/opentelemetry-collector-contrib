import time

from opentelemetry import trace
from opentelemetry.exporter import jaeger
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchExportSpanProcessor

trace.set_tracer_provider(TracerProvider())
tracer = trace.get_tracer(__name__)

# create a JaegerSpanExporter
jaeger_exporter = jaeger.JaegerSpanExporter(
    service_name="my-helloworld-service",
    # configure agent
    agent_host_name="localhost",
    agent_port=6831,
    # optional: configure also collector
    # collector_host_name="localhost",
    # collector_port=14268,
    # collector_endpoint="/api/traces?format=jaeger.thrift",
    # username=xxxx, # optional
    # password=xxxx, # optional
)

# create a BatchExportSpanProcessor and add the exporter to it
span_processor = BatchExportSpanProcessor(jaeger_exporter)

# add to the tracer factory
trace.get_tracer_provider().add_span_processor(span_processor)

# create some spans for testing
with tracer.start_as_current_span("foo") as foo:
    time.sleep(0.1)
    foo.set_attribute("my_atribbute", True)
    foo.add_event("event in foo", {"name": "foo1"})
    with tracer.start_as_current_span(
        "bar", links=[trace.Link(foo.get_span_context())]
    ) as bar:
        time.sleep(0.2)
        bar.set_attribute("speed", 100.0)

        with tracer.start_as_current_span("baz") as baz:
            time.sleep(0.3)
            baz.set_attribute("name", "mauricio")

        time.sleep(0.2)

    time.sleep(0.1)
