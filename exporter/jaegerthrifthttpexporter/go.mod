module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter

go 1.14

require (
	github.com/apache/thrift v0.13.0
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/google/go-cmp v0.5.2
	github.com/jaegertracing/jaeger v1.19.2
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.9.1-0.20200903192028-315e96ecb6d7
	go.uber.org/zap v1.15.0
	google.golang.org/protobuf v1.25.0
)
