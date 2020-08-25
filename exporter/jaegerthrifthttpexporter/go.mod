module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter

go 1.14

require (
	github.com/apache/thrift v0.13.0
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/google/go-cmp v0.5.1
	github.com/jaegertracing/jaeger v1.18.2-0.20200707061226-97d2319ff2be
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.8.1-0.20200825172515-42735a24c60b
	go.uber.org/zap v1.15.0
	google.golang.org/protobuf v1.25.0
)
