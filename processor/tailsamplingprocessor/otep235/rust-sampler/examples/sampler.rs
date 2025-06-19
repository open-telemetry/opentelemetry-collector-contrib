use opentelemetry::{
    global,
    trace::{TraceContextExt, Tracer},
    KeyValue,
};
use opentelemetry_sdk::trace::{TraceError, SdkTracerProvider};
use opentelemetry_sdk::Resource;

use rust_sampler::{ComposableSampler,CompositeSampler};

use std::error::Error;

fn init_tracer_provider() -> Result<opentelemetry_sdk::trace::SdkTracerProvider, TraceError> {
    let exporter = opentelemetry_stdout::SpanExporter::default();

    let sampler = CompositeSampler::new(Box::new(ComposableSampler::AlwaysOn));

    Ok(SdkTracerProvider::builder()
       .with_batch_exporter(exporter)
       .with_resource(
           Resource::builder()
               .with_service_name("tracing-stdout")
               .build(),
       )
       .with_sampler(sampler)
       .build())
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error + Send + Sync + 'static>> {
    let tracer_provider = init_tracer_provider().expect("Failed to initialize tracer provider.");
    global::set_tracer_provider(tracer_provider.clone());

    let tracer = global::tracer("rust-sampler");
    tracer.in_span("main-operation", |cx| {
        let span = cx.span();
        span.set_attribute(KeyValue::new("my-attribute", "my-value"));
        span.add_event(
            "Main span event".to_string(),
            vec![KeyValue::new("foo", "1")],
        );
        tracer.in_span("child-operation...", |cx| {
            let span = cx.span();
            span.add_event("Sub span event", vec![KeyValue::new("bar", "1")]);
        });
    });

    tracer_provider.shutdown()?;

    Ok(())
}
