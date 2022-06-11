# OpenTelemetry Collector Demo

This demo is a sample app to build the collector and exercise its tracing functionality.

To build and run the demo, switch to this directory and run

`docker-compose up`

There will be an app continuously sending traces, open
up `http://localhost:9411` or `http://localhost:16686` to search for the traces in the
Zipkin or Jaeger UIs.