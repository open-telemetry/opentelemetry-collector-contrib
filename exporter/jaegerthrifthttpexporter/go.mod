module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter

go 1.12

require (
	github.com/apache/thrift v0.0.0-20161221203622-b2a4d4ae21c7
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0
	github.com/jaegertracing/jaeger v1.17.0
	github.com/open-telemetry/opentelemetry-collector v0.3.1-0.20200424155504-9d16f5971ef9
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.10.0
)
